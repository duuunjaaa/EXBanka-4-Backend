package approval

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var orderCols = []string{
	"id", "user_id", "user_type", "asset_id", "order_type",
	"quantity", "contract_size", "price_per_unit", "limit_value", "stop_value",
	"direction", "status", "approved_by", "is_done", "last_modification",
	"remaining_portions", "after_hours", "is_aon", "is_margin", "account_id", "fund_id",
}

func newMocks(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	orderDB, orderMock, err := sqlmock.New()
	require.NoError(t, err)
	empDB, empMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = orderDB.Close(); _ = empDB.Close() })
	return orderDB, orderMock, empDB, empMock
}

func addOrderRow(rows *sqlmock.Rows, id, userID int64, userType, status string, contractSize int32, pricePerUnit float64, quantity int32) *sqlmock.Rows {
	return addOrderRowDir(rows, id, userID, userType, status, contractSize, pricePerUnit, quantity, "BUY")
}

func addOrderRowDir(rows *sqlmock.Rows, id, userID int64, userType, status string, contractSize int32, pricePerUnit float64, quantity int32, direction string) *sqlmock.Rows {
	ts := time.Now()
	return rows.AddRow(
		id, userID, userType, int64(5), "MARKET",
		quantity, contractSize, pricePerUnit, nil, nil,
		direction, status, nil, false, ts,
		quantity, false, false, false, int64(42), int64(0),
	)
}

// ── ApproveOrder ──────────────────────────────────────────────────────────────

