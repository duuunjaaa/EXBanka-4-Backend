package execution

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/order-service/models"
	pb_emp "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/employee"
	pb_exchange "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/exchange"
	pb_loan "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/loan"
)

// ── minimal mock clients ──────────────────────────────────────────────────────

type mockEmpClient struct {
	employee *pb_emp.Employee
	err      error
}

func (m *mockEmpClient) GetAllEmployees(ctx context.Context, in *pb_emp.GetAllEmployeesRequest, opts ...grpc.CallOption) (*pb_emp.GetAllEmployeesResponse, error) {
	return nil, nil
}
func (m *mockEmpClient) SearchEmployees(ctx context.Context, in *pb_emp.SearchEmployeesRequest, opts ...grpc.CallOption) (*pb_emp.SearchEmployeesResponse, error) {
	return nil, nil
}
func (m *mockEmpClient) GetEmployeeCredentials(ctx context.Context, in *pb_emp.GetEmployeeCredentialsRequest, opts ...grpc.CallOption) (*pb_emp.GetEmployeeCredentialsResponse, error) {
	return nil, nil
}
func (m *mockEmpClient) CreateEmployee(ctx context.Context, in *pb_emp.CreateEmployeeRequest, opts ...grpc.CallOption) (*pb_emp.CreateEmployeeResponse, error) {
	return nil, nil
}
func (m *mockEmpClient) GetEmployeeById(ctx context.Context, in *pb_emp.GetEmployeeByIdRequest, opts ...grpc.CallOption) (*pb_emp.GetEmployeeByIdResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &pb_emp.GetEmployeeByIdResponse{Employee: m.employee}, nil
}
func (m *mockEmpClient) UpdateEmployee(ctx context.Context, in *pb_emp.UpdateEmployeeRequest, opts ...grpc.CallOption) (*pb_emp.UpdateEmployeeResponse, error) {
	return nil, nil
}
func (m *mockEmpClient) ActivateEmployee(ctx context.Context, in *pb_emp.ActivateEmployeeRequest, opts ...grpc.CallOption) (*pb_emp.ActivateEmployeeResponse, error) {
	return nil, nil
}
func (m *mockEmpClient) GetEmployeeByEmail(ctx context.Context, in *pb_emp.GetEmployeeByEmailRequest, opts ...grpc.CallOption) (*pb_emp.GetEmployeeByEmailResponse, error) {
	return nil, nil
}
func (m *mockEmpClient) UpdatePassword(ctx context.Context, in *pb_emp.UpdatePasswordRequest, opts ...grpc.CallOption) (*pb_emp.UpdatePasswordResponse, error) {
	return nil, nil
}
func (m *mockEmpClient) GetActuaries(ctx context.Context, in *pb_emp.GetActuariesRequest, opts ...grpc.CallOption) (*pb_emp.GetActuariesResponse, error) {
	return nil, nil
}
func (m *mockEmpClient) SetAgentLimit(ctx context.Context, in *pb_emp.SetAgentLimitRequest, opts ...grpc.CallOption) (*pb_emp.SetAgentLimitResponse, error) {
	return nil, nil
}
func (m *mockEmpClient) ResetAgentUsedLimit(ctx context.Context, in *pb_emp.ResetAgentUsedLimitRequest, opts ...grpc.CallOption) (*pb_emp.ResetAgentUsedLimitResponse, error) {
	return nil, nil
}
func (m *mockEmpClient) SetNeedApproval(ctx context.Context, in *pb_emp.SetNeedApprovalRequest, opts ...grpc.CallOption) (*pb_emp.SetNeedApprovalResponse, error) {
	return nil, nil
}
func (m *mockEmpClient) ResetAllActuaryUsedLimits(ctx context.Context, in *pb_emp.ResetAllActuaryUsedLimitsRequest, opts ...grpc.CallOption) (*pb_emp.ResetAllActuaryUsedLimitsResponse, error) {
	return nil, nil
}
func (m *mockEmpClient) GetActuaryPerformers(ctx context.Context, in *pb_emp.GetActuaryPerformersRequest, opts ...grpc.CallOption) (*pb_emp.GetActuaryPerformersResponse, error) {
	return nil, nil
}

type mockLoanClient struct {
	loans []*pb_loan.LoanSummary
	err   error
}

