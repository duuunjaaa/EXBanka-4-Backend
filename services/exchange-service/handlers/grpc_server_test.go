package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/exchange"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func newExchangeServer(t *testing.T) (*ExchangeServer, sqlmock.Sqlmock, sqlmock.Sqlmock) {
	t.Helper()
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	accountDB, accountMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(); _ = accountDB.Close() })
	return &ExchangeServer{DB: db, AccountDB: accountDB}, dbMock, accountMock
}

// expectRatesAlreadyExist makes the mock respond to ensureTodayRates with count > 0 (no fetch needed).
func expectRatesAlreadyExist(dbMock sqlmock.Sqlmock) {
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM daily_exchange_rates WHERE date`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))
}

func rateColumns() []string {
	return []string{"currency_code", "buying_rate", "selling_rate", "middle_rate", "date"}
}

func sampleRateRows() *sqlmock.Rows {
	today := time.Now()
	return sqlmock.NewRows(rateColumns()).
		AddRow("EUR", 115.50, 118.50, 117.00, today).
		AddRow("USD", 107.00, 110.00, 108.50, today)
}

// ── GetExchangeRates ──────────────────────────────────────────────────────────

func TestGetExchangeRates_Happy(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT currency_code, buying_rate, selling_rate, middle_rate, date`).
		WillReturnRows(sampleRateRows())

	resp, err := s.GetExchangeRates(context.Background(), &pb.GetExchangeRatesRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Rates, 2)
	assert.Equal(t, "EUR", resp.Rates[0].CurrencyCode)
	assert.Equal(t, 115.50, resp.Rates[0].BuyingRate)
	assert.Equal(t, 118.50, resp.Rates[0].SellingRate)
}

func TestGetExchangeRates_Empty(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT currency_code, buying_rate, selling_rate, middle_rate, date`).
		WillReturnRows(sqlmock.NewRows(rateColumns()))

	resp, err := s.GetExchangeRates(context.Background(), &pb.GetExchangeRatesRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Rates)
}

func TestGetExchangeRates_QueryError(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT currency_code, buying_rate, selling_rate, middle_rate, date`).
		WillReturnError(sql.ErrConnDone)

	_, err := s.GetExchangeRates(context.Background(), &pb.GetExchangeRatesRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestGetExchangeRates_EnsureRatesError(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	// ensureTodayRates COUNT fails → logged, GetExchangeRates still proceeds
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM daily_exchange_rates WHERE date`).
		WillReturnError(sql.ErrConnDone)
	dbMock.ExpectQuery(`SELECT currency_code, buying_rate, selling_rate, middle_rate, date`).
		WillReturnRows(sqlmock.NewRows(rateColumns()))

	resp, err := s.GetExchangeRates(context.Background(), &pb.GetExchangeRatesRequest{})
	require.NoError(t, err) // error is only logged
	assert.Empty(t, resp.Rates)
}

func TestGetExchangeRates_ScanError(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	expectRatesAlreadyExist(dbMock)
	// Return "not-a-float" for buying_rate → scan into float64 fails
	dbMock.ExpectQuery(`SELECT currency_code, buying_rate, selling_rate, middle_rate, date`).
		WillReturnRows(sqlmock.NewRows(rateColumns()).
			AddRow("EUR", "not-a-float", 118.50, 117.00, time.Now()))

	_, err := s.GetExchangeRates(context.Background(), &pb.GetExchangeRatesRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── PreviewConversion ─────────────────────────────────────────────────────────

func TestPreviewConversion_InvalidAmount(t *testing.T) {
	s, _, _ := newExchangeServer(t)

	_, err := s.PreviewConversion(context.Background(), &pb.PreviewConversionRequest{
		FromCurrency: "RSD", ToCurrency: "EUR", Amount: 0,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestPreviewConversion_SameCurrency(t *testing.T) {
	s, _, _ := newExchangeServer(t)

	_, err := s.PreviewConversion(context.Background(), &pb.PreviewConversionRequest{
		FromCurrency: "EUR", ToCurrency: "EUR", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestPreviewConversion_RSDtoEUR(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	expectRatesAlreadyExist(dbMock)
	// RSD → EUR: needs selling_rate for EUR
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"}).AddRow(118.50))

	resp, err := s.PreviewConversion(context.Background(), &pb.PreviewConversionRequest{
		FromCurrency: "RSD", ToCurrency: "EUR", Amount: 11850,
	})
	require.NoError(t, err)
	assert.Equal(t, "RSD", resp.FromCurrency)
	assert.Equal(t, "EUR", resp.ToCurrency)
	assert.Equal(t, 11850.0, resp.FromAmount)
	// 11850 / 118.50 * 0.995 ≈ 99.50
	assert.InDelta(t, 99.50, resp.ToAmount, 0.01)
}

func TestPreviewConversion_EURtoRSD(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	expectRatesAlreadyExist(dbMock)
	// EUR → RSD: needs buying_rate for EUR
	dbMock.ExpectQuery(`SELECT buying_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"buying_rate"}).AddRow(115.50))

	resp, err := s.PreviewConversion(context.Background(), &pb.PreviewConversionRequest{
		FromCurrency: "EUR", ToCurrency: "RSD", Amount: 100,
	})
	require.NoError(t, err)
	// 100 * 115.50 * 0.995 ≈ 11492.25
	assert.InDelta(t, 11492.25, resp.ToAmount, 0.01)
	assert.Equal(t, 115.50, resp.Rate)
}

