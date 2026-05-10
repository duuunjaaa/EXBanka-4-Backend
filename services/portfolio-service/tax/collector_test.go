package tax

import (
	"context"
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