func (m *mockLoanClient) GetClientLoans(ctx context.Context, in *pb_loan.GetClientLoansRequest, opts ...grpc.CallOption) (*pb_loan.GetClientLoansResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &pb_loan.GetClientLoansResponse{Loans: m.loans}, nil
}
func (m *mockLoanClient) GetLoanDetails(ctx context.Context, in *pb_loan.GetLoanDetailsRequest, opts ...grpc.CallOption) (*pb_loan.GetLoanDetailsResponse, error) {
	return nil, nil
}
func (m *mockLoanClient) GetLoanInstallments(ctx context.Context, in *pb_loan.GetLoanInstallmentsRequest, opts ...grpc.CallOption) (*pb_loan.GetLoanInstallmentsResponse, error) {
	return nil, nil
}
func (m *mockLoanClient) SubmitLoanApplication(ctx context.Context, in *pb_loan.SubmitLoanApplicationRequest, opts ...grpc.CallOption) (*pb_loan.SubmitLoanApplicationResponse, error) {
	return nil, nil
}
func (m *mockLoanClient) ApproveLoan(ctx context.Context, in *pb_loan.ApproveLoanRequest, opts ...grpc.CallOption) (*pb_loan.ApproveLoanResponse, error) {
	return nil, nil
}
func (m *mockLoanClient) RejectLoan(ctx context.Context, in *pb_loan.RejectLoanRequest, opts ...grpc.CallOption) (*pb_loan.RejectLoanResponse, error) {
	return nil, nil
}
func (m *mockLoanClient) GetAllLoanApplications(ctx context.Context, in *pb_loan.GetAllLoanApplicationsRequest, opts ...grpc.CallOption) (*pb_loan.GetAllLoanApplicationsResponse, error) {
	return nil, nil
}
func (m *mockLoanClient) GetAllLoans(ctx context.Context, in *pb_loan.GetAllLoansRequest, opts ...grpc.CallOption) (*pb_loan.GetAllLoansResponse, error) {
	return nil, nil
}
func (m *mockLoanClient) TriggerInstallments(ctx context.Context, in *pb_loan.TriggerInstallmentsRequest, opts ...grpc.CallOption) (*pb_loan.TriggerInstallmentsResponse, error) {
	return nil, nil
}

type mockExchangeClient struct{}

func (m *mockExchangeClient) GetExchangeRates(ctx context.Context, in *pb_exchange.GetExchangeRatesRequest, opts ...grpc.CallOption) (*pb_exchange.GetExchangeRatesResponse, error) {
	return &pb_exchange.GetExchangeRatesResponse{}, nil
}
func (m *mockExchangeClient) ConvertAmount(ctx context.Context, in *pb_exchange.ConvertAmountRequest, opts ...grpc.CallOption) (*pb_exchange.ConvertAmountResponse, error) {
	return nil, nil
}
func (m *mockExchangeClient) GetExchangeHistory(ctx context.Context, in *pb_exchange.GetExchangeHistoryRequest, opts ...grpc.CallOption) (*pb_exchange.GetExchangeHistoryResponse, error) {
	return nil, nil
}
func (m *mockExchangeClient) PreviewConversion(ctx context.Context, in *pb_exchange.PreviewConversionRequest, opts ...grpc.CallOption) (*pb_exchange.PreviewConversionResponse, error) {
	return nil, nil
}

// ── listingCurrency ───────────────────────────────────────────────────────────

func TestListingCurrency_Success(t *testing.T) {
	secDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer secDB.Close()

	mock.ExpectQuery(`SELECT e\.currency`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"currency"}).AddRow("USD"))

	s := &Scheduler{SecuritiesDB: secDB}
	code, err := s.listingCurrency(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "USD", code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListingCurrency_NotFound(t *testing.T) {
	secDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer secDB.Close()

	mock.ExpectQuery(`SELECT e\.currency`).
		WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"currency"}))

	s := &Scheduler{SecuritiesDB: secDB}
	_, err = s.listingCurrency(context.Background(), 99)
	assert.Error(t, err)
}

// ── validateMargin ────────────────────────────────────────────────────────────

func TestValidateMargin_Employee_WithMarginPermission(t *testing.T) {
	accDB, accMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accDB.Close()

	accMock.ExpectQuery(`SELECT available_balance`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"available_balance"}).AddRow(100.0))

	s := &Scheduler{
		AccountDB:      accDB,
		EmployeeClient: &mockEmpClient{employee: &pb_emp.Employee{Permissions: []string{"AGENT", "MARGIN"}}},
	}
	order := models.Order{AccountID: 42, UserType: "EMPLOYEE", UserID: 1}
	// initialMarginCost > balance but MARGIN permission bypasses it
	assert.True(t, s.validateMargin(context.Background(), order, 50000.0))
}

