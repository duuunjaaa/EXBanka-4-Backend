package handlers

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/employee"
)

// ── UpdateEmployee: initial SELECT permissions returns no rows (lines 193-195) ──

func TestUpdateEmployee_PermissionsNotFound(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Empty result → sql.ErrNoRows on Scan → codes.NotFound
	dbMock.ExpectQuery("SELECT permissions FROM employees WHERE id").
		WillReturnRows(sqlmock.NewRows([]string{"permissions"}))

	s := &EmployeeServer{DB: db}
	_, err = s.UpdateEmployee(context.Background(), &pb.UpdateEmployeeRequest{Id: 42, Active: false})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

// ── UpdateEmployee: initial SELECT permissions returns a DB error (lines 196-198) ─

func TestUpdateEmployee_PermissionsDBError(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	dbMock.ExpectQuery("SELECT permissions FROM employees WHERE id").
		WillReturnError(sql.ErrConnDone)

	s := &EmployeeServer{DB: db}
	_, err = s.UpdateEmployee(context.Background(), &pb.UpdateEmployeeRequest{Id: 1, Active: false})
	require.Error(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

// ── GetActuaryPerformers: scan error (lines 519-521) ────────────────────────────

func TestGetActuaryPerformers_ScanError(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	dbMock.ExpectQuery(`SELECT id, first_name, last_name`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "first_name", "last_name", "position"}).
			AddRow("bad-id", "Ana", "Anić", "AGENT")) // "bad-id" cannot scan into int64

	s := &EmployeeServer{DB: db}
	_, err = s.GetActuaryPerformers(context.Background(), &pb.GetActuaryPerformersRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.NoError(t, dbMock.ExpectationsWereMet())
}
