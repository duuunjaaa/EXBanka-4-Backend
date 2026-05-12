package tax

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	pb_ex "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/exchange"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// mockExchangeClient satisfies pb_ex.ExchangeServiceClient.
type mockExchangeClient struct {
	rates []*pb_ex.ExchangeRate
}

func (m *mockExchangeClient) GetExchangeRates(_ context.Context, _ *pb_ex.GetExchangeRatesRequest, _ ...grpc.CallOption) (*pb_ex.GetExchangeRatesResponse, error) {
	return &pb_ex.GetExchangeRatesResponse{Rates: m.rates}, nil
}

func (m *mockExchangeClient) ConvertAmount(_ context.Context, _ *pb_ex.ConvertAmountRequest, _ ...grpc.CallOption) (*pb_ex.ConvertAmountResponse, error) {
	return &pb_ex.ConvertAmountResponse{}, nil
}

func (m *mockExchangeClient) GetExchangeHistory(_ context.Context, _ *pb_ex.GetExchangeHistoryRequest, _ ...grpc.CallOption) (*pb_ex.GetExchangeHistoryResponse, error) {
	return &pb_ex.GetExchangeHistoryResponse{}, nil
}

func (m *mockExchangeClient) PreviewConversion(_ context.Context, _ *pb_ex.PreviewConversionRequest, _ ...grpc.CallOption) (*pb_ex.PreviewConversionResponse, error) {
	return &pb_ex.PreviewConversionResponse{}, nil
}

