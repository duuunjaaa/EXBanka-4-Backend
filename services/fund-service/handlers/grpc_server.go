package handlers

import (
	"context"
	"database/sql"
	"strings"
	"time"

	pb_account "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/account"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/fund"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FundServer struct {
	pb.UnimplementedFundServiceServer
	DB            *sql.DB // fund_db
	AccountDB     *sql.DB // account_db
	EmployeeDB    *sql.DB // employee_db
	AccountClient pb_account.AccountServiceClient
}

func (s *FundServer) Ping(_ context.Context, _ *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{Message: "fund-service ok"}, nil
}

func (s *FundServer) CreateFund(ctx context.Context, req *pb.CreateFundRequest) (*pb.FundResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.ManagerId == 0 {
		return nil, status.Error(codes.InvalidArgument, "manager_id is required")
	}

	// Create a bank account for this fund
	accountResp, err := s.AccountClient.CreateAccount(ctx, &pb_account.CreateAccountRequest{
		ClientId:       0,
		AccountType:    "BANK",
		AccountSubtype: "FUND",
		CurrencyCode:   "RSD",
		InitialBalance: 0,
		AccountName:    "Fund: " + req.Name,
		CreateCard:     false,
		EmployeeId:     req.CreatedById,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create fund account: %v", err)
	}

	accountID := accountResp.GetAccount().GetId()

	var id int64
	err = s.DB.QueryRowContext(ctx, `
		INSERT INTO investment_funds (name, description, minimum_contribution, manager_id, account_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		req.Name, req.Description, req.MinimumContribution, req.ManagerId, accountID,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, status.Errorf(codes.AlreadyExists, "fund with name %q already exists", req.Name)
		}
		return nil, status.Errorf(codes.Internal, "failed to create fund: %v", err)
	}

	return s.fetchFundByID(ctx, id, true)
}

func (s *FundServer) ListFunds(ctx context.Context, req *pb.ListFundsRequest) (*pb.ListFundsResponse, error) {
	query := `SELECT id FROM investment_funds WHERE active = true`
	args := []interface{}{}

	if req.ManagerIdFilter != 0 {
		args = append(args, req.ManagerIdFilter)
		query += ` AND manager_id = $1`
	}
	query += ` ORDER BY name ASC`

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list funds: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to scan fund id: %v", err)
		}
		ids = append(ids, id)
	}

	funds := make([]*pb.FundResponse, 0, len(ids))
	for _, id := range ids {
		f, err := s.fetchFundByID(ctx, id, false)
		if err != nil {
			return nil, err
		}
		funds = append(funds, f)
	}
	return &pb.ListFundsResponse{Funds: funds}, nil
}

func (s *FundServer) GetFund(ctx context.Context, req *pb.GetFundRequest) (*pb.FundResponse, error) {
	return s.fetchFundByID(ctx, req.Id, true)
}

func (s *FundServer) UpdateFund(ctx context.Context, req *pb.UpdateFundRequest) (*pb.FundResponse, error) {
	if req.Id == 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	_, err := s.DB.ExecContext(ctx, `
		UPDATE investment_funds
		SET name = $1, description = $2, minimum_contribution = $3, manager_id = $4
		WHERE id = $5 AND active = true`,
		req.Name, req.Description, req.MinimumContribution, req.ManagerId, req.Id,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, status.Errorf(codes.AlreadyExists, "fund with name %q already exists", req.Name)
		}
		return nil, status.Errorf(codes.Internal, "failed to update fund: %v", err)
	}

	return s.fetchFundByID(ctx, req.Id, true)
}

func (s *FundServer) DeleteFund(ctx context.Context, req *pb.DeleteFundRequest) (*pb.DeleteFundResponse, error) {
	if req.Id == 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// Check for active positions
	var count int64
	err := s.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM client_fund_positions
		WHERE fund_id = $1 AND total_invested_amount > 0`, req.Id,
	).Scan(&count)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check fund positions: %v", err)
	}
	if count > 0 {
		return nil, status.Error(codes.PermissionDenied, "cannot delete fund with active client positions")
	}

	res, err := s.DB.ExecContext(ctx, `UPDATE investment_funds SET active = false WHERE id = $1 AND active = true`, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete fund: %v", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, status.Error(codes.NotFound, "fund not found")
	}

	return &pb.DeleteFundResponse{}, nil
}

