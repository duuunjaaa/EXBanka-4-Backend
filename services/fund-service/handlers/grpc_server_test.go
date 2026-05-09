package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb_account "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/account"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/fund"
)

// ---- mock AccountServiceClient ----

type mockAccountClient struct {
	createAccountFn func(context.Context, *pb_account.CreateAccountRequest, ...grpc.CallOption) (*pb_account.CreateAccountResponse, error)
}

func (m *mockAccountClient) CreateAccount(ctx context.Context, in *pb_account.CreateAccountRequest, opts ...grpc.CallOption) (*pb_account.CreateAccountResponse, error) {
	if m.createAccountFn != nil {
		return m.createAccountFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (m *mockAccountClient) GetMyAccounts(ctx context.Context, in *pb_account.GetMyAccountsRequest, opts ...grpc.CallOption) (*pb_account.GetMyAccountsResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockAccountClient) GetAccount(ctx context.Context, in *pb_account.GetAccountRequest, opts ...grpc.CallOption) (*pb_account.GetAccountResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockAccountClient) RenameAccount(ctx context.Context, in *pb_account.RenameAccountRequest, opts ...grpc.CallOption) (*pb_account.RenameAccountResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockAccountClient) GetAllAccounts(ctx context.Context, in *pb_account.GetAllAccountsRequest, opts ...grpc.CallOption) (*pb_account.GetAllAccountsResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockAccountClient) UpdateAccountLimits(ctx context.Context, in *pb_account.UpdateAccountLimitsRequest, opts ...grpc.CallOption) (*pb_account.UpdateAccountLimitsResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockAccountClient) DeleteAccount(ctx context.Context, in *pb_account.DeleteAccountRequest, opts ...grpc.CallOption) (*pb_account.DeleteAccountResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockAccountClient) GetBankAccounts(ctx context.Context, in *pb_account.GetBankAccountsRequest, opts ...grpc.CallOption) (*pb_account.GetBankAccountsResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// ---- helpers ----

// newFundServer creates a FundServer with mocked fund DB, account DB, and employee DB.
func newFundServer(t *testing.T, acctClient pb_account.AccountServiceClient) (*FundServer, sqlmock.Sqlmock, sqlmock.Sqlmock, sqlmock.Sqlmock) {
	t.Helper()
	fundDB, fundMock, err := sqlmock.New()
	require.NoError(t, err)
	accountDB, accountDBMock, err := sqlmock.New()
	require.NoError(t, err)
	employeeDB, empMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = fundDB.Close()
		_ = accountDB.Close()
		_ = employeeDB.Close()
	})
	return &FundServer{
		DB:            fundDB,
		AccountDB:     accountDB,
		EmployeeDB:    employeeDB,
		AccountClient: acctClient,
	}, fundMock, accountDBMock, empMock
}

// fundDetailColumns returns the columns scanned by fetchFundByID.
func fundDetailColumns() []string {
	return []string{"id", "name", "description", "minimum_contribution", "manager_id",
		"liquid_assets", "account_id", "created_at", "active"}
}

// addFetchFundRows sets up mock expectations for fetchFundByID with includeAccountNumber=true.
// It expects: SELECT fund, SUM profit, employee name, account number.
func addFetchFundRows(fundMock, accountDBMock, empMock sqlmock.Sqlmock,
	id int64, name string, accountID int64, managerID int64) {
	now := time.Now()
	fundMock.ExpectQuery("SELECT id, name, description").
		WillReturnRows(sqlmock.NewRows(fundDetailColumns()).
			AddRow(id, name,
				sql.NullString{String: "A test fund", Valid: true},
				float64(1000), managerID,
				float64(500000),
				sql.NullInt64{Int64: accountID, Valid: true},
				now, true))
	// SUM of invested
	fundMock.ExpectQuery("SELECT COALESCE").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(float64(0)))
	// manager name
	empMock.ExpectQuery("SELECT first_name").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Jane Manager"))
	// account number
	accountDBMock.ExpectQuery("SELECT account_number").
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow("123-456-78"))
}