func TestValidateMargin_Employee_NoMarginPermission_Insufficient(t *testing.T) {
	accDB, accMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accDB.Close()

	accMock.ExpectQuery(`SELECT available_balance`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"available_balance"}).AddRow(100.0))

	s := &Scheduler{
		AccountDB:      accDB,
		EmployeeClient: &mockEmpClient{employee: &pb_emp.Employee{Permissions: []string{"AGENT"}}},
	}
	order := models.Order{AccountID: 42, UserType: "EMPLOYEE", UserID: 1}
	assert.False(t, s.validateMargin(context.Background(), order, 50000.0))
}

func TestValidateMargin_Client_SufficientLoan(t *testing.T) {
	accDB, accMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accDB.Close()

	accMock.ExpectQuery(`SELECT available_balance`).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"available_balance"}).AddRow(500.0))

	loan := &pb_loan.LoanSummary{Status: "APPROVED", Amount: 20000.0}
	s := &Scheduler{
		AccountDB:  accDB,
		LoanClient: &mockLoanClient{loans: []*pb_loan.LoanSummary{loan}},
	}
	order := models.Order{AccountID: 10, UserType: "CLIENT", UserID: 5}
	assert.True(t, s.validateMargin(context.Background(), order, 15000.0))
}

func TestValidateMargin_Client_InsufficientFunds(t *testing.T) {
	accDB, accMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accDB.Close()

	accMock.ExpectQuery(`SELECT available_balance`).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"available_balance"}).AddRow(500.0))

	s := &Scheduler{
		AccountDB:  accDB,
		LoanClient: &mockLoanClient{loans: []*pb_loan.LoanSummary{}},
	}
	order := models.Order{AccountID: 10, UserType: "CLIENT", UserID: 5}
	assert.False(t, s.validateMargin(context.Background(), order, 15000.0))
}

// ── settleAccountAndCommission ────────────────────────────────────────────────

func TestSettle_Buy_SameCurrency(t *testing.T) {
	accDB, accMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accDB.Close()

	exchDB, exchMock, err := sqlmock.New()
	require.NoError(t, err)
	defer exchDB.Close()

	// 1. look up account currency_id
	accMock.ExpectQuery(`SELECT currency_id FROM accounts`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"currency_id"}).AddRow(int64(2)))
	// 2. resolve currency code → "USD"
	exchMock.ExpectQuery(`SELECT code FROM currencies`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("USD"))
	// 3. debit buyer (same currency, no conversion)
	accMock.ExpectExec(`UPDATE accounts`).
		WithArgs(-110.0, int64(1), 110.0).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// 4. credit commission to bank account: look up currency id
	exchMock.ExpectQuery(`SELECT id FROM currencies`).
		WithArgs("USD").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))
	// 5. find bank account
	accMock.ExpectQuery(`SELECT id FROM accounts`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))
	// 6. credit bank account
	accMock.ExpectExec(`UPDATE accounts`).
		WithArgs(10.0, int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	s := &Scheduler{
		AccountDB:      accDB,
		ExchangeDB:     exchDB,
		ExchangeClient: &mockExchangeClient{},
	}
	order := models.Order{AccountID: 1, Direction: "BUY", UserType: "EMPLOYEE"}
	err = s.settleAccountAndCommission(context.Background(), order, 100.0, 10.0, "USD")
	require.NoError(t, err)
	assert.NoError(t, accMock.ExpectationsWereMet())
	assert.NoError(t, exchMock.ExpectationsWereMet())
}