func (s *FundServer) fetchFundByID(ctx context.Context, id int64, includeAccountNumber bool) (*pb.FundResponse, error) {
	var f pb.FundResponse
	var description sql.NullString
	var accountID sql.NullInt64
	var createdAt time.Time

	err := s.DB.QueryRowContext(ctx, `
		SELECT id, name, description, minimum_contribution, manager_id,
		       liquid_assets, account_id, created_at, active
		FROM investment_funds WHERE id = $1`, id,
	).Scan(
		&f.Id, &f.Name, &description, &f.MinimumContribution, &f.ManagerId,
		&f.LiquidAssets, &accountID, &createdAt, &f.Active,
	)
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "fund not found")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch fund: %v", err)
	}

	if description.Valid {
		f.Description = description.String
	}
	if accountID.Valid {
		f.AccountId = accountID.Int64
	}
	f.CreatedAt = createdAt.Format(time.RFC3339)

	// fund_value = liquid_assets (no portfolio positions in Sprint 1)
	f.FundValue = f.LiquidAssets

	// profit = fund_value - total invested
	var totalInvested float64
	_ = s.DB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(total_invested_amount), 0)
		FROM client_fund_positions WHERE fund_id = $1`, id,
	).Scan(&totalInvested)
	f.Profit = f.FundValue - totalInvested

	// manager name
	if f.ManagerId != 0 {
		var managerName string
		_ = s.EmployeeDB.QueryRowContext(ctx,
			`SELECT first_name || ' ' || last_name FROM employees WHERE id = $1`, f.ManagerId,
		).Scan(&managerName)
		f.ManagerName = managerName
	}

	// account number (only needed for GetFund, not list)
	if includeAccountNumber && f.AccountId != 0 {
		var accountNumber string
		_ = s.AccountDB.QueryRowContext(ctx,
			`SELECT account_number FROM accounts WHERE id = $1`, f.AccountId,
		).Scan(&accountNumber)
		f.AccountNumber = accountNumber
	}

	return &f, nil
}

func (s *FundServer) InvestFund(ctx context.Context, req *pb.InvestFundRequest) (*pb.FundResponse, error) {
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be greater than 0")
	}

	var minimumContribution float64
	var active bool
	err := s.DB.QueryRowContext(ctx,
		`SELECT minimum_contribution, active FROM investment_funds WHERE id = $1`, req.FundId,
	).Scan(&minimumContribution, &active)
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "fund not found")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch fund: %v", err)
	}
	if !active {
		return nil, status.Error(codes.NotFound, "fund not found")
	}
	if req.Amount < minimumContribution {
		return nil, status.Errorf(codes.InvalidArgument, "amount %.2f is below minimum contribution %.2f", req.Amount, minimumContribution)
	}

	var availableBalance float64
	err = s.AccountDB.QueryRowContext(ctx,
		`SELECT available_balance FROM accounts WHERE id = $1`, req.SourceAccountId,
	).Scan(&availableBalance)
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "source account not found")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch account: %v", err)
	}
	if availableBalance < req.Amount {
		return nil, status.Error(codes.FailedPrecondition, "insufficient account balance")
	}

	_, err = s.AccountDB.ExecContext(ctx,
		`UPDATE accounts SET balance = balance - $1, available_balance = available_balance - $1 WHERE id = $2`,
		req.Amount, req.SourceAccountId,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to debit account: %v", err)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		// compensate
		_, _ = s.AccountDB.ExecContext(ctx,
			`UPDATE accounts SET balance = balance + $1, available_balance = available_balance + $1 WHERE id = $2`,
			req.Amount, req.SourceAccountId,
		)
		return nil, status.Errorf(codes.Internal, "failed to begin transaction: %v", err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE investment_funds SET liquid_assets = liquid_assets + $1 WHERE id = $2`,
		req.Amount, req.FundId,
	)
	if err != nil {
		_ = tx.Rollback()
		_, _ = s.AccountDB.ExecContext(ctx,
			`UPDATE accounts SET balance = balance + $1, available_balance = available_balance + $1 WHERE id = $2`,
			req.Amount, req.SourceAccountId,
		)
		return nil, status.Errorf(codes.Internal, "failed to update fund liquid assets: %v", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO client_fund_positions (client_id, client_type, fund_id, total_invested_amount)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (client_id, client_type, fund_id) DO UPDATE
		SET total_invested_amount = client_fund_positions.total_invested_amount + EXCLUDED.total_invested_amount,
		    last_modified_at = NOW()`,
		req.ClientId, req.ClientType, req.FundId, req.Amount,
	)
	if err != nil {
		_ = tx.Rollback()
		_, _ = s.AccountDB.ExecContext(ctx,
			`UPDATE accounts SET balance = balance + $1, available_balance = available_balance + $1 WHERE id = $2`,
			req.Amount, req.SourceAccountId,
		)
		return nil, status.Errorf(codes.Internal, "failed to upsert position: %v", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO client_fund_transactions (client_id, client_type, fund_id, amount, is_inflow, status) VALUES ($1, $2, $3, $4, true, 'COMPLETED')`,
		req.ClientId, req.ClientType, req.FundId, req.Amount,
	)
	if err != nil {
		_ = tx.Rollback()
		_, _ = s.AccountDB.ExecContext(ctx,
			`UPDATE accounts SET balance = balance + $1, available_balance = available_balance + $1 WHERE id = $2`,
			req.Amount, req.SourceAccountId,
		)
		return nil, status.Errorf(codes.Internal, "failed to insert transaction: %v", err)
	}

	if err = tx.Commit(); err != nil {
		_, _ = s.AccountDB.ExecContext(ctx,
			`UPDATE accounts SET balance = balance + $1, available_balance = available_balance + $1 WHERE id = $2`,
			req.Amount, req.SourceAccountId,
		)
		return nil, status.Errorf(codes.Internal, "failed to commit: %v", err)
	}

	return s.fetchFundByID(ctx, req.FundId, true)
}