// addFetchFundRowsNoAccount sets up mock expectations for fetchFundByID with includeAccountNumber=false
// (used in ListFunds).
func addFetchFundRowsNoAccount(fundMock, empMock sqlmock.Sqlmock,
	id int64, name string, accountID int64, managerID int64) {
	now := time.Now()
	fundMock.ExpectQuery("SELECT id, name, description").
		WillReturnRows(sqlmock.NewRows(fundDetailColumns()).
			AddRow(id, name,
				sql.NullString{String: "A test fund", Valid: true},
				float64(1000), managerID,
				float64(500000),
				sql.NullInt64{Int64: accountID, Valid: true},
				now, true))
	// SUM of invested
	fundMock.ExpectQuery("SELECT COALESCE").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(float64(0)))
	// manager name
	empMock.ExpectQuery("SELECT first_name").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Jane Manager"))
}

// ---- TestCreateFund_Happy ----

func TestCreateFund_Happy(t *testing.T) {
	acct := &mockAccountClient{
		createAccountFn: func(_ context.Context, _ *pb_account.CreateAccountRequest, _ ...grpc.CallOption) (*pb_account.CreateAccountResponse, error) {
			return &pb_account.CreateAccountResponse{
				Account: &pb_account.AccountResponse{
					Id:            100,
					AccountNumber: "123-456-78",
				},
			}, nil
		},
	}
	s, fundMock, accountDBMock, empMock := newFundServer(t, acct)

	// INSERT fund returns id=1
	fundMock.ExpectQuery("INSERT INTO investment_funds").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	// fetchFundByID with includeAccountNumber=true
	addFetchFundRows(fundMock, accountDBMock, empMock, 1, "Test Fund", 100, 5)

	resp, err := s.CreateFund(context.Background(), &pb.CreateFundRequest{
		Name:                "Test Fund",
		Description:         "A test fund",
		MinimumContribution: 1000.0,
		ManagerId:           5,
		CreatedById:         1,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Id)
	assert.Equal(t, "Test Fund", resp.Name)
	assert.Equal(t, int64(100), resp.AccountId)
}

// ---- TestCreateFund_MissingName ----

func TestCreateFund_MissingName(t *testing.T) {
	acct := &mockAccountClient{}
	s, _, _, _ := newFundServer(t, acct)

	_, err := s.CreateFund(context.Background(), &pb.CreateFundRequest{
		Name:      "",
		ManagerId: 5,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// ---- TestCreateFund_DuplicateName ----

func TestCreateFund_DuplicateName(t *testing.T) {
	acct := &mockAccountClient{
		createAccountFn: func(_ context.Context, _ *pb_account.CreateAccountRequest, _ ...grpc.CallOption) (*pb_account.CreateAccountResponse, error) {
			return &pb_account.CreateAccountResponse{
				Account: &pb_account.AccountResponse{Id: 101, AccountNumber: "222-333-44"},
			}, nil
		},
	}
	s, fundMock, _, _ := newFundServer(t, acct)

	// INSERT returns unique violation
	fundMock.ExpectQuery("INSERT INTO investment_funds").
		WillReturnError(fmt.Errorf("pq: duplicate key value violates unique constraint: 23505"))

	_, err := s.CreateFund(context.Background(), &pb.CreateFundRequest{
		Name:                "Existing Fund",
		MinimumContribution: 500,
		ManagerId:           5,
	})
	require.Error(t, err)
	assert.Equal(t, codes.AlreadyExists, status.Code(err))
}

// ---- TestListFunds_Empty ----

func TestListFunds_Empty(t *testing.T) {
	s, fundMock, _, _ := newFundServer(t, &mockAccountClient{})

	fundMock.ExpectQuery("SELECT id FROM investment_funds").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	resp, err := s.ListFunds(context.Background(), &pb.ListFundsRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Funds, 0)
}

// ---- TestListFunds_WithManagerFilter ----

func TestListFunds_WithManagerFilter(t *testing.T) {
	s, fundMock, _, empMock := newFundServer(t, &mockAccountClient{})

	// Returns 1 fund ID
	fundMock.ExpectQuery("SELECT id FROM investment_funds").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	// fetchFundByID with includeAccountNumber=false (ListFunds)
	addFetchFundRowsNoAccount(fundMock, empMock, 1, "Manager Fund", 100, 3)

	resp, err := s.ListFunds(context.Background(), &pb.ListFundsRequest{ManagerIdFilter: 3})
	require.NoError(t, err)
	assert.Len(t, resp.Funds, 1)
}

// ---- TestDeleteFund_HasPositions ----

func TestDeleteFund_HasPositions(t *testing.T) {
	s, fundMock, _, _ := newFundServer(t, &mockAccountClient{})

	// COUNT returns 1 (active position exists)
	fundMock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

	_, err := s.DeleteFund(context.Background(), &pb.DeleteFundRequest{Id: 1})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

// ---- TestDeleteFund_NotFound ----

func TestDeleteFund_NotFound(t *testing.T) {
	s, fundMock, _, _ := newFundServer(t, &mockAccountClient{})

	// COUNT returns 0 (no active positions)
	fundMock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	// UPDATE returns 0 rows affected
	fundMock.ExpectExec("UPDATE investment_funds SET active = false").
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, err := s.DeleteFund(context.Background(), &pb.DeleteFundRequest{Id: 99})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// ---- TestDeleteFund_Happy ----

func TestDeleteFund_Happy(t *testing.T) {
	s, fundMock, _, _ := newFundServer(t, &mockAccountClient{})

	// COUNT returns 0
	fundMock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	// UPDATE returns 1 row affected
	fundMock.ExpectExec("UPDATE investment_funds SET active = false").
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err := s.DeleteFund(context.Background(), &pb.DeleteFundRequest{Id: 1})
	require.NoError(t, err)
}

// ---- additional: TestListFunds_Happy (two funds) ----

func TestListFunds_Happy(t *testing.T) {
	s, fundMock, _, empMock := newFundServer(t, &mockAccountClient{})

	// Two fund IDs returned
	fundMock.ExpectQuery("SELECT id FROM investment_funds").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)).AddRow(int64(2)))

	// fetchFundByID for fund 1
	addFetchFundRowsNoAccount(fundMock, empMock, 1, "Fund A", 100, 5)
	// fetchFundByID for fund 2
	addFetchFundRowsNoAccount(fundMock, empMock, 2, "Fund B", 101, 5)

	resp, err := s.ListFunds(context.Background(), &pb.ListFundsRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Funds, 2)
}

// ---- TestGetFund_NotFound ----

func TestGetFund_NotFound(t *testing.T) {
	s, fundMock, _, _ := newFundServer(t, &mockAccountClient{})

	fundMock.ExpectQuery("SELECT id, name, description").
		WillReturnRows(sqlmock.NewRows(fundDetailColumns()))

	_, err := s.GetFund(context.Background(), &pb.GetFundRequest{Id: 999})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// ---- TestGetFund_Happy ----

func TestGetFund_Happy(t *testing.T) {
	s, fundMock, accountDBMock, empMock := newFundServer(t, &mockAccountClient{})

	addFetchFundRows(fundMock, accountDBMock, empMock, 1, "Happy Fund", 100, 5)

	resp, err := s.GetFund(context.Background(), &pb.GetFundRequest{Id: 1})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Id)
	assert.Equal(t, "Happy Fund", resp.Name)
	assert.Greater(t, resp.FundValue, float64(0))
}

// ---- TestUpdateFund_NotFound ----

func TestUpdateFund_NotFound(t *testing.T) {
	s, fundMock, _, _ := newFundServer(t, &mockAccountClient{})

	// UPDATE executes OK
	fundMock.ExpectExec("UPDATE investment_funds").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// fetchFundByID returns no rows
	fundMock.ExpectQuery("SELECT id, name, description").
		WillReturnRows(sqlmock.NewRows(fundDetailColumns()))

	_, err := s.UpdateFund(context.Background(), &pb.UpdateFundRequest{
		Id:        999,
		Name:      "No Fund",
		ManagerId: 5,
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// ---- TestUpdateFund_Happy ----

func TestUpdateFund_Happy(t *testing.T) {
	s, fundMock, accountDBMock, empMock := newFundServer(t, &mockAccountClient{})

	fundMock.ExpectExec("UPDATE investment_funds").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// fetchFundByID with includeAccountNumber=true
	addFetchFundRows(fundMock, accountDBMock, empMock, 1, "Updated Fund", 100, 5)

	resp, err := s.UpdateFund(context.Background(), &pb.UpdateFundRequest{
		Id:                  1,
		Name:                "Updated Fund",
		Description:         "new desc",
		MinimumContribution: 2000.0,
		ManagerId:           5,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Fund", resp.Name)
}

// ---- TestCreateFund_AccountClientError ----

func TestCreateFund_AccountClientError(t *testing.T) {
	acct := &mockAccountClient{
		createAccountFn: func(_ context.Context, _ *pb_account.CreateAccountRequest, _ ...grpc.CallOption) (*pb_account.CreateAccountResponse, error) {
			return nil, fmt.Errorf("account service unavailable")
		},
	}
	s, _, _, _ := newFundServer(t, acct)

	_, err := s.CreateFund(context.Background(), &pb.CreateFundRequest{
		Name:                "New Fund",
		MinimumContribution: 1000.0,
		ManagerId:           5,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ---- TestPing ----

func TestPing_FundService(t *testing.T) {
	s, _, _, _ := newFundServer(t, &mockAccountClient{})
	resp, err := s.Ping(context.Background(), &pb.PingRequest{})
	require.NoError(t, err)
	assert.Equal(t, "fund-service ok", resp.Message)
}

// ---- TestCreateFund_MissingManagerId ----

func TestCreateFund_MissingManagerId(t *testing.T) {
	s, _, _, _ := newFundServer(t, &mockAccountClient{})
	_, err := s.CreateFund(context.Background(), &pb.CreateFundRequest{
		Name:      "Fund Without Manager",
		ManagerId: 0,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// ---- TestCreateFund_DBError (non-unique internal error) ----

func TestCreateFund_DBError(t *testing.T) {
	acct := &mockAccountClient{
		createAccountFn: func(_ context.Context, _ *pb_account.CreateAccountRequest, _ ...grpc.CallOption) (*pb_account.CreateAccountResponse, error) {
			return &pb_account.CreateAccountResponse{
				Account: &pb_account.AccountResponse{Id: 200, AccountNumber: "999-000-01"},
			}, nil
		},
	}
	s, fundMock, _, _ := newFundServer(t, acct)

	fundMock.ExpectQuery("INSERT INTO investment_funds").
		WillReturnError(fmt.Errorf("connection reset by peer"))

	_, err := s.CreateFund(context.Background(), &pb.CreateFundRequest{
		Name:                "Error Fund",
		MinimumContribution: 1000.0,
		ManagerId:           5,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ---- TestListFunds_DBError ----

func TestListFunds_DBError(t *testing.T) {
	s, fundMock, _, _ := newFundServer(t, &mockAccountClient{})
	fundMock.ExpectQuery("SELECT id FROM investment_funds").
		WillReturnError(sql.ErrConnDone)

	_, err := s.ListFunds(context.Background(), &pb.ListFundsRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ---- TestUpdateFund_IdZero ----

func TestUpdateFund_IdZero(t *testing.T) {
	s, _, _, _ := newFundServer(t, &mockAccountClient{})
	_, err := s.UpdateFund(context.Background(), &pb.UpdateFundRequest{Id: 0, Name: "X", ManagerId: 1})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// ---- TestUpdateFund_DBError ----

func TestUpdateFund_DBError(t *testing.T) {
	s, fundMock, _, _ := newFundServer(t, &mockAccountClient{})
	fundMock.ExpectExec("UPDATE investment_funds").
		WillReturnError(fmt.Errorf("connection reset by peer"))

	_, err := s.UpdateFund(context.Background(), &pb.UpdateFundRequest{
		Id:        1,
		Name:      "Some Fund",
		ManagerId: 5,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ---- TestUpdateFund_DuplicateName ----

func TestUpdateFund_DuplicateName(t *testing.T) {
	s, fundMock, _, _ := newFundServer(t, &mockAccountClient{})
	fundMock.ExpectExec("UPDATE investment_funds").
		WillReturnError(fmt.Errorf("pq: duplicate key value violates unique constraint: 23505"))

	_, err := s.UpdateFund(context.Background(), &pb.UpdateFundRequest{
		Id:        2,
		Name:      "Existing Fund",
		ManagerId: 5,
	})
	require.Error(t, err)
	assert.Equal(t, codes.AlreadyExists, status.Code(err))
}

// ---- TestDeleteFund_IdZero ----

func TestDeleteFund_IdZero(t *testing.T) {
	s, _, _, _ := newFundServer(t, &mockAccountClient{})
	_, err := s.DeleteFund(context.Background(), &pb.DeleteFundRequest{Id: 0})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// ---- TestDeleteFund_CountDBError ----

func TestDeleteFund_CountDBError(t *testing.T) {
	s, fundMock, _, _ := newFundServer(t, &mockAccountClient{})
	fundMock.ExpectQuery("SELECT COUNT").
		WillReturnError(sql.ErrConnDone)

	_, err := s.DeleteFund(context.Background(), &pb.DeleteFundRequest{Id: 1})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── GetBankPositions ──────────────────────────────────────────────────────────

func TestGetBankPositions_Empty(t *testing.T) {
	s, fundMock, _, _ := newFundServer(t, &mockAccountClient{})

	fundMock.ExpectQuery(`SELECT cfp\.fund_id`).
		WillReturnRows(sqlmock.NewRows([]string{"fund_id", "bank_invested", "name", "fund_value", "manager_id", "total_all_invested"}))

	resp, err := s.GetBankPositions(context.Background(), &pb.GetBankPositionsRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Positions)
	require.NoError(t, fundMock.ExpectationsWereMet())
}

func TestGetBankPositions_Happy(t *testing.T) {
	// Bank invested 1000, total all invested 4000, fund value 8000.
	// bankShareRSD = (1000/4000)*8000 = 2000
	// bankSharePercent = (2000/8000)*100 = 25%
	// profitRSD = 2000 - 1000 = 1000
	s, fundMock, _, empMock := newFundServer(t, &mockAccountClient{})

	fundMock.ExpectQuery(`SELECT cfp\.fund_id`).
		WillReturnRows(sqlmock.NewRows([]string{"fund_id", "bank_invested", "name", "fund_value", "manager_id", "total_all_invested"}).
			AddRow(int64(1), 1000.0, "RAF Growth Fund", 8000.0, int64(5), 4000.0))
	empMock.ExpectQuery(`SELECT first_name`).
		WillReturnRows(sqlmock.NewRows([]string{"manager_name"}).AddRow("Ana Jovanovic"))

	resp, err := s.GetBankPositions(context.Background(), &pb.GetBankPositionsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Positions, 1)
	pos := resp.Positions[0]
	assert.Equal(t, int64(1), pos.FundId)
	assert.Equal(t, "RAF Growth Fund", pos.FundName)
	assert.Equal(t, "Ana Jovanovic", pos.ManagerName)
	assert.InDelta(t, 25.0, pos.BankSharePercent, 0.01)
	assert.InDelta(t, 2000.0, pos.BankShareRsd, 0.01)
	assert.InDelta(t, 1000.0, pos.ProfitRsd, 0.01)
	require.NoError(t, fundMock.ExpectationsWereMet())
}

func TestGetBankPositions_ZeroFundValue(t *testing.T) {
	// Fund has no value yet — bankSharePercent should be 0, not divide-by-zero.
	s, fundMock, _, _ := newFundServer(t, &mockAccountClient{})

	fundMock.ExpectQuery(`SELECT cfp\.fund_id`).
		WillReturnRows(sqlmock.NewRows([]string{"fund_id", "bank_invested", "name", "fund_value", "manager_id", "total_all_invested"}).
			AddRow(int64(2), 500.0, "Empty Fund", 0.0, int64(0), 500.0))

	resp, err := s.GetBankPositions(context.Background(), &pb.GetBankPositionsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Positions, 1)
	assert.InDelta(t, 0.0, resp.Positions[0].BankSharePercent, 0.001)
	assert.InDelta(t, 0.0, resp.Positions[0].BankShareRsd, 0.001)
	require.NoError(t, fundMock.ExpectationsWereMet())
}

func TestGetBankPositions_DBError(t *testing.T) {
	s, fundMock, _, _ := newFundServer(t, &mockAccountClient{})

	fundMock.ExpectQuery(`SELECT cfp\.fund_id`).WillReturnError(sql.ErrConnDone)

	_, err := s.GetBankPositions(context.Background(), &pb.GetBankPositionsRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}