func TestPreviewConversion_EURtoUSD(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	expectRatesAlreadyExist(dbMock)
	// EUR → USD (two-step: EUR buying, USD selling)
	dbMock.ExpectQuery(`SELECT buying_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"buying_rate"}).AddRow(115.50))
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WithArgs("USD").
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"}).AddRow(110.00))

	resp, err := s.PreviewConversion(context.Background(), &pb.PreviewConversionRequest{
		FromCurrency: "EUR", ToCurrency: "USD", Amount: 100,
	})
	require.NoError(t, err)
	// rsd = 100 * 115.50 * 0.995 ≈ 11492.25
	// to  = 11492.25 / 110.00 * 0.995 ≈ 103.87
	assert.InDelta(t, 103.87, resp.ToAmount, 0.1)
}

func TestPreviewConversion_EnsureRatesError(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	// ensureTodayRates COUNT fails → logged, PreviewConversion still proceeds
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM daily_exchange_rates WHERE date`).
		WillReturnError(sql.ErrConnDone)
	// getRate("EUR", "selling_rate") succeeds
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"}).AddRow(118.50))

	resp, err := s.PreviewConversion(context.Background(), &pb.PreviewConversionRequest{
		FromCurrency: "RSD", ToCurrency: "EUR", Amount: 11850,
	})
	require.NoError(t, err) // ensureRates error is only logged
	assert.InDelta(t, 99.50, resp.ToAmount, 0.01)
}

