package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"time"

	pb_client "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/client"
	pb_email "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/email"
)

const maxRetries = 5

// StartCronJobs launches the monthly and daily cron goroutines.
func (s *LoanServer) StartCronJobs() {
	go s.runDailyCron()
	go s.runMonthlyCron()
}

// --- Daily cron: automatic installment deduction (#98) ---

func (s *LoanServer) runDailyCron() {
	for {
		now := time.Now()
		// Schedule next run at midnight
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		time.Sleep(time.Until(next))
		log.Println("loan-service: running daily installment deduction")
		s.collectInstallments(0)
	}
}

func (s *LoanServer) collectInstallments(forceLoanID int64) int {
	ctx := context.Background()

	var rows *sql.Rows
	var err error

	if forceLoanID > 0 {
		// Force-process a specific loan regardless of next_installment_date (used by tests)
		rows, err = s.DB.QueryContext(ctx, `
			SELECT id, loan_number, client_id, account_number, next_installment_amount, currency, remaining_debt
			FROM loans
			WHERE id = $1 AND status IN ('APPROVED', 'IN_DELAY')`, forceLoanID)
	} else {
		// Normal daily run: find all loans due today
		rows, err = s.DB.QueryContext(ctx, `
			SELECT id, loan_number, client_id, account_number, next_installment_amount, currency, remaining_debt
			FROM loans
			WHERE (
			    (status = 'APPROVED' AND next_installment_date = CURRENT_DATE)
			    OR
			    (status = 'IN_DELAY' AND EXISTS (
			        SELECT 1 FROM loan_installments
			        WHERE loan_id = loans.id AND status IN ('UNPAID', 'LATE')
			          AND (last_retry_at IS NULL OR last_retry_at < NOW() - INTERVAL '72 hours')
			        LIMIT 1
			    ))
			)`)
	}
	if err != nil {
		log.Printf("loan-service: daily cron query error: %v", err)
		return 0
	}
	defer func() { _ = rows.Close() }()

	var loans []struct {
		id            int64
		loanNumber    int64
		clientID      int64
		accountNumber string
		amount        float64
		currency      string
		remainingDebt float64
	}
	for rows.Next() {
		var l struct {
			id            int64
			loanNumber    int64
			clientID      int64
			accountNumber string
			amount        float64
			currency      string
			remainingDebt float64
		}
		if err := rows.Scan(&l.id, &l.loanNumber, &l.clientID, &l.accountNumber, &l.amount, &l.currency, &l.remainingDebt); err != nil {
			log.Printf("loan-service: daily cron scan error: %v", err)
			continue
		}
		loans = append(loans, l)
	}

	for _, l := range loans {
		s.processInstallment(ctx, l.id, l.loanNumber, l.clientID, l.accountNumber, l.amount, l.currency, l.remainingDebt)
	}
	return len(loans)
}