func TestCollectUnpaid_RSD_SingleRecord(t *testing.T) {
	portfolioDB, pMock, err := sqlmock.New()
	require.NoError(t, err)
	defer portfolioDB.Close()

	accountDB, aMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accountDB.Close()

	exchangeDB, eMock, err := sqlmock.New()
	require.NoError(t, err)
	defer exchangeDB.Close()

	// unpaid records query
	pMock.ExpectQuery(`SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}).
			AddRow(int64(1), int64(10), "CLIENT", 100.0, 4, 2026, false, nil))

	// account_id lookup in portfolioDB
	pMock.ExpectQuery(`SELECT account_id FROM portfolio_entry`).
		WithArgs(int64(10), "CLIENT").
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(int64(42)))

	// currency_id lookup in accountDB
	aMock.ExpectQuery(`SELECT currency_id FROM accounts`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"currency_id"}).AddRow(int64(1)))

	// currency code lookup in exchangeDB
	eMock.ExpectQuery(`SELECT code FROM currencies`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))

	// deduct from account
	aMock.ExpectExec(`UPDATE accounts`).
		WithArgs(100.0, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// mark paid
	pMock.ExpectExec(`UPDATE tax_record SET is_paid`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// credit state account
	aMock.ExpectExec(`UPDATE accounts`).
		WithArgs(100.0).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = CollectUnpaid(context.Background(), portfolioDB, accountDB, exchangeDB, &mockExchangeClient{}, 0, "")
	require.NoError(t, err)
	assert.NoError(t, pMock.ExpectationsWereMet())
	assert.NoError(t, aMock.ExpectationsWereMet())
	assert.NoError(t, eMock.ExpectationsWereMet())
}

func TestCollectUnpaid_ForeignCurrency_ConvertsToRSD(t *testing.T) {
	portfolioDB, pMock, err := sqlmock.New()
	require.NoError(t, err)
	defer portfolioDB.Close()

	accountDB, aMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accountDB.Close()

	exchangeDB, eMock, err := sqlmock.New()
	require.NoError(t, err)
	defer exchangeDB.Close()

	exchangeClient := &mockExchangeClient{
		rates: []*pb_ex.ExchangeRate{
			{CurrencyCode: "USD", MiddleRate: 110.0},
		},
	}

	pMock.ExpectQuery(`SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}).
			AddRow(int64(2), int64(11), "CLIENT", 220.0, 4, 2026, false, nil))

	pMock.ExpectQuery(`SELECT account_id FROM portfolio_entry`).
		WithArgs(int64(11), "CLIENT").
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(int64(55)))

	// currency_id lookup in accountDB
	aMock.ExpectQuery(`SELECT currency_id FROM accounts`).
		WithArgs(int64(55)).
		WillReturnRows(sqlmock.NewRows([]string{"currency_id"}).AddRow(int64(4)))

	// currency code lookup in exchangeDB
	eMock.ExpectQuery(`SELECT code FROM currencies`).
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("USD"))

	// 220 RSD / 110 middle rate = 2 USD
	aMock.ExpectExec(`UPDATE accounts`).
		WithArgs(2.0, int64(55)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	pMock.ExpectExec(`UPDATE tax_record SET is_paid`).
		WithArgs(int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// credit state account with original RSD amount
	aMock.ExpectExec(`UPDATE accounts`).
		WithArgs(220.0).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = CollectUnpaid(context.Background(), portfolioDB, accountDB, exchangeDB, exchangeClient, 0, "")
	require.NoError(t, err)
	assert.NoError(t, pMock.ExpectationsWereMet())
	assert.NoError(t, aMock.ExpectationsWereMet())
	assert.NoError(t, eMock.ExpectationsWereMet())
}

func TestCollectUnpaid_NoRecords(t *testing.T) {
	portfolioDB, pMock, err := sqlmock.New()
	require.NoError(t, err)
	defer portfolioDB.Close()

	accountDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer accountDB.Close()

	exchangeDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer exchangeDB.Close()

	pMock.ExpectQuery(`SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}))

	err = CollectUnpaid(context.Background(), portfolioDB, accountDB, exchangeDB, &mockExchangeClient{}, 0, "")
	require.NoError(t, err)
	assert.NoError(t, pMock.ExpectationsWereMet())
}

// ── mockExchangeClientErr returns an error from GetExchangeRates ──────────────

type mockExchangeClientErr struct{}

func (m *mockExchangeClientErr) GetExchangeRates(_ context.Context, _ *pb_ex.GetExchangeRatesRequest, _ ...grpc.CallOption) (*pb_ex.GetExchangeRatesResponse, error) {
	return nil, fmt.Errorf("exchange service unavailable")
}
func (m *mockExchangeClientErr) ConvertAmount(_ context.Context, _ *pb_ex.ConvertAmountRequest, _ ...grpc.CallOption) (*pb_ex.ConvertAmountResponse, error) {
	return &pb_ex.ConvertAmountResponse{}, nil
}
func (m *mockExchangeClientErr) GetExchangeHistory(_ context.Context, _ *pb_ex.GetExchangeHistoryRequest, _ ...grpc.CallOption) (*pb_ex.GetExchangeHistoryResponse, error) {
	return &pb_ex.GetExchangeHistoryResponse{}, nil
}
func (m *mockExchangeClientErr) PreviewConversion(_ context.Context, _ *pb_ex.PreviewConversionRequest, _ ...grpc.CallOption) (*pb_ex.PreviewConversionResponse, error) {
	return &pb_ex.PreviewConversionResponse{}, nil
}

// ── fetchMiddleRates error ────────────────────────────────────────────────────

func TestFetchMiddleRates_Error(t *testing.T) {
	_, err := fetchMiddleRates(context.Background(), &mockExchangeClientErr{})
	assert.Error(t, err)
}

func TestFetchMiddleRates_Success(t *testing.T) {
	client := &mockExchangeClient{
		rates: []*pb_ex.ExchangeRate{
			{CurrencyCode: "EUR", MiddleRate: 117.0},
		},
	}
	rates, err := fetchMiddleRates(context.Background(), client)
	require.NoError(t, err)
	assert.Equal(t, 117.0, rates["EUR"])
}

// ── getAccountCurrency exchangeDB error ───────────────────────────────────────

func TestGetAccountCurrency_ExchangeDBError(t *testing.T) {
	accountDB, aMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accountDB.Close()

	exchangeDB, eMock, err := sqlmock.New()
	require.NoError(t, err)
	defer exchangeDB.Close()

	aMock.ExpectQuery(`SELECT currency_id FROM accounts`).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"currency_id"}).AddRow(int64(3)))

	eMock.ExpectQuery(`SELECT code FROM currencies`).
		WithArgs(int64(3)).
		WillReturnError(sql.ErrNoRows)

	_, err = getAccountCurrency(context.Background(), accountDB, exchangeDB, 10)
	assert.Error(t, err)
}

func TestGetAccountCurrency_AccountDBError(t *testing.T) {
	accountDB, aMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accountDB.Close()

	exchangeDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer exchangeDB.Close()

	aMock.ExpectQuery(`SELECT currency_id FROM accounts`).
		WithArgs(int64(10)).
		WillReturnError(sql.ErrConnDone)

	_, err = getAccountCurrency(context.Background(), accountDB, exchangeDB, 10)
	assert.ErrorIs(t, err, sql.ErrConnDone)
}

// ── getAccountForUser EMPLOYEE branch ────────────────────────────────────────

func TestGetAccountForUser_Employee(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// EMPLOYEE → uid=0
	mock.ExpectQuery(`SELECT account_id FROM portfolio_entry`).
		WithArgs(int64(0), "EMPLOYEE").
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(int64(7)))

	id, err := getAccountForUser(context.Background(), db, 99, "EMPLOYEE")
	require.NoError(t, err)
	assert.Equal(t, int64(7), id)
}

func TestGetAccountForUser_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT account_id FROM portfolio_entry`).
		WithArgs(int64(10), "CLIENT").
		WillReturnError(sql.ErrNoRows)

	_, err = getAccountForUser(context.Background(), db, 10, "CLIENT")
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

// ── CollectUnpaid error paths ─────────────────────────────────────────────────

func TestCollectUnpaid_LoadError(t *testing.T) {
	portfolioDB, pMock, err := sqlmock.New()
	require.NoError(t, err)
	defer portfolioDB.Close()

	accountDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer accountDB.Close()

	exchangeDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer exchangeDB.Close()

	pMock.ExpectQuery(`SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at`).
		WillReturnError(sql.ErrConnDone)

	err = CollectUnpaid(context.Background(), portfolioDB, accountDB, exchangeDB, &mockExchangeClient{}, 0, "")
	assert.Error(t, err)
}

func TestCollectUnpaid_PerUser_Path(t *testing.T) {
	portfolioDB, pMock, err := sqlmock.New()
	require.NoError(t, err)
	defer portfolioDB.Close()

	accountDB, aMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accountDB.Close()

	exchangeDB, eMock, err := sqlmock.New()
	require.NoError(t, err)
	defer exchangeDB.Close()

	// userID != 0 → GetUnpaidRecordsForUser
	pMock.ExpectQuery(`SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}).
			AddRow(int64(1), int64(10), "CLIENT", 50.0, 3, 2026, false, nil))

	pMock.ExpectQuery(`SELECT account_id FROM portfolio_entry`).
		WithArgs(int64(10), "CLIENT").
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(int64(20)))

	aMock.ExpectQuery(`SELECT currency_id FROM accounts`).
		WithArgs(int64(20)).
		WillReturnRows(sqlmock.NewRows([]string{"currency_id"}).AddRow(int64(1)))

	eMock.ExpectQuery(`SELECT code FROM currencies`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))

	aMock.ExpectExec(`UPDATE accounts`).
		WithArgs(50.0, int64(20)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	pMock.ExpectExec(`UPDATE tax_record SET is_paid`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	aMock.ExpectExec(`UPDATE accounts`).
		WithArgs(50.0).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = CollectUnpaid(context.Background(), portfolioDB, accountDB, exchangeDB, &mockExchangeClient{}, 10, "CLIENT")
	require.NoError(t, err)
}

func TestCollectUnpaid_GetAccountForUser_Skips(t *testing.T) {
	// getAccountForUser returns error → record is skipped (continue)
	portfolioDB, pMock, err := sqlmock.New()
	require.NoError(t, err)
	defer portfolioDB.Close()

	accountDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer accountDB.Close()

	exchangeDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer exchangeDB.Close()

	pMock.ExpectQuery(`SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}).
			AddRow(int64(1), int64(10), "CLIENT", 100.0, 4, 2026, false, nil))

	pMock.ExpectQuery(`SELECT account_id FROM portfolio_entry`).
		WithArgs(int64(10), "CLIENT").
		WillReturnError(sql.ErrNoRows)

	err = CollectUnpaid(context.Background(), portfolioDB, accountDB, exchangeDB, &mockExchangeClient{}, 0, "")
	require.NoError(t, err)
}