func TestPreviewConversion_ToRSDRateError(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	expectRatesAlreadyExist(dbMock)
	// EUR → RSD: getRate("EUR", "buying_rate") fails
	dbMock.ExpectQuery(`SELECT buying_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"buying_rate"})) // empty → ErrNoRows

	_, err := s.PreviewConversion(context.Background(), &pb.PreviewConversionRequest{
		FromCurrency: "EUR", ToCurrency: "RSD", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestPreviewConversion_DefaultFromRateError(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	expectRatesAlreadyExist(dbMock)
	// EUR → USD (default), getRate("EUR", "buying_rate") fails
	dbMock.ExpectQuery(`SELECT buying_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"buying_rate"})) // empty → ErrNoRows

	_, err := s.PreviewConversion(context.Background(), &pb.PreviewConversionRequest{
		FromCurrency: "EUR", ToCurrency: "USD", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestPreviewConversion_DefaultToRateError(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT buying_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"buying_rate"}).AddRow(115.50))
	// getRate("USD", "selling_rate") fails
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WithArgs("USD").
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"})) // empty → ErrNoRows

	_, err := s.PreviewConversion(context.Background(), &pb.PreviewConversionRequest{
		FromCurrency: "EUR", ToCurrency: "USD", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestPreviewConversion_RateNotFound(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"})) // empty → ErrNoRows

	_, err := s.PreviewConversion(context.Background(), &pb.PreviewConversionRequest{
		FromCurrency: "RSD", ToCurrency: "EUR", Amount: 1000,
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// ── ConvertAmount ─────────────────────────────────────────────────────────────

func TestConvertAmount_NegativeAmount(t *testing.T) {
	s, _, _ := newExchangeServer(t)

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: -100, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestConvertAmount_SameAccount(t *testing.T) {
	s, _, _ := newExchangeServer(t)

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC001", Amount: 100, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestConvertAmount_SourceNotFound(t *testing.T) {
	s, _, accountMock := newExchangeServer(t)

	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}))

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestConvertAmount_WrongOwner(t *testing.T) {
	s, _, accountMock := newExchangeServer(t)

	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(99), 50000.0, int64(1))) // owner is 99, caller is 1

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestConvertAmount_DestNotFound(t *testing.T) {
	s, _, accountMock := newExchangeServer(t)

	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 50000.0, int64(1)))

	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}))

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestConvertAmount_SameCurrencyError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)

	// Both accounts have currency_id = 1
	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 50000.0, int64(1)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(1)))

	// Both resolve to the same currency code
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestConvertAmount_InsufficientFunds(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)

	// From: RSD (currency_id=1, balance=100), To: EUR (currency_id=2)
	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 100.0, int64(1)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(2)))

	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("EUR"))

	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"}).AddRow(118.50))

	// amount=5000 > available_balance=100
	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 5000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestConvertAmount_RSDtoEUR_Happy(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)

	// From: RSD (currency_id=1), To: EUR (currency_id=2)
	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 50000.0, int64(1)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(2)))

	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("EUR"))

	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"}).AddRow(118.50))

	// Bank intermediary accounts
	accountMock.ExpectQuery(`SELECT account_number FROM accounts WHERE owner_id = 0`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow("BANK-RSD"))
	accountMock.ExpectQuery(`SELECT account_number FROM accounts WHERE owner_id = 0`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow("BANK-EUR"))

	// Transaction: 4 UPDATEs
	accountMock.ExpectBegin()
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance - .* WHERE account_number = \$2`).
		WithArgs(11850.0, "ACC001").WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance \+ .* WHERE account_number = \$2`).
		WithArgs(11850.0, "BANK-RSD").WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance - .* WHERE account_number = \$2`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance \+ .* WHERE account_number = \$2`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectCommit()

	// Record exchange transaction
	dbMock.ExpectQuery(`INSERT INTO exchange_transactions`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	resp, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 11850, ClientId: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, "RSD", resp.FromCurrency)
	assert.Equal(t, "EUR", resp.ToCurrency)
	assert.Equal(t, 11850.0, resp.FromAmount)
	// 11850 / 118.50 * 0.995 ≈ 99.50
	assert.InDelta(t, 99.50, resp.ToAmount, 0.01)
	assert.Equal(t, int64(1), resp.TransactionId)
}

func TestConvertAmount_EURtoRSD_Happy(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)

	// From: EUR (currency_id=2, balance=100), To: RSD (currency_id=1)
	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 100.0, int64(2)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(1)))

	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("EUR"))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))

	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT buying_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"buying_rate"}).AddRow(115.50))

	// Bank intermediary accounts
	accountMock.ExpectQuery(`SELECT account_number FROM accounts WHERE owner_id = 0`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow("BANK-EUR"))
	accountMock.ExpectQuery(`SELECT account_number FROM accounts WHERE owner_id = 0`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow("BANK-RSD"))

	accountMock.ExpectBegin()
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance -`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance \+`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance -`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance \+`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectCommit()

	dbMock.ExpectQuery(`INSERT INTO exchange_transactions`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))

	resp, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 100, ClientId: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, "EUR", resp.FromCurrency)
	assert.Equal(t, "RSD", resp.ToCurrency)
	// 100 * 115.50 * 0.995 ≈ 11492.25
	assert.InDelta(t, 11492.25, resp.ToAmount, 0.01)
}

