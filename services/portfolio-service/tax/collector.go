package tax

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/portfolio-service/repository"
	pb_ex "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/exchange"
)

// CollectUnpaid deducts all unpaid tax records from the relevant accounts and
// credits the collected amount to the state's RSD account.
// Pass userID=0 and userType="" to process all users (supervisor/scheduled job).
func CollectUnpaid(ctx context.Context, portfolioDB, accountDB, exchangeDB *sql.DB, exchangeClient pb_ex.ExchangeServiceClient, userID int64, userType string) error {
	var (
		records []repository.TaxRecord
		err     error
	)
	if userID == 0 {
		records, err = repository.GetUnpaidRecords(ctx, portfolioDB)
	} else {
		records, err = repository.GetUnpaidRecordsForUser(ctx, portfolioDB, userID, userType)
	}
	if err != nil {
		return fmt.Errorf("load unpaid records: %w", err)
	}

	// Cache exchange rates for the duration of this collection run.
	var rates map[string]float64

	for _, rec := range records {
		accountID, err := getAccountForUser(ctx, portfolioDB, rec.UserID, rec.UserType)
		if err != nil {
			continue
		}

		currencyCode, err := getAccountCurrency(ctx, accountDB, exchangeDB, accountID)
		if err != nil {
			continue
		}

		deductAmount := rec.AmountRSD
		if currencyCode != "RSD" {
			if rates == nil {
				rates, err = fetchMiddleRates(ctx, exchangeClient)
				if err != nil {
					return fmt.Errorf("fetch exchange rates: %w", err)
				}
			}
			middleRate, ok := rates[currencyCode]
			if !ok || middleRate == 0 {
				continue
			}
			deductAmount = rec.AmountRSD / middleRate
		}

		if err := deductFromAccount(ctx, accountDB, accountID, deductAmount); err != nil {
			continue
		}

		_ = repository.MarkTaxPaid(ctx, portfolioDB, rec.ID)

		// Credit the state's RSD account with the full RSD amount (no FX conversion —
		// the state account is always RSD).
		_ = creditStateAccount(ctx, accountDB, rec.AmountRSD)
	}
	return nil
}

func getAccountForUser(ctx context.Context, db *sql.DB, userID int64, userType string) (int64, error) {
	uid := userID
	if userType == "EMPLOYEE" {
		uid = 0
	}
	var accountID int64
	err := db.QueryRowContext(ctx, `
		SELECT account_id FROM portfolio_entry
		WHERE user_id = $1 AND user_type = $2
		LIMIT 1`,
		uid, userType,
	).Scan(&accountID)
	return accountID, err
}

func getAccountCurrency(ctx context.Context, accountDB, exchangeDB *sql.DB, accountID int64) (string, error) {
	var currencyID int64
	if err := accountDB.QueryRowContext(ctx,
		`SELECT currency_id FROM accounts WHERE id = $1`, accountID,
	).Scan(&currencyID); err != nil {
		return "", err
	}
	var code string
	if err := exchangeDB.QueryRowContext(ctx,
		`SELECT code FROM currencies WHERE id = $1`, currencyID,
	).Scan(&code); err != nil {
		return "", fmt.Errorf("currency_id %d not found in exchange db: %w", currencyID, err)
	}
	return code, nil
}

func deductFromAccount(ctx context.Context, accountDB *sql.DB, accountID int64, amount float64) error {
	_, err := accountDB.ExecContext(ctx, `
		UPDATE accounts
		SET balance = balance - $1, available_balance = available_balance - $1
		WHERE id = $2`,
		amount, accountID,
	)
	return err
}

func creditStateAccount(ctx context.Context, accountDB *sql.DB, amountRSD float64) error {
	_, err := accountDB.ExecContext(ctx, `
		UPDATE accounts
		SET balance = balance + $1, available_balance = available_balance + $1
		WHERE account_type = 'STATE'`,
		amountRSD,
	)
	return err
}

func fetchMiddleRates(ctx context.Context, client pb_ex.ExchangeServiceClient) (map[string]float64, error) {
	resp, err := client.GetExchangeRates(ctx, &pb_ex.GetExchangeRatesRequest{})
	if err != nil {
		return nil, err
	}
	m := make(map[string]float64, len(resp.Rates))
	for _, r := range resp.Rates {
		m[r.CurrencyCode] = r.MiddleRate
	}
	return m, nil
}