func TestSettle_Sell_SameCurrency(t *testing.T) {
	accDB, accMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accDB.Close()

	exchDB, exchMock, err := sqlmock.New()
	require.NoError(t, err)
	defer exchDB.Close()

	accMock.ExpectQuery(`SELECT currency_id FROM accounts`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"currency_id"}).AddRow(int64(2)))
	exchMock.ExpectQuery(`SELECT code FROM currencies`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("USD"))
	// credit seller: totalPrice - commission = 100 - 10 = 90
	accMock.ExpectExec(`UPDATE accounts`).
		WithArgs(90.0, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	exchMock.ExpectQuery(`SELECT id FROM currencies`).
		WithArgs("USD").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))
	accMock.ExpectQuery(`SELECT id FROM accounts`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))
	accMock.ExpectExec(`UPDATE accounts`).
		WithArgs(10.0, int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	s := &Scheduler{
		AccountDB:      accDB,
		ExchangeDB:     exchDB,
		ExchangeClient: &mockExchangeClient{},
	}
	order := models.Order{AccountID: 1, Direction: "SELL", UserType: "EMPLOYEE"}
	err = s.settleAccountAndCommission(context.Background(), order, 100.0, 10.0, "USD")
	require.NoError(t, err)
	assert.NoError(t, accMock.ExpectationsWereMet())
	assert.NoError(t, exchMock.ExpectationsWereMet())
}

func TestSettle_Buy_InsufficientFunds(t *testing.T) {
	accDB, accMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accDB.Close()

	exchDB, exchMock, err := sqlmock.New()
	require.NoError(t, err)
	defer exchDB.Close()

	accMock.ExpectQuery(`SELECT currency_id FROM accounts`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"currency_id"}).AddRow(int64(2)))
	exchMock.ExpectQuery(`SELECT code FROM currencies`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("USD"))
	// UPDATE returns 0 rows (insufficient funds)
	accMock.ExpectExec(`UPDATE accounts`).
		WithArgs(-110.0, int64(1), 110.0).
		WillReturnResult(sqlmock.NewResult(0, 0))

	s := &Scheduler{AccountDB: accDB, ExchangeDB: exchDB, ExchangeClient: &mockExchangeClient{}}
	order := models.Order{AccountID: 1, Direction: "BUY", UserType: "EMPLOYEE"}
	err = s.settleAccountAndCommission(context.Background(), order, 100.0, 10.0, "USD")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient funds")
}

// ── declineExpiredOrders ──────────────────────────────────────────────────────

func TestDeclineExpiredOrders_NoPendingOrders(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT id, user_id`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "user_type", "asset_id", "order_type", "quantity", "contract_size",
			"price_per_unit", "limit_value", "stop_value", "direction", "status", "approved_by",
			"is_done", "last_modification", "remaining_portions", "after_hours", "is_aon", "is_margin", "account_id",
		}))

	secDB, secMock, err := sqlmock.New()
	require.NoError(t, err)
	defer secDB.Close()

	s := &Scheduler{DB: db, SecuritiesDB: secDB}
	s.declineExpiredOrders()
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.NoError(t, secMock.ExpectationsWereMet())
}

func TestDeclineExpiredOrders_ExpiredFutures(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	pastDate := time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02")

	mock.ExpectQuery(`SELECT id, user_id`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "user_type", "asset_id", "order_type", "quantity", "contract_size",
			"price_per_unit", "limit_value", "stop_value", "direction", "status", "approved_by",
			"is_done", "last_modification", "remaining_portions", "after_hours", "is_aon", "is_margin", "account_id",
		}).AddRow(
			int64(7), int64(1), "EMPLOYEE", int64(10), "MARKET", int32(1), int32(1),
			100.0, nil, nil, "BUY", "PENDING", nil,
			false, time.Now(), int32(1), false, false, false, int64(1),
		))

	secDB, secMock, err := sqlmock.New()
	require.NoError(t, err)
	defer secDB.Close()

	secMock.ExpectQuery(`SELECT settlement_date`).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"settlement_date"}).AddRow(pastDate))

	// DeclineOrder: fetch order status, then update
	mock.ExpectQuery(`SELECT id, user_id`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "user_type", "asset_id", "order_type", "quantity", "contract_size",
			"price_per_unit", "limit_value", "stop_value", "direction", "status", "approved_by",
			"is_done", "last_modification", "remaining_portions", "after_hours", "is_aon", "is_margin", "account_id",
		}).AddRow(
			int64(7), int64(1), "EMPLOYEE", int64(10), "MARKET", int32(1), int32(1),
			100.0, nil, nil, "BUY", "PENDING", nil,
			false, time.Now(), int32(1), false, false, false, int64(1),
		))
	mock.ExpectExec(`UPDATE orders SET status`).
		WithArgs("DECLINED", nil, sqlmock.AnyArg(), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	s := &Scheduler{DB: db, SecuritiesDB: secDB}
	s.declineExpiredOrders()
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.NoError(t, secMock.ExpectationsWereMet())
}
