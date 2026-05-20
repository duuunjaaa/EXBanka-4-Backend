package handlers

import (
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// TestStartCronJobs covers StartCronJobs and the synchronous setup lines in
// runDailyReset (up to time.Sleep). The goroutine blocks at the sleep and is
// killed when the test binary exits — no goroutine leak concern in tests.
func TestStartCronJobs(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	s := &EmployeeServer{DB: db}
	s.StartCronJobs()
	// Give the goroutine a few ms to reach time.Sleep (lines 16-22 in cron.go).
	time.Sleep(5 * time.Millisecond)
}