func TestConvertAmount_EURtoUSD_Happy(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)

	// From: EUR (currency_id=2, balance=200), To: USD (currency_id=3)
	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 200.0, int64(2)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(3)))

	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("EUR"))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("USD"))

	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT buying_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"buying_rate"}).AddRow(115.50))
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WithArgs("USD").
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"}).AddRow(110.00))

	accountMock.ExpectQuery(`SELECT account_number FROM accounts WHERE owner_id = 0`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow("BANK-EUR"))
	accountMock.ExpectQuery(`SELECT account_number FROM accounts WHERE owner_id = 0`).
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow("BANK-USD"))

	accountMock.ExpectBegin()
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance -`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance \+`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance -`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance \+`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectCommit()

	dbMock.ExpectQuery(`INSERT INTO exchange_transactions`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(3)))

	resp, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 100, ClientId: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, "EUR", resp.FromCurrency)
	assert.Equal(t, "USD", resp.ToCurrency)
	// rsd = 100 * 115.50 * 0.995 ≈ 11492.25 → 11492.25 / 110 * 0.995 ≈ 103.87
	assert.InDelta(t, 103.87, resp.ToAmount, 0.1)
}

func TestConvertAmount_BankSourceAccountNotFound(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)

	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 50000.0, int64(1)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(2)))

	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("EUR"))

	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"}).AddRow(118.50))

	// Bank source account not found
	accountMock.ExpectQuery(`SELECT account_number FROM accounts WHERE owner_id = 0`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}))

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── GetExchangeHistory ────────────────────────────────────────────────────────

func TestGetExchangeHistory_Empty(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	dbMock.ExpectQuery(`SELECT id, from_account, to_account, from_currency`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "from_account", "to_account", "from_currency", "to_currency",
			"from_amount", "to_amount", "rate", "commission", "timestamp", "status",
		}))

	resp, err := s.GetExchangeHistory(context.Background(), &pb.GetExchangeHistoryRequest{ClientId: 1})
	require.NoError(t, err)
	assert.Empty(t, resp.Transactions)
}

func TestGetExchangeHistory_Happy(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	dbMock.ExpectQuery(`SELECT id, from_account, to_account, from_currency`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "from_account", "to_account", "from_currency", "to_currency",
			"from_amount", "to_amount", "rate", "commission", "timestamp", "status",
		}).AddRow(
			int64(10), "ACC001", "ACC002", "RSD", "EUR",
			11850.0, 99.50, 118.50, 59.25, time.Now(), "COMPLETED",
		))

	resp, err := s.GetExchangeHistory(context.Background(), &pb.GetExchangeHistoryRequest{ClientId: 1})
	require.NoError(t, err)
	require.Len(t, resp.Transactions, 1)
	assert.Equal(t, int64(10), resp.Transactions[0].Id)
	assert.Equal(t, "RSD", resp.Transactions[0].FromCurrency)
	assert.Equal(t, "EUR", resp.Transactions[0].ToCurrency)
}