func (s *LoanServer) processInstallment(ctx context.Context, loanID int64, loanNumber int64, clientID int64, accountNumber string, amount float64, currency string, remainingDebt float64) {
	// Find the UNPAID or LATE installment due today or earliest overdue
	var installmentID int64
	var retryCount int
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, retry_count FROM loan_installments
		WHERE loan_id = $1 AND status IN ('UNPAID', 'LATE')
		ORDER BY expected_due_date ASC LIMIT 1`, loanID,
	).Scan(&installmentID, &retryCount)
	if err == sql.ErrNoRows {
		return
	} else if err != nil {
		log.Printf("loan-service: installment lookup error for loan %d: %v", loanID, err)
		return
	}

	// Resolve bank account for this currency
	var currencyID int64
	if err := s.ExchangeDB.QueryRowContext(ctx,
		`SELECT id FROM currencies WHERE code = $1`, currency,
	).Scan(&currencyID); err != nil {
		log.Printf("loan-service: failed to resolve currency %s for loan %d: %v", currency, loanID, err)
		return
	}
	var bankAccountNumber string
	if err := s.AccountDB.QueryRowContext(ctx,
		`SELECT account_number FROM accounts WHERE account_type = 'BANK' AND currency_id = $1`, currencyID,
	).Scan(&bankAccountNumber); err != nil {
		log.Printf("loan-service: no bank account for currency %s, loan %d: %v", currency, loanID, err)
		return
	}

	// Attempt debit
	res, err := s.AccountDB.ExecContext(ctx, `
		UPDATE accounts
		SET balance = balance - $1, available_balance = available_balance - $1
		WHERE account_number = $2 AND available_balance >= $1`,
		amount, accountNumber)
	if err != nil {
		log.Printf("loan-service: debit error for loan %d: %v", loanID, err)
		return
	}
	affected, _ := res.RowsAffected()

	if affected > 0 {
		// Credit bank account with the collected installment
		if _, err := s.AccountDB.ExecContext(ctx, `
			UPDATE accounts SET balance = balance + $1, available_balance = available_balance + $1
			WHERE account_number = $2`, amount, bankAccountNumber); err != nil {
			log.Printf("loan-service: failed to credit bank account for loan %d: %v", loanID, err)
		}
		// Success: mark PAID, advance schedule
		newRemaining := math.Round((remainingDebt-amount)*100) / 100
		if newRemaining < 0 {
			newRemaining = 0
		}
		_, err = s.DB.ExecContext(ctx, `
			UPDATE loan_installments
			SET status = 'PAID', actual_due_date = CURRENT_DATE
			WHERE id = $1`, installmentID)
		if err != nil {
			log.Printf("loan-service: mark PAID error: %v", err)
		}

		loanStatus := "APPROVED"
		if newRemaining == 0 {
			loanStatus = "PAID_OFF"
		}
		_, err = s.DB.ExecContext(ctx, `
			UPDATE loans SET
				remaining_debt = $1,
				next_installment_date = next_installment_date + INTERVAL '1 month',
				status = $2
			WHERE id = $3`, newRemaining, loanStatus, loanID)
		if err != nil {
			log.Printf("loan-service: advance schedule error: %v", err)
		}
		log.Printf("loan-service: installment collected for loan %d (remaining: %.2f)", loanID, newRemaining)
	} else {
		// Insufficient funds
		newRetry := retryCount + 1
		now := time.Now()
		_, err = s.DB.ExecContext(ctx, `
			UPDATE loan_installments
			SET status = 'LATE', retry_count = $1, last_retry_at = $2
			WHERE id = $3`, newRetry, now, installmentID)
		if err != nil {
			log.Printf("loan-service: mark LATE error: %v", err)
		}

		penaltyRate := 0.0
		if newRetry >= maxRetries {
			penaltyRate = 0.05 // +0.05% penalty per issue #98
			log.Printf("loan-service: loan %d exceeded max retries — applying penalty rate +%.2f%%", loanID, penaltyRate)
		}

		_, err = s.DB.ExecContext(ctx, `
			UPDATE loans SET status = 'IN_DELAY',
			effective_rate = effective_rate + $1
			WHERE id = $2`, penaltyRate, loanID)
		if err != nil {
			log.Printf("loan-service: set IN_DELAY error: %v", err)
		}
		log.Printf("loan-service: insufficient funds for loan %d (retry %d)", loanID, newRetry)
		if s.EmailClient != nil && s.ClientClient != nil {
			if clientResp, err := s.ClientClient.GetClientById(ctx, &pb_client.GetClientByIdRequest{Id: clientID}); err == nil {
				c := clientResp.Client
				_, _ = s.EmailClient.SendLoanLatePaymentEmail(ctx, &pb_email.SendLoanLatePaymentEmailRequest{
					Email:      c.Email,
					FirstName:  c.FirstName,
					LoanNumber: fmt.Sprintf("%d", loanNumber),
					AmountDue:  amount,
					Currency:   currency,
					RetryCount: int32(newRetry),
				})
			} else {
				log.Printf("loan-service: could not fetch client %d for late payment email: %v", clientID, err)
			}
		}
	}
}

// --- Monthly cron: variable rate spread simulation (#97) ---

func (s *LoanServer) runMonthlyCron() {
	for {
		now := time.Now()
		// Schedule for 1st of next month at 01:00
		firstOfNext := time.Date(now.Year(), now.Month()+1, 1, 1, 0, 0, 0, now.Location())
		time.Sleep(time.Until(firstOfNext))
		log.Println("loan-service: running monthly variable rate update")
		s.updateVariableRates()
	}
}

func (s *LoanServer) updateVariableRates() {
	ctx := context.Background()

	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, loan_type, amount, nominal_rate, effective_rate, remaining_debt, repayment_period, agreed_date
		FROM loans
		WHERE interest_rate_type = 'VARIABLE' AND status = 'APPROVED'`)
	if err != nil {
		log.Printf("loan-service: monthly cron query error: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	type varLoan struct {
		id              int64
		loanType        string
		amount          float64
		nominalRate     float64
		effectiveRate   float64
		remainingDebt   float64
		repaymentPeriod int
		agreedDate      time.Time
	}

	var loans []varLoan
	for rows.Next() {
		var l varLoan
		if err := rows.Scan(&l.id, &l.loanType, &l.amount, &l.nominalRate, &l.effectiveRate,
			&l.remainingDebt, &l.repaymentPeriod, &l.agreedDate); err != nil {
			log.Printf("loan-service: monthly cron scan error: %v", err)
			continue
		}
		loans = append(loans, l)
	}

	for _, l := range loans {
		// Generate random spread in [-1.50%, +1.50%]
		spread := (rand.Float64()*3.0 - 1.5)
		newEffective := effectiveAnnualRate(l.loanType, l.amount, false, spread)
		// Clamp to avoid negative rates
		if newEffective < 0.01 {
			newEffective = 0.01
		}

		// Recalculate remaining installments
		paidCount, _ := s.paidInstallmentCount(ctx, l.id)
		remaining := l.repaymentPeriod - paidCount
		if remaining <= 0 {
			continue
		}
		newInstallment := monthlyInstallment(l.remainingDebt, newEffective, remaining)

		_, err = s.DB.ExecContext(ctx, `
			UPDATE loans SET effective_rate = $1, next_installment_amount = $2
			WHERE id = $3`, newEffective, newInstallment, l.id)
		if err != nil {
			log.Printf("loan-service: monthly rate update error for loan %d: %v", l.id, err)
			continue
		}
		log.Printf("loan-service: updated variable rate for loan %d → %.4f%% (spread: %+.4f%%)", l.id, newEffective, spread)
	}
}

func (s *LoanServer) paidInstallmentCount(ctx context.Context, loanID int64) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM loan_installments WHERE loan_id = $1 AND status = 'PAID'`, loanID,
	).Scan(&count)
	return count, err
}