func TestCollectUnpaid_GetAccountCurrency_Skips(t *testing.T) {
	// getAccountCurrency returns error → record is skipped
	portfolioDB, pMock, err := sqlmock.New()
	require.NoError(t, err)
	defer portfolioDB.Close()

	accountDB, aMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accountDB.Close()

	exchangeDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer exchangeDB.Close()

	pMock.ExpectQuery(`SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}).
			AddRow(int64(1), int64(10), "CLIENT", 100.0, 4, 2026, false, nil))

	pMock.ExpectQuery(`SELECT account_id FROM portfolio_entry`).
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(int64(42)))

	aMock.ExpectQuery(`SELECT currency_id FROM accounts`).
		WillReturnError(sql.ErrConnDone)

	err = CollectUnpaid(context.Background(), portfolioDB, accountDB, exchangeDB, &mockExchangeClient{}, 0, "")
	require.NoError(t, err)
}

func TestCollectUnpaid_FetchRatesError_ReturnsError(t *testing.T) {
	// fetchMiddleRates fails → CollectUnpaid returns error
	portfolioDB, pMock, err := sqlmock.New()
	require.NoError(t, err)
	defer portfolioDB.Close()

	accountDB, aMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accountDB.Close()

	exchangeDB, eMock, err := sqlmock.New()
	require.NoError(t, err)
	defer exchangeDB.Close()

	pMock.ExpectQuery(`SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}).
			AddRow(int64(1), int64(10), "CLIENT", 100.0, 4, 2026, false, nil))

	pMock.ExpectQuery(`SELECT account_id FROM portfolio_entry`).
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(int64(42)))

	aMock.ExpectQuery(`SELECT currency_id FROM accounts`).
		WillReturnRows(sqlmock.NewRows([]string{"currency_id"}).AddRow(int64(4)))

	eMock.ExpectQuery(`SELECT code FROM currencies`).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("USD"))

	err = CollectUnpaid(context.Background(), portfolioDB, accountDB, exchangeDB, &mockExchangeClientErr{}, 0, "")
	assert.Error(t, err)
}

