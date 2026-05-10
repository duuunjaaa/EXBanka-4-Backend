package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertTaxRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec(`INSERT INTO tax_record`).
		WithArgs(int64(1), "CLIENT", 22.5, 4, 2026).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = InsertTaxRecord(context.Background(), db, 1, "CLIENT", 22.5, 4, 2026)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUnpaidRecords(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery(`SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}).
			AddRow(int64(1), int64(10), "CLIENT", 22.5, 4, 2026, false, (*time.Time)(nil)).
			AddRow(int64(2), int64(20), "EMPLOYEE", 45.0, 4, 2026, false, &now))

	records, err := GetUnpaidRecords(context.Background(), db)
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, int64(1), records[0].ID)
	assert.InDelta(t, 22.5, records[0].AmountRSD, 0.001)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUnpaidRecordsForUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT id, user_id, user_type, amount_rsd, month, year, is_paid, paid_at`).
		WithArgs(int64(5), "CLIENT").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_type", "amount_rsd", "month", "year", "is_paid", "paid_at"}).
			AddRow(int64(3), int64(5), "CLIENT", 15.0, 3, 2026, false, (*time.Time)(nil)))

	records, err := GetUnpaidRecordsForUser(context.Background(), db, 5, "CLIENT")
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, int64(5), records[0].UserID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMarkTaxPaid(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec(`UPDATE tax_record SET is_paid`).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = MarkTaxPaid(context.Background(), db, 7)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMyTax(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT`).
		WithArgs(int64(1), "CLIENT", 2026, 4).
		WillReturnRows(sqlmock.NewRows([]string{"paid", "unpaid"}).AddRow(4500.0, 2250.0))

	paid, unpaid, err := GetMyTax(context.Background(), db, 1, "CLIENT", 2026, 4)
	require.NoError(t, err)
	assert.InDelta(t, 4500.0, paid, 0.001)
	assert.InDelta(t, 2250.0, unpaid, 0.001)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTaxDebtList_NoFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT user_id, user_type`).
		WithArgs("").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "user_type", "debt_rsd"}).
			AddRow(int64(1), "CLIENT", 1500.0).
			AddRow(int64(2), "EMPLOYEE", 750.0))

	debts, err := GetTaxDebtList(context.Background(), db, "")
	require.NoError(t, err)
	require.Len(t, debts, 2)
	assert.Equal(t, int64(1), debts[0].UserID)
	assert.InDelta(t, 1500.0, debts[0].DebtRSD, 0.001)
	assert.Equal(t, "EMPLOYEE", debts[1].UserType)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTaxDebtList_UserTypeFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT user_id, user_type`).
		WithArgs("CLIENT").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "user_type", "debt_rsd"}).
			AddRow(int64(1), "CLIENT", 1500.0))

	debts, err := GetTaxDebtList(context.Background(), db, "CLIENT")
	require.NoError(t, err)
	require.Len(t, debts, 1)
	assert.Equal(t, "CLIENT", debts[0].UserType)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTaxDebtList_IncludesPaidUsers(t *testing.T) {
	// A user with all taxes paid should still appear with debt_rsd = 0
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT user_id, user_type`).
		WithArgs("").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "user_type", "debt_rsd"}).
			AddRow(int64(3), "CLIENT", 0.0))

	debts, err := GetTaxDebtList(context.Background(), db, "")
	require.NoError(t, err)
	require.Len(t, debts, 1)
	assert.InDelta(t, 0.0, debts[0].DebtRSD, 0.001)
	assert.NoError(t, mock.ExpectationsWereMet())
}
