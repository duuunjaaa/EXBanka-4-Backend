package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	pb_ex "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/exchange"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/portfolio"
	pb_sec "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/securities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// mockSecClient implements SecurityPriceFetcher for tests.
type mockSecClient struct {
	price     float64
	ticker    string
	assetType string
}

func (m *mockSecClient) GetListingById(_ context.Context, _ *pb_sec.GetListingByIdRequest, _ ...grpc.CallOption) (*pb_sec.GetListingByIdResponse, error) {
	return &pb_sec.GetListingByIdResponse{
		Summary: &pb_sec.ListingSummary{
			Price:  m.price,
			Ticker: m.ticker,
			Type:   m.assetType,
		},
	}, nil
}

func newServer(t *testing.T) (*PortfolioServer, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return &PortfolioServer{DB: db}, mock
}

func newServerWithSec(t *testing.T, sec SecurityPriceFetcher) (*PortfolioServer, sqlmock.Sqlmock) {
	t.Helper()
	srv, mock := newServer(t)
	srv.SecuritiesClient = sec
	return srv, mock
}

// newServerWithSecDB returns a server with both the main DB mock and a SecuritiesDB mock.
// Use this for SELL+STOCK tests where convertToRSD queries SecuritiesDB for the listing currency.
func newServerWithSecDB(t *testing.T) (*PortfolioServer, sqlmock.Sqlmock, sqlmock.Sqlmock) {
	t.Helper()
	srv, mainMock := newServer(t)
	secDB, secMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = secDB.Close() })
	srv.SecuritiesDB = secDB
	return srv, mainMock, secMock
}

// ── UpdateHolding ─────────────────────────────────────────────────────────────