func TestApproveOrder_NotFound(t *testing.T) {
	orderDB, orderMock, empDB, _ := newMocks(t)
	orderMock.ExpectQuery("SELECT id").WillReturnError(sql.ErrNoRows)

	err := ApproveOrder(context.Background(), orderDB, empDB, 99, 1)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestApproveOrder_DBError(t *testing.T) {
	orderDB, orderMock, empDB, _ := newMocks(t)
	orderMock.ExpectQuery("SELECT id").WillReturnError(sql.ErrConnDone)

	err := ApproveOrder(context.Background(), orderDB, empDB, 1, 1)
	assert.ErrorIs(t, err, sql.ErrConnDone)
}

func TestApproveOrder_NotPending(t *testing.T) {
	orderDB, orderMock, empDB, _ := newMocks(t)
	rows := sqlmock.NewRows(orderCols)
	addOrderRow(rows, 1, 10, "CLIENT", "APPROVED", 1, 100.0, 10)
	orderMock.ExpectQuery("SELECT id").WillReturnRows(rows)

	err := ApproveOrder(context.Background(), orderDB, empDB, 1, 5)
	assert.ErrorIs(t, err, ErrNotPending)
}

func TestApproveOrder_UpdateError(t *testing.T) {
	orderDB, orderMock, empDB, _ := newMocks(t)
	rows := sqlmock.NewRows(orderCols)
	addOrderRow(rows, 1, 10, "CLIENT", "PENDING", 1, 100.0, 10)
	orderMock.ExpectQuery("SELECT id").WillReturnRows(rows)
	orderMock.ExpectExec("UPDATE orders SET status").WillReturnError(sql.ErrConnDone)

	err := ApproveOrder(context.Background(), orderDB, empDB, 1, 5)
	assert.ErrorIs(t, err, sql.ErrConnDone)
}

func TestApproveOrder_Happy_ClientOrder(t *testing.T) {
	orderDB, orderMock, empDB, _ := newMocks(t)
	rows := sqlmock.NewRows(orderCols)
	addOrderRow(rows, 1, 10, "CLIENT", "PENDING", 1, 100.0, 10)
	orderMock.ExpectQuery("SELECT id").WillReturnRows(rows)
	orderMock.ExpectExec("UPDATE orders SET status").WillReturnResult(sqlmock.NewResult(1, 1))
	// CLIENT → IsActuary not called

	err := ApproveOrder(context.Background(), orderDB, empDB, 1, 5)
	require.NoError(t, err)
}

func TestApproveOrder_Happy_EmployeeActuary(t *testing.T) {
	orderDB, orderMock, empDB, empMock := newMocks(t)
	rows := sqlmock.NewRows(orderCols)
	addOrderRow(rows, 1, 20, "EMPLOYEE", "PENDING", 2, 150.0, 5)
	orderMock.ExpectQuery("SELECT id").WillReturnRows(rows)
	orderMock.ExpectExec("UPDATE orders SET status").WillReturnResult(sqlmock.NewResult(1, 1))
	// IsActuary
	empMock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	// DeductActuaryUsedLimit: contractSize(2) * pricePerUnit(150) * quantity(5) = 1500
	empMock.ExpectExec("UPDATE actuary_info SET used_limit").WillReturnResult(sqlmock.NewResult(1, 1))

	err := ApproveOrder(context.Background(), orderDB, empDB, 1, 5)
	require.NoError(t, err)
}

func TestApproveOrder_Happy_EmployeeSupervisor(t *testing.T) {
	orderDB, orderMock, empDB, empMock := newMocks(t)
	rows := sqlmock.NewRows(orderCols)
	addOrderRow(rows, 1, 30, "EMPLOYEE", "PENDING", 1, 100.0, 10)
	orderMock.ExpectQuery("SELECT id").WillReturnRows(rows)
	orderMock.ExpectExec("UPDATE orders SET status").WillReturnResult(sqlmock.NewResult(1, 1))
	// IsActuary → false (supervisor)
	empMock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	// No DeductActuaryUsedLimit called

	err := ApproveOrder(context.Background(), orderDB, empDB, 1, 5)
	require.NoError(t, err)
}

func TestApproveOrder_Happy_EmployeeActuary_Sell(t *testing.T) {
	// SELL orders: status is approved but used_limit must NOT be deducted.
	orderDB, orderMock, empDB, empMock := newMocks(t)
	rows := sqlmock.NewRows(orderCols)
	addOrderRowDir(rows, 1, 20, "EMPLOYEE", "PENDING", 2, 150.0, 5, "SELL")
	orderMock.ExpectQuery("SELECT id").WillReturnRows(rows)
	orderMock.ExpectExec("UPDATE orders SET status").WillReturnResult(sqlmock.NewResult(1, 1))
	// IsActuary and DeductActuaryUsedLimit must NOT be called for SELL orders.

	err := ApproveOrder(context.Background(), orderDB, empDB, 1, 5)
	require.NoError(t, err)
	require.NoError(t, empMock.ExpectationsWereMet())
}

// ── DeclineOrder ─────────────────────────────────────────────────────────────

func TestApproveOrder_EmployeeActuary_DeductError(t *testing.T) {
	// DeductActuaryUsedLimit fails → non-fatal (line 35-38 in service.go).
	orderDB, orderMock, empDB, empMock := newMocks(t)
	rows := sqlmock.NewRows(orderCols)
	addOrderRow(rows, 1, 20, "EMPLOYEE", "PENDING", 2, 150.0, 5)
	orderMock.ExpectQuery("SELECT id").WillReturnRows(rows)
	orderMock.ExpectExec("UPDATE orders SET status").WillReturnResult(sqlmock.NewResult(1, 1))
	empMock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	empMock.ExpectExec("UPDATE actuary_info SET used_limit").WillReturnError(fmt.Errorf("db error"))

	err := ApproveOrder(context.Background(), orderDB, empDB, 1, 5)
	require.NoError(t, err)
	require.NoError(t, orderMock.ExpectationsWereMet())
	require.NoError(t, empMock.ExpectationsWereMet())
}

func TestDeclineOrder_NotFound(t *testing.T) {
	orderDB, orderMock, _, _ := newMocks(t)
	orderMock.ExpectQuery("SELECT id").WillReturnError(sql.ErrNoRows)

	err := DeclineOrder(context.Background(), orderDB, 99, 1)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestDeclineOrder_DBError(t *testing.T) {
	orderDB, orderMock, _, _ := newMocks(t)
	orderMock.ExpectQuery("SELECT id").WillReturnError(sql.ErrConnDone)

	err := DeclineOrder(context.Background(), orderDB, 1, 1)
	assert.ErrorIs(t, err, sql.ErrConnDone)
}

func TestDeclineOrder_NotPending(t *testing.T) {
	orderDB, orderMock, _, _ := newMocks(t)
	rows := sqlmock.NewRows(orderCols)
	addOrderRow(rows, 1, 10, "CLIENT", "DECLINED", 1, 100.0, 10)
	orderMock.ExpectQuery("SELECT id").WillReturnRows(rows)

	err := DeclineOrder(context.Background(), orderDB, 1, 5)
	assert.ErrorIs(t, err, ErrNotPending)
}

func TestDeclineOrder_Happy(t *testing.T) {
	orderDB, orderMock, _, _ := newMocks(t)
	rows := sqlmock.NewRows(orderCols)
	addOrderRow(rows, 1, 10, "CLIENT", "PENDING", 1, 100.0, 10)
	orderMock.ExpectQuery("SELECT id").WillReturnRows(rows)
	orderMock.ExpectExec("UPDATE orders SET status").WillReturnResult(sqlmock.NewResult(1, 1))

	err := DeclineOrder(context.Background(), orderDB, 1, 5)
	require.NoError(t, err)
}

func TestDeclineOrder_AutoDecline_ZeroSupervisor(t *testing.T) {
	// supervisorID=0 → approved_by stays nil
	orderDB, orderMock, _, _ := newMocks(t)
	rows := sqlmock.NewRows(orderCols)
	addOrderRow(rows, 1, 10, "CLIENT", "PENDING", 1, 100.0, 10)
	orderMock.ExpectQuery("SELECT id").WillReturnRows(rows)
	orderMock.ExpectExec("UPDATE orders SET status").WillReturnResult(sqlmock.NewResult(1, 1))

	err := DeclineOrder(context.Background(), orderDB, 1, 0)
	require.NoError(t, err)
}