func TestGetExchangeHistory_DBError(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	dbMock.ExpectQuery(`SELECT id, from_account, to_account, from_currency`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	_, err := s.GetExchangeHistory(context.Background(), &pb.GetExchangeHistoryRequest{ClientId: 1})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestGetExchangeHistory_ScanError(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	cols := []string{"id", "from_account", "to_account", "from_currency", "to_currency",
		"from_amount", "to_amount", "rate", "commission", "timestamp", "status"}
	// Return "not-a-float" for from_amount → scan into float64 fails
	dbMock.ExpectQuery(`SELECT id, from_account, to_account, from_currency`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			int64(10), "ACC001", "ACC002", "RSD", "EUR",
			"not-a-float", 99.50, 118.50, 59.25, time.Now(), "COMPLETED",
		))

	_, err := s.GetExchangeHistory(context.Background(), &pb.GetExchangeHistoryRequest{ClientId: 1})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── ensureTodayRates (store path) ─────────────────────────────────────────────

func TestEnsureTodayRates_AlreadyExists(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	expectRatesAlreadyExist(dbMock)

	err := s.ensureTodayRates(context.Background())
	require.NoError(t, err)
}

func TestEnsureTodayRates_FetchAndStore(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	// No rates today → trigger store path
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM daily_exchange_rates WHERE date`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// fetchAndStoreRates: will try external API (fails in test environment → uses fallback)
	// Then inserts for each currency
	today := time.Now().Format("2006-01-02")
	for range []string{"EUR", "CHF", "USD", "GBP", "JPY", "CAD", "AUD"} {
		dbMock.ExpectExec(`INSERT INTO daily_exchange_rates`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), today).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	err := s.ensureTodayRates(context.Background())
	require.NoError(t, err)
}

func TestEnsureTodayRates_DBError(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM daily_exchange_rates WHERE date`).
		WillReturnError(sql.ErrConnDone)

	err := s.ensureTodayRates(context.Background())
	require.Error(t, err)
}

func TestFetchAndStoreRates_InsertError(t *testing.T) {
	s, dbMock, _ := newExchangeServer(t)

	// No rates today → trigger store path
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM daily_exchange_rates WHERE date`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// First INSERT fails
	today := time.Now().Format("2006-01-02")
	dbMock.ExpectExec(`INSERT INTO daily_exchange_rates`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), today).
		WillReturnError(sql.ErrConnDone)

	err := s.ensureTodayRates(context.Background())
	require.Error(t, err)
}

func TestConvertAmount_SourceAccountDBError(t *testing.T) {
	s, _, accountMock := newExchangeServer(t)

	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnError(sql.ErrConnDone)

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestConvertAmount_DestAccountDBError(t *testing.T) {
	s, _, accountMock := newExchangeServer(t)

	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 50000.0, int64(1)))

	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnError(sql.ErrConnDone)

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestConvertAmount_BankDestAccountNotFound(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)

	// From: RSD (currency_id=1), To: EUR (currency_id=2)
	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 50000.0, int64(1)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(2)))

	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("EUR"))

	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"}).AddRow(118.50))

	// Bank source account found
	accountMock.ExpectQuery(`SELECT account_number FROM accounts WHERE owner_id = 0`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow("BANK-RSD"))
	// Bank dest account NOT found
	accountMock.ExpectQuery(`SELECT account_number FROM accounts WHERE owner_id = 0`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}))

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// helper: sets up accounts + currencies + rates + bank accounts for RSD→EUR, ready for tx
func setupConvertRSDtoEUR(t *testing.T, s *ExchangeServer, dbMock, accountMock sqlmock.Sqlmock) {
	t.Helper()
	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 50000.0, int64(1)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(2)))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("EUR"))
	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"}).AddRow(118.50))
	accountMock.ExpectQuery(`SELECT account_number FROM accounts WHERE owner_id = 0`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow("BANK-RSD"))
	accountMock.ExpectQuery(`SELECT account_number FROM accounts WHERE owner_id = 0`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow("BANK-EUR"))
}

// helper: sets up accounts + currencies for RSD→EUR, stops before rates
func setupConvertAccountsAndCurrencies(t *testing.T, s *ExchangeServer, dbMock, accountMock sqlmock.Sqlmock) {
	t.Helper()
	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 50000.0, int64(1)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(2)))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("EUR"))
}

func TestConvertAmount_GetRateFromRSDError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)
	setupConvertAccountsAndCurrencies(t, s, dbMock, accountMock)

	expectRatesAlreadyExist(dbMock)
	// getRate("EUR", "selling_rate") returns ErrNoRows
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"})) // empty → ErrNoRows

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestConvertAmount_GetRateToRSDError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)

	// EUR → RSD: getRate("EUR", "buying_rate") fails
	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 200.0, int64(2)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(1)))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("EUR"))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))

	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT buying_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"buying_rate"})) // empty → ErrNoRows

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 100, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestConvertAmount_GetRateDefaultFromError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)

	// EUR → USD (default case), from EUR rate fails
	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 200.0, int64(2)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(3)))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("EUR"))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("USD"))

	expectRatesAlreadyExist(dbMock)
	// getRate("EUR", "buying_rate") fails
	dbMock.ExpectQuery(`SELECT buying_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"buying_rate"})) // empty → ErrNoRows

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 100, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestConvertAmount_GetRateDefaultToError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)

	// EUR → USD (default case), to USD rate fails
	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 200.0, int64(2)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(3)))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("EUR"))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("USD"))

	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT buying_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"buying_rate"}).AddRow(115.50))
	// getRate("USD", "selling_rate") fails
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WithArgs("USD").
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"})) // empty → ErrNoRows

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 100, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestConvertAmount_BeginTxError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)
	setupConvertRSDtoEUR(t, s, dbMock, accountMock)

	accountMock.ExpectBegin().WillReturnError(sql.ErrConnDone)

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestConvertAmount_InsertRecordError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)
	setupConvertRSDtoEUR(t, s, dbMock, accountMock)

	accountMock.ExpectBegin()
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance -`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance \+`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance -`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance \+`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectCommit()

	// INSERT fails — should be logged but NOT returned as error
	dbMock.ExpectQuery(`INSERT INTO exchange_transactions`).
		WillReturnError(sql.ErrConnDone)

	resp, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.NoError(t, err) // failure is only logged
	assert.Equal(t, "RSD", resp.FromCurrency)
	assert.Equal(t, "EUR", resp.ToCurrency)
	assert.Equal(t, int64(0), resp.TransactionId) // no ID returned
}