func TestCollectUnpaid_MissingExchangeRate_Skips(t *testing.T) {
	// Rate for currency not in map → record is skipped
	portfolioDB, pMock, err := sqlmock.New()
	require.NoError(t, err)
	defer portfolioDB.Close()

	accountDB, aMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accountDB.Close()

	exchangeDB, eMock, err := sqlmock.New()
	require.NoError(t, err)
	defer exchangeDB.Close()

	exchangeClient := &mockExchangeClient{
		rates: []*pb_ex.ExchangeRate{
			{CurrencyCode: "EUR", MiddleRate: 117.0},
			// CHF not present → !ok branch
		},
	}

	pMock.ExpectQuery(`SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}).
			AddRow(int64(1), int64(10), "CLIENT", 100.0, 4, 2026, false, nil))

	pMock.ExpectQuery(`SELECT account_id FROM portfolio_entry`).
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(int64(42)))

	aMock.ExpectQuery(`SELECT currency_id FROM accounts`).
		WillReturnRows(sqlmock.NewRows([]string{"currency_id"}).AddRow(int64(5)))

	eMock.ExpectQuery(`SELECT code FROM currencies`).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("CHF"))

	err = CollectUnpaid(context.Background(), portfolioDB, accountDB, exchangeDB, exchangeClient, 0, "")
	require.NoError(t, err)
}

func TestCollectUnpaid_ZeroMiddleRate_Skips(t *testing.T) {
	// middleRate == 0 → skipped
	portfolioDB, pMock, err := sqlmock.New()
	require.NoError(t, err)
	defer portfolioDB.Close()

	accountDB, aMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accountDB.Close()

	exchangeDB, eMock, err := sqlmock.New()
	require.NoError(t, err)
	defer exchangeDB.Close()

	exchangeClient := &mockExchangeClient{
		rates: []*pb_ex.ExchangeRate{
			{CurrencyCode: "USD", MiddleRate: 0},
		},
	}

	pMock.ExpectQuery(`SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}).
			AddRow(int64(1), int64(10), "CLIENT", 100.0, 4, 2026, false, nil))

	pMock.ExpectQuery(`SELECT account_id FROM portfolio_entry`).
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(int64(42)))

	aMock.ExpectQuery(`SELECT currency_id FROM accounts`).
		WillReturnRows(sqlmock.NewRows([]string{"currency_id"}).AddRow(int64(4)))

	eMock.ExpectQuery(`SELECT code FROM currencies`).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("USD"))

	err = CollectUnpaid(context.Background(), portfolioDB, accountDB, exchangeDB, exchangeClient, 0, "")
	require.NoError(t, err)
}

func TestCollectUnpaid_DeductFromAccount_Skips(t *testing.T) {
	// deductFromAccount fails → record is skipped, no error returned
	portfolioDB, pMock, err := sqlmock.New()
	require.NoError(t, err)
	defer portfolioDB.Close()

	accountDB, aMock, err := sqlmock.New()
	require.NoError(t, err)
	defer accountDB.Close()

	exchangeDB, eMock, err := sqlmock.New()
	require.NoError(t, err)
	defer exchangeDB.Close()

	pMock.ExpectQuery(`SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}).
			AddRow(int64(1), int64(10), "CLIENT", 100.0, 4, 2026, false, nil))

	pMock.ExpectQuery(`SELECT account_id FROM portfolio_entry`).
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(int64(42)))

	aMock.ExpectQuery(`SELECT currency_id FROM accounts`).
		WillReturnRows(sqlmock.NewRows([]string{"currency_id"}).AddRow(int64(1)))

	eMock.ExpectQuery(`SELECT code FROM currencies`).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))

	// deductFromAccount UPDATE fails → continue, no error propagated
	aMock.ExpectExec(`UPDATE accounts`).
		WithArgs(100.0, int64(42)).
		WillReturnError(sql.ErrConnDone)

	err = CollectUnpaid(context.Background(), portfolioDB, accountDB, exchangeDB, &mockExchangeClient{}, 0, "")
	require.NoError(t, err)
}