func TestUpdateHolding_Buy_NewEntry(t *testing.T) {
	srv, mock := newServer(t)

	mock.ExpectExec(`INSERT INTO portfolio_entry`).
		WithArgs(int64(1), "CLIENT", int64(10), int32(5), float64(100.0), int64(42)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err := srv.UpdateHolding(context.Background(), &pb.UpdateHoldingRequest{
		UserId:    1,
		UserType:  "CLIENT",
		ListingId: 10,
		Quantity:  5,
		Price:     100.0,
		Direction: "BUY",
		AccountId: 42,
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateHolding_Buy_ExistingEntry_WeightedAvg(t *testing.T) {
	srv, mock := newServer(t)

	// sharedUserID returns 0 for EMPLOYEE so all actuaries share one portfolio.
	mock.ExpectExec(`INSERT INTO portfolio_entry`).
		WithArgs(int64(0), "EMPLOYEE", int64(20), int32(3), float64(200.0), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	_, err := srv.UpdateHolding(context.Background(), &pb.UpdateHoldingRequest{
		UserId:    2,
		UserType:  "EMPLOYEE",
		ListingId: 20,
		Quantity:  3,
		Price:     200.0,
		Direction: "BUY",
		AccountId: 99,
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateHolding_Sell_WithTax(t *testing.T) {
	// buy at 100, sell at 150 → profit=50*2=100, tax=15
	srv, mock, secMock := newServerWithSecDB(t)

	mock.ExpectQuery(`SELECT buy_price FROM portfolio_entry`).
		WithArgs(int64(1), "CLIENT", int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"buy_price"}).AddRow(100.0))
	mock.ExpectExec(`UPDATE portfolio_entry`).
		WithArgs(int32(2), int64(1), "CLIENT", int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM portfolio_entry`).
		WithArgs(int64(1), "CLIENT", int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	// convertToRSD looks up the listing's currency from SecuritiesDB
	secMock.ExpectQuery(`SELECT e.currency`).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"currency"}).AddRow("RSD"))
	mock.ExpectExec(`INSERT INTO tax_record`).
		WithArgs(int64(1), "CLIENT", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err := srv.UpdateHolding(context.Background(), &pb.UpdateHoldingRequest{
		UserId:    1,
		UserType:  "CLIENT",
		ListingId: 10,
		Quantity:  2,
		Price:     150.0,
		Direction: "SELL",
		AssetType: "STOCK",
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.NoError(t, secMock.ExpectationsWereMet())
}

func TestUpdateHolding_Sell_Loss_NoTax(t *testing.T) {
	// buy at 200, sell at 150 → loss, no tax record
	srv, mock, secMock := newServerWithSecDB(t)

	mock.ExpectQuery(`SELECT buy_price FROM portfolio_entry`).
		WithArgs(int64(1), "CLIENT", int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"buy_price"}).AddRow(200.0))
	mock.ExpectExec(`UPDATE portfolio_entry`).
		WithArgs(int32(5), int64(1), "CLIENT", int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM portfolio_entry`).
		WithArgs(int64(1), "CLIENT", int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// convertToRSD is still called (profit is negative, but conversion happens before the tax>0 check)
	secMock.ExpectQuery(`SELECT e.currency`).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"currency"}).AddRow("RSD"))

	_, err := srv.UpdateHolding(context.Background(), &pb.UpdateHoldingRequest{
		UserId:    1,
		UserType:  "CLIENT",
		ListingId: 10,
		Quantity:  5,
		Price:     150.0,
		Direction: "SELL",
		AssetType: "STOCK",
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.NoError(t, secMock.ExpectationsWereMet())
}

func TestUpdateHolding_Sell_Forex_NoTax(t *testing.T) {
	// profitable sell but FOREX_PAIR — no tax record should be inserted
	srv, mock := newServer(t)

	mock.ExpectQuery(`SELECT buy_price FROM portfolio_entry`).
		WithArgs(int64(1), "CLIENT", int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"buy_price"}).AddRow(100.0))
	mock.ExpectExec(`UPDATE portfolio_entry`).
		WithArgs(int32(2), int64(1), "CLIENT", int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM portfolio_entry`).
		WithArgs(int64(1), "CLIENT", int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, err := srv.UpdateHolding(context.Background(), &pb.UpdateHoldingRequest{
		UserId:    1,
		UserType:  "CLIENT",
		ListingId: 10,
		Quantity:  2,
		Price:     150.0,
		Direction: "SELL",
		AssetType: "FOREX_PAIR",
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateHolding_InvalidDirection(t *testing.T) {
	srv, _ := newServer(t)

	_, err := srv.UpdateHolding(context.Background(), &pb.UpdateHoldingRequest{
		UserId:    1,
		ListingId: 10,
		Quantity:  1,
		Direction: "HOLD",
	})
	require.Error(t, err)
}

func TestUpdateHolding_ZeroQuantity(t *testing.T) {
	srv, _ := newServer(t)

	_, err := srv.UpdateHolding(context.Background(), &pb.UpdateHoldingRequest{
		UserId:    1,
		ListingId: 10,
		Quantity:  0,
		Direction: "BUY",
	})
	require.Error(t, err)
}

// ── GetPortfolio ──────────────────────────────────────────────────────────────

func TestGetPortfolio_WithPriceEnrichment(t *testing.T) {
	sec := &mockSecClient{price: 200.0, ticker: "AAPL", assetType: "STOCK"}
	srv, mock := newServerWithSec(t, sec)

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "user_type", "listing_id", "amount",
		"buy_price", "last_modified", "is_public", "public_amount", "account_id",
	}).AddRow(1, int64(1), "CLIENT", int64(10), int32(5), float64(150.0), time.Now(), false, 0, int64(42))

	mock.ExpectQuery(`SELECT`).WithArgs(int64(1), "").WillReturnRows(rows)

	resp, err := srv.GetPortfolio(context.Background(), &pb.GetPortfolioRequest{UserId: 1})
	require.NoError(t, err)
	require.Len(t, resp.Entries, 1)

	e := resp.Entries[0]
	assert.Equal(t, "AAPL", e.Ticker)
	assert.Equal(t, "STOCK", e.AssetType)
	assert.InDelta(t, 200.0, e.Price, 0.001)
	// profit = (200 - 150) * 5 = 250
	assert.InDelta(t, 250.0, e.Profit, 0.001)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── GetProfit ─────────────────────────────────────────────────────────────────

func TestGetProfit_HappyPath(t *testing.T) {
	sec := &mockSecClient{price: 200.0, ticker: "AAPL", assetType: "STOCK"}
	srv, mock := newServerWithSec(t, sec)

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "user_type", "listing_id", "amount",
		"buy_price", "last_modified", "is_public", "public_amount", "account_id",
	}).
		AddRow(1, int64(1), "CLIENT", int64(10), int32(5), float64(150.0), now, false, 0, int64(42)).
		AddRow(2, int64(1), "CLIENT", int64(20), int32(2), float64(300.0), now, false, 0, int64(42))

	mock.ExpectQuery(`SELECT`).WithArgs(int64(1), "").WillReturnRows(rows)

	callCount := 0
	srv.SecuritiesClient = &callCountMockSecClient{
		prices: []float64{200.0, 280.0},
		call:   &callCount,
	}

	resp, err := srv.GetProfit(context.Background(), &pb.GetProfitRequest{UserId: 1})
	require.NoError(t, err)
	// (200-150)*5 + (280-300)*2 = 250 + (-40) = 210
	assert.InDelta(t, 210.0, resp.TotalProfit, 0.001)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProfit_EmptyPortfolio(t *testing.T) {
	sec := &mockSecClient{}
	srv, mock := newServerWithSec(t, sec)

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "user_type", "listing_id", "amount",
		"buy_price", "last_modified", "is_public", "public_amount", "account_id",
	})
	mock.ExpectQuery(`SELECT`).WithArgs(int64(99), "").WillReturnRows(rows)

	resp, err := srv.GetProfit(context.Background(), &pb.GetProfitRequest{UserId: 99})
	require.NoError(t, err)
	assert.InDelta(t, 0.0, resp.TotalProfit, 0.001)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProfit_NegativeProfit(t *testing.T) {
	sec := &mockSecClient{price: 80.0, ticker: "XYZ", assetType: "STOCK"}
	srv, mock := newServerWithSec(t, sec)

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "user_type", "listing_id", "amount",
		"buy_price", "last_modified", "is_public", "public_amount", "account_id",
	}).AddRow(1, int64(1), "CLIENT", int64(5), int32(10), float64(100.0), time.Now(), false, 0, int64(42))

	mock.ExpectQuery(`SELECT`).WithArgs(int64(1), "").WillReturnRows(rows)

	resp, err := srv.GetProfit(context.Background(), &pb.GetProfitRequest{UserId: 1})
	require.NoError(t, err)
	// (80 - 100) * 10 = -200
	assert.InDelta(t, -200.0, resp.TotalProfit, 0.001)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── GetMyTax ──────────────────────────────────────────────────────────────────

func TestGetMyTax_HappyPath(t *testing.T) {
	srv, mock := newServer(t)

	mock.ExpectQuery(`SELECT`).
		WithArgs(int64(1), "CLIENT", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"paid", "unpaid"}).AddRow(4500.0, 2250.0))

	resp, err := srv.GetMyTax(context.Background(), &pb.GetMyTaxRequest{UserId: 1, UserType: "CLIENT"})
	require.NoError(t, err)
	assert.InDelta(t, 4500.0, resp.PaidThisYear, 0.001)
	assert.InDelta(t, 2250.0, resp.UnpaidThisMonth, 0.001)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── GetTaxList ────────────────────────────────────────────────────────────────

func TestGetTaxList_ReturnsDebts(t *testing.T) {
	srv, mock := newServer(t)

	mock.ExpectQuery(`SELECT user_id, user_type`).
		WithArgs("").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "user_type", "debt_rsd"}).
			AddRow(int64(1), "CLIENT", 1500.0).
			AddRow(int64(2), "EMPLOYEE", 750.0))

	resp, err := srv.GetTaxList(context.Background(), &pb.GetTaxListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Entries, 2)
	assert.Equal(t, int64(1), resp.Entries[0].UserId)
	assert.InDelta(t, 1500.0, resp.Entries[0].DebtRsd, 0.001)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// callCountMockSecClient returns different prices per sequential call.
type callCountMockSecClient struct {
	prices []float64
	call   *int
}

func (m *callCountMockSecClient) GetListingById(_ context.Context, _ *pb_sec.GetListingByIdRequest, _ ...grpc.CallOption) (*pb_sec.GetListingByIdResponse, error) {
	idx := *m.call
	*m.call++
	price := 0.0
	if idx < len(m.prices) {
		price = m.prices[idx]
	}
	return &pb_sec.GetListingByIdResponse{
		Summary: &pb_sec.ListingSummary{Price: price},
	}, nil
}

// ── SetPublicMode ─────────────────────────────────────────────────────────────

func TestSetPublicMode_MissingTicker(t *testing.T) {
	srv, _ := newServer(t)
	_, err := srv.SetPublicMode(context.Background(), &pb.SetPublicModeRequest{
		UserId: 10, UserType: "CLIENT", Ticker: "", IsPublic: true,
	})
	require.Error(t, err)
	assert.Equal(t, "ticker is required", statusMessage(err))
}

func TestSetPublicMode_TickerNotFound(t *testing.T) {
	srv, _, mSec := newServerWithSecDB(t)
	mSec.ExpectQuery("SELECT id FROM listing WHERE ticker").
		WillReturnError(errNoRows())
	_, err := srv.SetPublicMode(context.Background(), &pb.SetPublicModeRequest{
		UserId: 10, UserType: "CLIENT", Ticker: "UNKNOWN", IsPublic: true,
	})
	require.Error(t, err)
	assert.Contains(t, statusMessage(err), "listing not found")
	assert.NoError(t, mSec.ExpectationsWereMet())
}

func TestSetPublicMode_PositionNotFound(t *testing.T) {
	srv, mDB, mSec := newServerWithSecDB(t)
	mSec.ExpectQuery("SELECT id FROM listing WHERE ticker").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))
	mDB.ExpectExec("UPDATE portfolio_entry").
		WillReturnResult(sqlmock.NewResult(0, 0))
	_, err := srv.SetPublicMode(context.Background(), &pb.SetPublicModeRequest{
		UserId: 10, UserType: "CLIENT", Ticker: "AAPL", IsPublic: true,
	})
	require.Error(t, err)
	assert.Contains(t, statusMessage(err), "position not found")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSetPublicMode_Enable(t *testing.T) {
	srv, mDB, mSec := newServerWithSecDB(t)
	mSec.ExpectQuery("SELECT id FROM listing WHERE ticker").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))
	mDB.ExpectExec("UPDATE portfolio_entry").
		WillReturnResult(sqlmock.NewResult(0, 1))

	resp, err := srv.SetPublicMode(context.Background(), &pb.SetPublicModeRequest{
		UserId: 10, UserType: "CLIENT", Ticker: "AAPL", IsPublic: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "AAPL", resp.Ticker)
	assert.True(t, resp.IsPublic)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSetPublicMode_Disable(t *testing.T) {
	srv, mDB, mSec := newServerWithSecDB(t)
	mSec.ExpectQuery("SELECT id FROM listing WHERE ticker").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))
	mDB.ExpectExec("UPDATE portfolio_entry").
		WillReturnResult(sqlmock.NewResult(0, 1))

	resp, err := srv.SetPublicMode(context.Background(), &pb.SetPublicModeRequest{
		UserId: 10, UserType: "CLIENT", Ticker: "AAPL", IsPublic: false,
	})
	require.NoError(t, err)
	assert.Equal(t, "AAPL", resp.Ticker)
	assert.False(t, resp.IsPublic)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

func TestSetPublicMode_EmployeeUsesSharedUserID(t *testing.T) {
	// EMPLOYEE portfolios are stored under user_id=0 regardless of actual employee ID.
	// SetPublicMode must map any EMPLOYEE to user_id=0 when building the UPDATE.
	srv, mDB, mSec := newServerWithSecDB(t)
	mSec.ExpectQuery("SELECT id FROM listing WHERE ticker").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))
	// WithArgs verifies that user_id=0 is used in the UPDATE, not the caller's employee ID (5).
	mDB.ExpectExec("UPDATE portfolio_entry").
		WithArgs(true, int64(0), "EMPLOYEE", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	resp, err := srv.SetPublicMode(context.Background(), &pb.SetPublicModeRequest{
		UserId: 5, UserType: "EMPLOYEE", Ticker: "AAPL", IsPublic: true,
	})
	require.NoError(t, err)
	assert.True(t, resp.IsPublic)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// statusMessage extracts the gRPC status message from an error.
func statusMessage(err error) string {
	return status.Convert(err).Message()
}

// errNoRows returns sql.ErrNoRows for use in mock expectations.
func errNoRows() error {
	return sql.ErrNoRows
}

// ── mockExchangeClient ────────────────────────────────────────────────────────

type mockExchangeClient struct {
	getRatesFn func(context.Context, *pb_ex.GetExchangeRatesRequest, ...grpc.CallOption) (*pb_ex.GetExchangeRatesResponse, error)
}

func (m *mockExchangeClient) GetExchangeRates(ctx context.Context, in *pb_ex.GetExchangeRatesRequest, opts ...grpc.CallOption) (*pb_ex.GetExchangeRatesResponse, error) {
	if m.getRatesFn != nil {
		return m.getRatesFn(ctx, in, opts...)
	}
	return &pb_ex.GetExchangeRatesResponse{}, nil
}
func (m *mockExchangeClient) ConvertAmount(ctx context.Context, in *pb_ex.ConvertAmountRequest, opts ...grpc.CallOption) (*pb_ex.ConvertAmountResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockExchangeClient) GetExchangeHistory(ctx context.Context, in *pb_ex.GetExchangeHistoryRequest, opts ...grpc.CallOption) (*pb_ex.GetExchangeHistoryResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockExchangeClient) PreviewConversion(ctx context.Context, in *pb_ex.PreviewConversionRequest, opts ...grpc.CallOption) (*pb_ex.PreviewConversionResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// newServerWithExchange returns a server with SecuritiesDB mock + ExchangeClient mock.
func newServerWithExchange(t *testing.T, exClient pb_ex.ExchangeServiceClient) (*PortfolioServer, sqlmock.Sqlmock, sqlmock.Sqlmock) {
	t.Helper()
	srv, mainMock, secMock := newServerWithSecDB(t)
	srv.ExchangeClient = exClient
	return srv, mainMock, secMock
}

// ── SetPublicAmount ───────────────────────────────────────────────────────────

func TestSetPublicAmount_Unimplemented(t *testing.T) {
	srv, _ := newServer(t)
	_, err := srv.SetPublicAmount(context.Background(), &pb.SetPublicAmountRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Unimplemented, status.Code(err))
}

// ── CollectTax ────────────────────────────────────────────────────────────────

func TestCollectTax_DBError(t *testing.T) {
	srv, mock := newServer(t)
	mock.ExpectQuery("SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at").
		WillReturnError(fmt.Errorf("db error"))
	_, err := srv.CollectTax(context.Background(), &pb.CollectTaxRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestCollectTax_Happy(t *testing.T) {
	srv, mock := newServer(t)
	mock.ExpectQuery("SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}))
	_, err := srv.CollectTax(context.Background(), &pb.CollectTaxRequest{})
	require.NoError(t, err)
}

// ── CollectTaxForUser ─────────────────────────────────────────────────────────

func TestCollectTaxForUser_DBError(t *testing.T) {
	srv, mock := newServer(t)
	mock.ExpectQuery("SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at").
		WillReturnError(fmt.Errorf("db error"))
	_, err := srv.CollectTaxForUser(context.Background(), &pb.CollectTaxForUserRequest{UserId: 1, UserType: "CLIENT"})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestCollectTaxForUser_Happy(t *testing.T) {
	srv, mock := newServer(t)
	mock.ExpectQuery("SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}))
	_, err := srv.CollectTaxForUser(context.Background(), &pb.CollectTaxForUserRequest{UserId: 1, UserType: "CLIENT"})
	require.NoError(t, err)
}

func TestCollectTaxForUser_UserTypeFromCtx(t *testing.T) {
	// UserType not set in request → read from gRPC metadata
	srv, mock := newServer(t)
	mock.ExpectQuery("SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}))
	md := metadata.Pairs("user-type", "EMPLOYEE")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err := srv.CollectTaxForUser(ctx, &pb.CollectTaxForUserRequest{UserId: 5, UserType: ""})
	require.NoError(t, err)
}

// ── convertToRSD via UpdateHolding ────────────────────────────────────────────

func TestUpdateHolding_Sell_ForeignCurrency_RateFound(t *testing.T) {
	exClient := &mockExchangeClient{
		getRatesFn: func(_ context.Context, _ *pb_ex.GetExchangeRatesRequest, _ ...grpc.CallOption) (*pb_ex.GetExchangeRatesResponse, error) {
			return &pb_ex.GetExchangeRatesResponse{
				Rates: []*pb_ex.ExchangeRate{
					{CurrencyCode: "USD", MiddleRate: 108.5},
				},
			}, nil
		},
	}
	srv, mock, secMock := newServerWithExchange(t, exClient)

	mock.ExpectQuery(`SELECT buy_price FROM portfolio_entry`).
		WillReturnRows(sqlmock.NewRows([]string{"buy_price"}).AddRow(100.0))
	mock.ExpectExec(`UPDATE portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	// convertToRSD: currency = USD (not RSD) → calls ExchangeClient
	secMock.ExpectQuery(`SELECT e.currency`).
		WillReturnRows(sqlmock.NewRows([]string{"currency"}).AddRow("USD"))
	// taxOwed = (150-100)*2 * 108.5 * 0.15 = 100*108.5*0.15 = 1627.5 → insert tax
	mock.ExpectExec(`INSERT INTO tax_record`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err := srv.UpdateHolding(context.Background(), &pb.UpdateHoldingRequest{
		UserId: 1, UserType: "CLIENT", ListingId: 10,
		Quantity: 2, Price: 150.0, Direction: "SELL", AssetType: "STOCK",
	})
	require.NoError(t, err)
}

func TestUpdateHolding_Sell_ForeignCurrency_RateNotFound(t *testing.T) {
	// ExchangeClient returns no rates for the currency → falls back to raw profit
	exClient := &mockExchangeClient{
		getRatesFn: func(_ context.Context, _ *pb_ex.GetExchangeRatesRequest, _ ...grpc.CallOption) (*pb_ex.GetExchangeRatesResponse, error) {
			return &pb_ex.GetExchangeRatesResponse{Rates: []*pb_ex.ExchangeRate{}}, nil
		},
	}
	srv, mock, secMock := newServerWithExchange(t, exClient)

	mock.ExpectQuery(`SELECT buy_price FROM portfolio_entry`).
		WillReturnRows(sqlmock.NewRows([]string{"buy_price"}).AddRow(100.0))
	mock.ExpectExec(`UPDATE portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	secMock.ExpectQuery(`SELECT e.currency`).
		WillReturnRows(sqlmock.NewRows([]string{"currency"}).AddRow("EUR"))
	// convertToRSD returns error → falls back to raw profit; tax still owed
	mock.ExpectExec(`INSERT INTO tax_record`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err := srv.UpdateHolding(context.Background(), &pb.UpdateHoldingRequest{
		UserId: 1, UserType: "CLIENT", ListingId: 10,
		Quantity: 2, Price: 150.0, Direction: "SELL", AssetType: "STOCK",
	})
	require.NoError(t, err)
}

func TestUpdateHolding_Sell_ExchangeClientError(t *testing.T) {
	// GetExchangeRates fails → convertToRSD returns error → falls back to raw profit
	exClient := &mockExchangeClient{
		getRatesFn: func(_ context.Context, _ *pb_ex.GetExchangeRatesRequest, _ ...grpc.CallOption) (*pb_ex.GetExchangeRatesResponse, error) {
			return nil, fmt.Errorf("exchange service down")
		},
	}
	srv, mock, secMock := newServerWithExchange(t, exClient)

	mock.ExpectQuery(`SELECT buy_price FROM portfolio_entry`).
		WillReturnRows(sqlmock.NewRows([]string{"buy_price"}).AddRow(100.0))
	mock.ExpectExec(`UPDATE portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	secMock.ExpectQuery(`SELECT e.currency`).
		WillReturnRows(sqlmock.NewRows([]string{"currency"}).AddRow("USD"))
	// fall back to raw profit → tax inserted
	mock.ExpectExec(`INSERT INTO tax_record`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err := srv.UpdateHolding(context.Background(), &pb.UpdateHoldingRequest{
		UserId: 1, UserType: "CLIENT", ListingId: 10,
		Quantity: 2, Price: 150.0, Direction: "SELL", AssetType: "STOCK",
	})
	require.NoError(t, err)
}

func TestUpdateHolding_Sell_CurrencyLookupError(t *testing.T) {
	// SecuritiesDB fails on currency lookup → convertToRSD returns error → falls back to raw profit
	srv, mock, secMock := newServerWithSecDB(t)
	mock.ExpectQuery(`SELECT buy_price FROM portfolio_entry`).
		WillReturnRows(sqlmock.NewRows([]string{"buy_price"}).AddRow(100.0))
	mock.ExpectExec(`UPDATE portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	secMock.ExpectQuery(`SELECT e.currency`).
		WillReturnError(fmt.Errorf("db error"))
	// fall back to raw profit → tax record inserted
	mock.ExpectExec(`INSERT INTO tax_record`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	_, err := srv.UpdateHolding(context.Background(), &pb.UpdateHoldingRequest{
		UserId: 1, UserType: "CLIENT", ListingId: 10,
		Quantity: 2, Price: 150.0, Direction: "SELL", AssetType: "STOCK",
	})
	require.NoError(t, err)
}

func TestUpdateHolding_DBError(t *testing.T) {
	srv, mock := newServer(t)
	mock.ExpectExec(`INSERT INTO portfolio_entry`).
		WillReturnError(fmt.Errorf("db error"))
	_, err := srv.UpdateHolding(context.Background(), &pb.UpdateHoldingRequest{
		UserId: 1, UserType: "CLIENT", ListingId: 10, Quantity: 1, Price: 100, Direction: "BUY",
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── GetPortfolio additional paths ─────────────────────────────────────────────

func TestGetPortfolio_DBError(t *testing.T) {
	srv, mock := newServer(t)
	mock.ExpectQuery(`SELECT`).WillReturnError(fmt.Errorf("db error"))
	_, err := srv.GetPortfolio(context.Background(), &pb.GetPortfolioRequest{UserId: 1, UserType: "CLIENT"})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestGetPortfolio_SecClientError(t *testing.T) {
	// GetListingById returns error → entry still included but without ticker/price enrichment
	srv, mock := newServer(t)
	srv.SecuritiesClient = &failingSecClient{}

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "user_type", "listing_id", "amount",
		"buy_price", "last_modified", "is_public", "public_amount", "account_id",
	}).AddRow(1, int64(1), "CLIENT", int64(10), int32(5), float64(100.0), time.Now(), false, 0, int64(42))
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	resp, err := srv.GetPortfolio(context.Background(), &pb.GetPortfolioRequest{UserId: 1, UserType: "CLIENT"})
	require.NoError(t, err)
	require.Len(t, resp.Entries, 1)
	assert.Equal(t, "", resp.Entries[0].Ticker) // not enriched due to error
}

// failingSecClient returns an error on GetListingById.
type failingSecClient struct{}

func (f *failingSecClient) GetListingById(_ context.Context, _ *pb_sec.GetListingByIdRequest, _ ...grpc.CallOption) (*pb_sec.GetListingByIdResponse, error) {
	return nil, fmt.Errorf("securities service down")
}

// ── GetProfit additional paths ────────────────────────────────────────────────

func TestGetProfit_DBError(t *testing.T) {
	srv, mock := newServerWithSec(t, &mockSecClient{})
	mock.ExpectQuery(`SELECT`).WillReturnError(fmt.Errorf("db error"))
	_, err := srv.GetProfit(context.Background(), &pb.GetProfitRequest{UserId: 1, UserType: "CLIENT"})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── GetMyTax additional paths ─────────────────────────────────────────────────

func TestGetMyTax_DBError(t *testing.T) {
	srv, mock := newServer(t)
	mock.ExpectQuery(`SELECT`).WillReturnError(fmt.Errorf("db error"))
	_, err := srv.GetMyTax(context.Background(), &pb.GetMyTaxRequest{UserId: 1, UserType: "CLIENT"})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestGetMyTax_UserTypeFromCtx(t *testing.T) {
	// UserType empty in request → read from gRPC incoming metadata
	srv, mock := newServer(t)
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"paid", "unpaid"}).AddRow(0.0, 500.0))
	md := metadata.Pairs("user-type", "CLIENT")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	resp, err := srv.GetMyTax(ctx, &pb.GetMyTaxRequest{UserId: 1, UserType: ""})
	require.NoError(t, err)
	assert.InDelta(t, 500.0, resp.UnpaidThisMonth, 0.001)
}

// ── GetTaxList additional paths ───────────────────────────────────────────────

func TestGetTaxList_DBError(t *testing.T) {
	srv, mock := newServer(t)
	mock.ExpectQuery(`SELECT user_id, user_type`).WillReturnError(fmt.Errorf("db error"))
	_, err := srv.GetTaxList(context.Background(), &pb.GetTaxListRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── SetPublicMode additional paths ────────────────────────────────────────────

func TestSetPublicMode_TickerDBInternalError(t *testing.T) {
	srv, _, mSec := newServerWithSecDB(t)
	mSec.ExpectQuery("SELECT id FROM listing WHERE ticker").
		WillReturnError(fmt.Errorf("connection reset"))
	_, err := srv.SetPublicMode(context.Background(), &pb.SetPublicModeRequest{
		UserId: 1, UserType: "CLIENT", Ticker: "AAPL", IsPublic: true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestSetPublicMode_UpdateDBError(t *testing.T) {
	srv, mDB, mSec := newServerWithSecDB(t)
	mSec.ExpectQuery("SELECT id FROM listing WHERE ticker").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))
	mDB.ExpectExec("UPDATE portfolio_entry").
		WillReturnError(fmt.Errorf("update failed"))
	_, err := srv.SetPublicMode(context.Background(), &pb.SetPublicModeRequest{
		UserId: 1, UserType: "CLIENT", Ticker: "AAPL", IsPublic: true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}