func TestConvertAmount_DebitSourceError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)
	setupConvertRSDtoEUR(t, s, dbMock, accountMock)

	accountMock.ExpectBegin()
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance -`).
		WillReturnError(sql.ErrConnDone)

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestConvertAmount_CreditBankSourceError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)
	setupConvertRSDtoEUR(t, s, dbMock, accountMock)

	accountMock.ExpectBegin()
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance -`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance \+`).
		WillReturnError(sql.ErrConnDone)

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestConvertAmount_DebitBankDestError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)
	setupConvertRSDtoEUR(t, s, dbMock, accountMock)

	accountMock.ExpectBegin()
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance -`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance \+`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance -`).
		WillReturnError(sql.ErrConnDone)

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestConvertAmount_CreditDestError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)
	setupConvertRSDtoEUR(t, s, dbMock, accountMock)

	accountMock.ExpectBegin()
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance -`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance \+`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance -`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance \+`).
		WillReturnError(sql.ErrConnDone)

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestConvertAmount_CommitError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)

	// From: RSD (currency_id=1), To: EUR (currency_id=2)
	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), 50000.0, int64(1)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(2)))

	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("EUR"))

	expectRatesAlreadyExist(dbMock)
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WithArgs("EUR").
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"}).AddRow(118.50))

	accountMock.ExpectQuery(`SELECT account_number FROM accounts WHERE owner_id = 0`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow("BANK-RSD"))
	accountMock.ExpectQuery(`SELECT account_number FROM accounts WHERE owner_id = 0`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow("BANK-EUR"))

	accountMock.ExpectBegin()
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance -`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance \+`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance -`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectExec(`UPDATE accounts SET balance = balance \+`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	accountMock.ExpectCommit().WillReturnError(sql.ErrConnDone)

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── fetchRatesFromAPI / fetchAndStoreRates (httptest) ─────────────────────────

// serveRates starts a test HTTP server returning the given JSON body, overrides
// rateAPIURL, and restores it on cleanup.
func serveRates(t *testing.T, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	orig := rateAPIURL
	rateAPIURL = srv.URL
	t.Cleanup(func() {
		srv.Close()
		rateAPIURL = orig
	})
}

func TestFetchRatesFromAPI_HappyPath(t *testing.T) {
	payload, _ := json.Marshal(erAPIResponse{
		Result: "success",
		Rates:  map[string]float64{"EUR": 0.0085, "CHF": 0.0086, "USD": 0.0093},
	})
	serveRates(t, string(payload))

	rates, err := fetchRatesFromAPI()
	require.NoError(t, err)
	assert.InDelta(t, 0.0085, rates["EUR"], 1e-6)
}

func TestFetchRatesFromAPI_NonSuccessResult(t *testing.T) {
	payload, _ := json.Marshal(erAPIResponse{Result: "error", Rates: nil})
	serveRates(t, string(payload))

	_, err := fetchRatesFromAPI()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-success")
}

func TestFetchRatesFromAPI_InvalidJSON(t *testing.T) {
	serveRates(t, "not-json{{{")

	_, err := fetchRatesFromAPI()
	require.Error(t, err)
}

func TestFetchRatesFromAPI_NetworkError(t *testing.T) {
	// "://invalid" is an unparseable URL — http.Client.Get returns immediately with a parse error.
	orig := rateAPIURL
	rateAPIURL = "://invalid-url"
	defer func() { rateAPIURL = orig }()

	_, err := fetchRatesFromAPI()
	require.Error(t, err)
}

func TestFetchAndStoreRates_APIErrorFallback(t *testing.T) {
	// Force the API to fail so the fallback branch is deterministically covered.
	orig := rateAPIURL
	rateAPIURL = "://invalid-url"
	defer func() { rateAPIURL = orig }()

	s, dbMock, _ := newExchangeServer(t)
	today := time.Now().Format("2006-01-02")
	for i := 0; i < 7; i++ {
		dbMock.ExpectExec(`INSERT INTO daily_exchange_rates`).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	err := s.fetchAndStoreRates(context.Background(), today)
	require.NoError(t, err)
}

func TestFetchAndStoreRates_AllRatesFromAPI(t *testing.T) {
	// Serve all 7 currencies so the fallback branch is NOT taken
	payload, _ := json.Marshal(erAPIResponse{
		Result: "success",
		Rates: map[string]float64{
			"EUR": 0.008547, "CHF": 0.008621, "USD": 0.009259,
			"GBP": 0.007353, "JPY": 1.380000, "CAD": 0.012500, "AUD": 0.014286,
		},
	})
	serveRates(t, string(payload))

	s, dbMock, _ := newExchangeServer(t)
	today := time.Now().Format("2006-01-02")
	// Expect 7 successful inserts
	for i := 0; i < 7; i++ {
		dbMock.ExpectExec(`INSERT INTO daily_exchange_rates`).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	err := s.fetchAndStoreRates(context.Background(), today)
	require.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestFetchAndStoreRates_PartialAPIResponse_FallbackUsed(t *testing.T) {
	// API returns only EUR — CHF, USD, GBP, JPY, CAD, AUD will use fallback
	payload, _ := json.Marshal(erAPIResponse{
		Result: "success",
		Rates:  map[string]float64{"EUR": 0.008547},
	})
	serveRates(t, string(payload))

	s, dbMock, _ := newExchangeServer(t)
	today := time.Now().Format("2006-01-02")
	for i := 0; i < 7; i++ {
		dbMock.ExpectExec(`INSERT INTO daily_exchange_rates`).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	err := s.fetchAndStoreRates(context.Background(), today)
	require.NoError(t, err)
}