func (s *FundServer) WithdrawFund(ctx context.Context, req *pb.WithdrawFundRequest) (*pb.FundResponse, error) {
	var liquidAssets float64
	var active bool
	err := s.DB.QueryRowContext(ctx,
		`SELECT liquid_assets, active FROM investment_funds WHERE id = $1`, req.FundId,
	).Scan(&liquidAssets, &active)
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "fund not found")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch fund: %v", err)
	}
	if !active {
		return nil, status.Error(codes.NotFound, "fund not found")
	}

	var positionAmount float64
	err = s.DB.QueryRowContext(ctx,
		`SELECT total_invested_amount FROM client_fund_positions WHERE fund_id = $1 AND client_id = $2 AND client_type = $3`,
		req.FundId, req.ClientId, req.ClientType,
	).Scan(&positionAmount)
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "no position in this fund")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch position: %v", err)
	}

	amount := req.Amount
	if amount == 0 {
		amount = positionAmount
	}
	if amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "nothing to withdraw")
	}
	if amount > positionAmount {
		return nil, status.Errorf(codes.InvalidArgument, "withdrawal amount %.2f exceeds position %.2f", amount, positionAmount)
	}
	if amount > liquidAssets {
		return nil, status.Error(codes.FailedPrecondition, "insufficient fund liquidity")
	}

	_, err = s.AccountDB.ExecContext(ctx,
		`UPDATE accounts SET balance = balance + $1, available_balance = available_balance + $1 WHERE id = $2`,
		amount, req.DestinationAccountId,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to credit account: %v", err)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		_, _ = s.AccountDB.ExecContext(ctx,
			`UPDATE accounts SET balance = balance - $1, available_balance = available_balance - $1 WHERE id = $2`,
			amount, req.DestinationAccountId,
		)
		return nil, status.Errorf(codes.Internal, "failed to begin transaction: %v", err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE investment_funds SET liquid_assets = liquid_assets - $1 WHERE id = $2`,
		amount, req.FundId,
	)
	if err != nil {
		_ = tx.Rollback()
		_, _ = s.AccountDB.ExecContext(ctx,
			`UPDATE accounts SET balance = balance - $1, available_balance = available_balance - $1 WHERE id = $2`,
			amount, req.DestinationAccountId,
		)
		return nil, status.Errorf(codes.Internal, "failed to update fund liquid assets: %v", err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE client_fund_positions SET total_invested_amount = total_invested_amount - $1, last_modified_at = NOW() WHERE fund_id = $2 AND client_id = $3 AND client_type = $4`,
		amount, req.FundId, req.ClientId, req.ClientType,
	)
	if err != nil {
		_ = tx.Rollback()
		_, _ = s.AccountDB.ExecContext(ctx,
			`UPDATE accounts SET balance = balance - $1, available_balance = available_balance - $1 WHERE id = $2`,
			amount, req.DestinationAccountId,
		)
		return nil, status.Errorf(codes.Internal, "failed to update position: %v", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO client_fund_transactions (client_id, client_type, fund_id, amount, is_inflow, status) VALUES ($1, $2, $3, $4, false, 'COMPLETED')`,
		req.ClientId, req.ClientType, req.FundId, amount,
	)
	if err != nil {
		_ = tx.Rollback()
		_, _ = s.AccountDB.ExecContext(ctx,
			`UPDATE accounts SET balance = balance - $1, available_balance = available_balance - $1 WHERE id = $2`,
			amount, req.DestinationAccountId,
		)
		return nil, status.Errorf(codes.Internal, "failed to insert transaction: %v", err)
	}

	if err = tx.Commit(); err != nil {
		_, _ = s.AccountDB.ExecContext(ctx,
			`UPDATE accounts SET balance = balance - $1, available_balance = available_balance - $1 WHERE id = $2`,
			amount, req.DestinationAccountId,
		)
		return nil, status.Errorf(codes.Internal, "failed to commit: %v", err)
	}

	return s.fetchFundByID(ctx, req.FundId, true)
}

// isUniqueViolation checks if the error is a PostgreSQL unique constraint violation (error code 23505).
func isUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "unique constraint")
}

func (s *FundServer) ValidateFundAccount(ctx context.Context, req *pb.ValidateFundAccountRequest) (*pb.ValidateFundAccountResponse, error) {
	var accountID, managerID int64
	var liquidAssets float64
	err := s.DB.QueryRowContext(ctx,
		`SELECT account_id, manager_id, liquid_assets FROM investment_funds WHERE id = $1 AND active = TRUE`,
		req.FundId,
	).Scan(&accountID, &managerID, &liquidAssets)
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "fund not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "validate fund account: %v", err)
	}
	if managerID != req.ManagerId {
		return nil, status.Error(codes.PermissionDenied, "not the fund manager")
	}
	return &pb.ValidateFundAccountResponse{
		AccountId:   accountID,
		IsLiquid:    liquidAssets >= req.RequiredAmount,
		LiquidAssets: liquidAssets,
	}, nil
}

func (s *FundServer) UpdateFundHolding(ctx context.Context, req *pb.UpdateFundHoldingRequest) (*pb.UpdateFundHoldingResponse, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	tradeValue := float64(req.Quantity) * req.Price

	if req.Direction == "BUY" {
		_, err = tx.ExecContext(ctx,
			`UPDATE investment_funds SET liquid_assets = liquid_assets - $1 WHERE id = $2`,
			tradeValue, req.FundId)
	} else {
		_, err = tx.ExecContext(ctx,
			`UPDATE investment_funds SET liquid_assets = liquid_assets + $1 WHERE id = $2`,
			tradeValue, req.FundId)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update liquid_assets: %v", err)
	}

	quantityDelta := req.Quantity
	if req.Direction == "SELL" {
		quantityDelta = -req.Quantity
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO fund_portfolio_positions (fund_id, listing_id, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (fund_id, listing_id) DO UPDATE
		  SET quantity = fund_portfolio_positions.quantity + EXCLUDED.quantity`,
		req.FundId, req.ListingId, quantityDelta)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "upsert fund position: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, status.Errorf(codes.Internal, "commit: %v", err)
	}
	return &pb.UpdateFundHoldingResponse{}, nil
}

