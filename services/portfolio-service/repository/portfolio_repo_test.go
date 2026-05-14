package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── sharedUserID ──────────────────────────────────────────────────────────────

func TestSharedUserID_Employee(t *testing.T) {
	assert.Equal(t, int64(0), sharedUserID(42, "EMPLOYEE"))
}

func TestSharedUserID_Client(t *testing.T) {
	assert.Equal(t, int64(99), sharedUserID(99, "CLIENT"))
}

func TestSharedUserID_OtherType(t *testing.T) {
	assert.Equal(t, int64(7), sharedUserID(7, "AGENT"))
}

// ── UpsertHolding BUY ─────────────────────────────────────────────────────────

func TestUpsertHolding_BUY_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`INSERT INTO portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	buyPrice, err := UpsertHolding(context.Background(), db, 10, "CLIENT", 5, 42, 3, 100.0, "BUY")
	require.NoError(t, err)
	assert.Equal(t, 0.0, buyPrice)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpsertHolding_BUY_Employee_SharedUID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// EMPLOYEE → uid=0
	mock.ExpectExec(`INSERT INTO portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	buyPrice, err := UpsertHolding(context.Background(), db, 99, "EMPLOYEE", 5, 42, 2, 200.0, "BUY")
	require.NoError(t, err)
	assert.Equal(t, 0.0, buyPrice)
}

func TestUpsertHolding_BUY_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`INSERT INTO portfolio_entry`).
		WillReturnError(sql.ErrConnDone)

	_, err = UpsertHolding(context.Background(), db, 10, "CLIENT", 5, 42, 3, 100.0, "BUY")
	assert.ErrorIs(t, err, sql.ErrConnDone)
}

// ── UpsertHolding SELL ────────────────────────────────────────────────────────

func TestUpsertHolding_SELL_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT buy_price FROM portfolio_entry`).
		WillReturnRows(sqlmock.NewRows([]string{"buy_price"}).AddRow(150.0))
	mock.ExpectExec(`UPDATE portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	buyPrice, err := UpsertHolding(context.Background(), db, 10, "CLIENT", 5, 42, 2, 155.0, "SELL")
	require.NoError(t, err)
	assert.Equal(t, 150.0, buyPrice)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpsertHolding_SELL_ScanError_ContinuesWithZero(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// scan fails → buyPrice stays 0, operation continues
	mock.ExpectQuery(`SELECT buy_price FROM portfolio_entry`).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`UPDATE portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	buyPrice, err := UpsertHolding(context.Background(), db, 10, "CLIENT", 5, 42, 1, 100.0, "SELL")
	require.NoError(t, err)
	assert.Equal(t, 0.0, buyPrice)
}

func TestUpsertHolding_SELL_UpdateError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT buy_price FROM portfolio_entry`).
		WillReturnRows(sqlmock.NewRows([]string{"buy_price"}).AddRow(120.0))
	mock.ExpectExec(`UPDATE portfolio_entry`).
		WillReturnError(sql.ErrConnDone)

	_, err = UpsertHolding(context.Background(), db, 10, "CLIENT", 5, 42, 1, 100.0, "SELL")
	assert.ErrorIs(t, err, sql.ErrConnDone)
}

func TestUpsertHolding_SELL_DeleteError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT buy_price FROM portfolio_entry`).
		WillReturnRows(sqlmock.NewRows([]string{"buy_price"}).AddRow(120.0))
	mock.ExpectExec(`UPDATE portfolio_entry`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM portfolio_entry`).
		WillReturnError(sql.ErrConnDone)

	_, err = UpsertHolding(context.Background(), db, 10, "CLIENT", 5, 42, 1, 100.0, "SELL")
	assert.ErrorIs(t, err, sql.ErrConnDone)
}

// ── GetHoldings ───────────────────────────────────────────────────────────────

var holdingCols = []string{
	"id", "user_id", "user_type", "listing_id", "amount",
	"buy_price", "last_modified", "is_public", "public_amount", "account_id",
}

func TestGetHoldings_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ts := time.Now()
	mock.ExpectQuery(`SELECT id, user_id`).
		WillReturnRows(sqlmock.NewRows(holdingCols).AddRow(
			int64(1), int64(10), "CLIENT", int64(5), int32(3),
			100.0, ts, false, int32(0), int64(42),
		))

	entries, err := GetHoldings(context.Background(), db, 10, "CLIENT")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, int64(1), entries[0].ID)
	assert.Equal(t, int64(10), entries[0].UserID)
	assert.Equal(t, "CLIENT", entries[0].UserType)
	assert.Equal(t, int32(3), entries[0].Amount)
	assert.Equal(t, 100.0, entries[0].BuyPrice)
}

func TestGetHoldings_Employee_SharedUID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ts := time.Now()
	mock.ExpectQuery(`SELECT id, user_id`).
		WillReturnRows(sqlmock.NewRows(holdingCols).AddRow(
			int64(2), int64(0), "EMPLOYEE", int64(7), int32(10),
			200.0, ts, false, int32(0), int64(1),
		))

	entries, err := GetHoldings(context.Background(), db, 99, "EMPLOYEE")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, int64(0), entries[0].UserID)
}

func TestGetHoldings_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT id, user_id`).
		WillReturnRows(sqlmock.NewRows(holdingCols))

	entries, err := GetHoldings(context.Background(), db, 10, "CLIENT")
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

func TestGetHoldings_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT id, user_id`).
		WillReturnError(sql.ErrConnDone)

	_, err = GetHoldings(context.Background(), db, 10, "CLIENT")
	assert.ErrorIs(t, err, sql.ErrConnDone)
}

func TestGetHoldings_ScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// "bad" can't scan into int64 for id field
	mock.ExpectQuery(`SELECT id, user_id`).
		WillReturnRows(sqlmock.NewRows(holdingCols).AddRow(
			"bad", int64(10), "CLIENT", int64(5), int32(3),
			100.0, time.Now(), false, int32(0), int64(42),
		))

	_, err = GetHoldings(context.Background(), db, 10, "CLIENT")
	assert.Error(t, err)
}
