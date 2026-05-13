package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/exchange"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func newExchangeServerFull(t *testing.T) (*ExchangeServer, sqlmock.Sqlmock, sqlmock.Sqlmock, *miniredis.Miniredis) {
	t.Helper()
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	accountDB, accountMock, err := sqlmock.New()
	require.NoError(t, err)
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = db.Close(); _ = accountDB.Close(); _ = rc.Close() })
	return &ExchangeServer{DB: db, AccountDB: accountDB, Redis: rc}, dbMock, accountMock, mr
}

// seedRateCache stores EUR and USD rates in miniredis for cache-hit tests.
func seedRateCache(t *testing.T, mr *miniredis.Miniredis) {
	t.Helper()
	rates := map[string]cachedRate{
		"EUR": {BuyingRate: 115.50, SellingRate: 118.50, MiddleRate: 117.00},
		"USD": {BuyingRate: 107.00, SellingRate: 110.00, MiddleRate: 108.50},
	}
	data, err := json.Marshal(rates)
	require.NoError(t, err)
	require.NoError(t, mr.Set(ratesCacheKey(), string(data)))
}

// ── ConvertAmount: source currency code error (line 267-269) ─────────────────

func TestConvertAmount_SourceCurrencyCodeError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)

	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), float64(50000), int64(1)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(2)))

	// Source currency code lookup fails
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── ConvertAmount: destination currency code error (line 272-274) ────────────

func TestConvertAmount_DestCurrencyCodeError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)

	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), float64(50000), int64(1)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC002").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(2)))

	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))
	// Destination currency code lookup fails
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(2)).
		WillReturnError(sql.ErrConnDone)

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── ConvertAmount: ensureTodayRates error is logged but non-fatal (line 281-283)

func TestConvertAmount_EnsureRatesError(t *testing.T) {
	s, dbMock, accountMock := newExchangeServer(t)

	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), float64(100), int64(1)))
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

	// ensureTodayRates COUNT fails → non-fatal log (line 281-283)
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM daily_exchange_rates WHERE date`).
		WillReturnError(sql.ErrConnDone)

	// getRate("EUR", "selling_rate") returns empty → NotFound
	dbMock.ExpectQuery(`SELECT selling_rate FROM daily_exchange_rates`).
		WillReturnRows(sqlmock.NewRows([]string{"selling_rate"}))

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	// ensureTodayRates error was logged; rate lookup failed → NotFound
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// ── ConvertAmount: Redis cache hit — selling_rate + buying_rate branches ──────
// (lines 293-301: ratesCache != nil, case "selling_rate", case "buying_rate")

func TestConvertAmount_CacheHit_InsufficientFunds(t *testing.T) {
	s, dbMock, accountMock, mr := newExchangeServerFull(t)
	seedRateCache(t, mr)

	// from=RSD (currency_id=1), to=EUR (currency_id=2); balance=100
	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC001").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), float64(100), int64(1)))
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

	// ensureTodayRates: count > 0 (no fetch needed)
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM daily_exchange_rates WHERE date`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

	// getRate("EUR", "selling_rate") hits cache → no DB query needed

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC001", ToAccount: "ACC002", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err)) // insufficient funds
}

// ── PreviewConversion: Redis cache hit — selling_rate branch (lines 450-461) ─

func TestPreviewConversion_CacheHit_RSDtoEUR(t *testing.T) {
	s, dbMock, _, mr := newExchangeServerFull(t)
	seedRateCache(t, mr)

	// ensureTodayRates: rates already exist
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM daily_exchange_rates WHERE date`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

	// getRate("EUR", "selling_rate") hits cache — no DB query for rate

	resp, err := s.PreviewConversion(context.Background(), &pb.PreviewConversionRequest{
		FromCurrency: "RSD", ToCurrency: "EUR", Amount: 11850,
	})
	require.NoError(t, err)
	assert.Equal(t, "RSD", resp.FromCurrency)
	assert.Equal(t, "EUR", resp.ToCurrency)
	assert.InDelta(t, 99.50, resp.ToAmount, 0.1)
}

// ── ConvertAmount: Redis cache hit — buying_rate branch (lines 296-297) ──────
// from=EUR, to=RSD → getRate("EUR", "buying_rate") hits cache.

func TestConvertAmount_CacheHit_EURtoRSD(t *testing.T) {
	s, dbMock, accountMock, mr := newExchangeServerFull(t)
	seedRateCache(t, mr)

	// from=EUR (currency_id=2), to=RSD (currency_id=1); balance=100 < amount=1000
	accountMock.ExpectQuery(`SELECT owner_id, available_balance, currency_id FROM accounts`).
		WithArgs("ACC-EUR").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "available_balance", "currency_id"}).
			AddRow(int64(1), float64(100), int64(2)))
	accountMock.ExpectQuery(`SELECT owner_id, currency_id FROM accounts`).
		WithArgs("ACC-RSD").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "currency_id"}).
			AddRow(int64(1), int64(1)))

	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("EUR"))
	dbMock.ExpectQuery(`SELECT code FROM currencies WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("RSD"))

	// ensureTodayRates: rates exist in DB
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM daily_exchange_rates WHERE date`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

	// getRate("EUR", "buying_rate") hits cache → lines 296-297 covered
	// balance=100 < amount=1000 → FailedPrecondition (insufficient funds)

	_, err := s.ConvertAmount(context.Background(), &pb.ConvertAmountRequest{
		FromAccount: "ACC-EUR", ToAccount: "ACC-RSD", Amount: 1000, ClientId: 1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// ── PreviewConversion: Redis cache hit — buying_rate branch ──────────────────

func TestPreviewConversion_CacheHit_EURtoRSD(t *testing.T) {
	s, dbMock, _, mr := newExchangeServerFull(t)
	seedRateCache(t, mr)

	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM daily_exchange_rates WHERE date`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

	// getRate("EUR", "buying_rate") hits cache

	resp, err := s.PreviewConversion(context.Background(), &pb.PreviewConversionRequest{
		FromCurrency: "EUR", ToCurrency: "RSD", Amount: 100,
	})
	require.NoError(t, err)
	assert.Equal(t, "RSD", resp.ToCurrency)
	assert.InDelta(t, 11492.25, resp.ToAmount, 1.0) // 100 * 115.50 * 0.995
}