// GetBankPositions returns all investment fund positions held by the bank (client_type='BANK', client_id=0).
// Used by the bank profit portal (#234).
func (s *FundServer) GetBankPositions(ctx context.Context, _ *pb.GetBankPositionsRequest) (*pb.GetBankPositionsResponse, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT cfp.fund_id,
		       cfp.total_invested_amount                                         AS bank_invested,
		       f.name,
		       f.liquid_assets                                                   AS fund_value,
		       f.manager_id,
		       COALESCE((SELECT SUM(total_invested_amount)
		                 FROM client_fund_positions
		                 WHERE fund_id = cfp.fund_id), 0)                        AS total_all_invested
		FROM client_fund_positions cfp
		JOIN investment_funds f ON f.id = cfp.fund_id
		WHERE cfp.client_type = 'BANK'
		  AND cfp.client_id   = 0
		  AND f.active        = TRUE`)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get bank positions: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var positions []*pb.BankFundPosition
	for rows.Next() {
		var (
			fundID           int64
			bankInvested     float64
			fundName         string
			fundValue        float64
			managerID        int64
			totalAllInvested float64
		)
		if err := rows.Scan(&fundID, &bankInvested, &fundName, &fundValue, &managerID, &totalAllInvested); err != nil {
			return nil, status.Errorf(codes.Internal, "scan bank position: %v", err)
		}

		var bankShareRSD float64
		if totalAllInvested > 0 {
			bankShareRSD = (bankInvested / totalAllInvested) * fundValue
		}
		var bankSharePercent float64
		if fundValue > 0 {
			bankSharePercent = (bankShareRSD / fundValue) * 100
		}

		var managerName string
		if managerID != 0 {
			_ = s.EmployeeDB.QueryRowContext(ctx,
				`SELECT first_name || ' ' || last_name FROM employees WHERE id = $1`, managerID,
			).Scan(&managerName)
		}

		positions = append(positions, &pb.BankFundPosition{
			FundId:           fundID,
			FundName:         fundName,
			ManagerName:      managerName,
			BankSharePercent: bankSharePercent,
			BankShareRsd:     bankShareRSD,
			ProfitRsd:        bankShareRSD - bankInvested,
		})
	}
	if positions == nil {
		positions = []*pb.BankFundPosition{}
	}
	return &pb.GetBankPositionsResponse{Positions: positions}, nil
}
