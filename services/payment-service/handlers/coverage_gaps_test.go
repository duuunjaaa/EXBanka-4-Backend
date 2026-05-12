package handlers

// Tests targeting the ~13% uncovered statements in payment-service handlers.
// The existing tests at lines 1881–2393 in grpc_server_test.go use a stale query
// pattern ("SELECT id, owner_id, available_balance, currency_id") that no longer
// matches the actual CREATE TRANSFER query ("SELECT id, owner_id, currency_id").
// These new tests use the correct patterns so the error paths are actually hit.

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── GetPayments: account row scan error (line 451-453) ───────────────────────

func TestGetPayments_AccountScanError(t *testing.T) {
	s, _, accountMock := newMockServer(t)
	// Return 2 columns; accRows.Scan(&an) expects 1 → scan error
	accountMock.ExpectQuery("SELECT account_number FROM accounts").
		WillReturnRows(sqlmock.NewRows([]string{"account_number", "extra"}).
			AddRow("ACC-001", int64(99)))

	_, err := s.GetPayments(context.Background(), &pb.GetPaymentsRequest{ClientId: 1})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── GetPayments: recipientName.Valid = true (lines 523-525) ──────────────────

func TestGetPayments_WithRecipientName(t *testing.T) {
	s, paymentMock, accountMock := newMockServer(t)

	accountMock.ExpectQuery("SELECT account_number FROM accounts").
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow("ACC-100"))

	ts := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)
	paymentMock.ExpectQuery("FROM payments p").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "order_number", "from_account", "to_account",
			"initial_amount", "final_amount", "fee",
			"payment_code", "reference_number", "purpose",
			"timestamp", "status", "name",
		}).AddRow(1, "ORD-001", "ACC-100", "EXT-999", 500.0, 500.0, 0.0,
			"", "", "rent", ts, "COMPLETED", "Landlord Corp")) // name is non-null

	resp, err := s.GetPayments(context.Background(), &pb.GetPaymentsRequest{ClientId: 5})
	require.NoError(t, err)
	require.Len(t, resp.Payments, 1)
	assert.Equal(t, "Landlord Corp", resp.Payments[0].RecipientName)
}

// ── CreateTransfer helpers ────────────────────────────────────────────────────

// expectFromAccount sets up the source account query (no available_balance).
func expectFromAccount(m sqlmock.Sqlmock, id, ownerID, currID int64) {
	m.ExpectQuery("SELECT id, owner_id, currency_id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id", "currency_id"}).
			AddRow(id, ownerID, currID))
}

// expectToAccount sets up the destination account query.
func expectToAccount(m sqlmock.Sqlmock, id, ownerID, currID int64) {
	m.ExpectQuery("SELECT id, owner_id, currency_id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id", "currency_id"}).
			AddRow(id, ownerID, currID))
}

// expectCurrCode sets up a currencies.code lookup.
func expectCurrCode(m sqlmock.Sqlmock, code string) {
	m.ExpectQuery("SELECT code FROM currencies").
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow(code))
}

// expectRate sets up a successful daily_exchange_rates lookup.
func expectRate(m sqlmock.Sqlmock, rate float64) {
	m.ExpectQuery("FROM daily_exchange_rates").
		WillReturnRows(sqlmock.NewRows([]string{"rate"}).AddRow(rate))
}

// expectBankAcct sets up a BANK account lookup.
func expectBankAcct(m sqlmock.Sqlmock, acctNum string) {
	m.ExpectQuery("SELECT account_number FROM accounts WHERE owner_id = 0").
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow(acctNum))
}

// ── CreateTransfer: destination account internal error (lines 595-597) ───────

func TestCreateTransfer_DestAccountInternalError(t *testing.T) {
	s, _, accountMock, _ := newTransferServer(t)
	expectFromAccount(accountMock, 1, 1, 1)
	accountMock.ExpectQuery("SELECT id, owner_id, currency_id").
		WillReturnError(sql.ErrConnDone) // to-account lookup fails with internal error

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "ACC1", ToAccount: "ACC2", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateTransfer: source currency code error (lines 614-616) ───────────────

func TestCreateTransfer_SourceCurrencyCodeError(t *testing.T) {
	s, _, accountMock, exchangeMock := newTransferServer(t)
	// Different currencies so !sameCurrency
	expectFromAccount(accountMock, 1, 1, 2)
	expectToAccount(accountMock, 2, 1, 3)
	exchangeMock.ExpectQuery("SELECT code FROM currencies").
		WillReturnError(sql.ErrConnDone) // source code lookup fails

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "EUR-ACC", ToAccount: "USD-ACC", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateTransfer: destination currency code error (lines 619-621) ──────────

func TestCreateTransfer_DestCurrencyCodeError(t *testing.T) {
	s, _, accountMock, exchangeMock := newTransferServer(t)
	expectFromAccount(accountMock, 1, 1, 2)
	expectToAccount(accountMock, 2, 1, 3)
	expectCurrCode(exchangeMock, "EUR") // source code ok
	exchangeMock.ExpectQuery("SELECT code FROM currencies").
		WillReturnError(sql.ErrConnDone) // dest code fails

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "EUR-ACC", ToAccount: "USD-ACC", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateTransfer: toCode=="RSD" buying_rate error (lines 658-660) ──────────
// from=EUR, to=RSD → getRate("EUR", "buying_rate") fails

func TestCreateTransfer_ToRSD_BuyingRateError(t *testing.T) {
	s, _, accountMock, exchangeMock := newTransferServer(t)
	expectFromAccount(accountMock, 1, 1, 2) // EUR currency
	expectToAccount(accountMock, 2, 1, 1)   // RSD currency (different)
	expectCurrCode(exchangeMock, "EUR")
	expectCurrCode(exchangeMock, "RSD")
	exchangeMock.ExpectQuery("FROM daily_exchange_rates").
		WillReturnError(sql.ErrConnDone) // buying_rate for EUR fails

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "EUR-ACC", ToAccount: "RSD-ACC", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateTransfer: default case fromBuying error (lines 665-667) ────────────
// from=EUR, to=USD → getRate("EUR", "buying_rate") fails

func TestCreateTransfer_BothForeign_FromRateError(t *testing.T) {
	s, _, accountMock, exchangeMock := newTransferServer(t)
	expectFromAccount(accountMock, 1, 1, 2) // EUR
	expectToAccount(accountMock, 2, 1, 3)   // USD
	expectCurrCode(exchangeMock, "EUR")
	expectCurrCode(exchangeMock, "USD")
	exchangeMock.ExpectQuery("FROM daily_exchange_rates").
		WillReturnError(sql.ErrConnDone) // fromBuying fails

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "EUR-ACC", ToAccount: "USD-ACC", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateTransfer: default case toSelling error (lines 669-671) ─────────────
// from=EUR, to=USD → fromBuying ok, getRate("USD", "selling_rate") fails

func TestCreateTransfer_BothForeign_ToRateError(t *testing.T) {
	s, _, accountMock, exchangeMock := newTransferServer(t)
	expectFromAccount(accountMock, 1, 1, 2)
	expectToAccount(accountMock, 2, 1, 3)
	expectCurrCode(exchangeMock, "EUR")
	expectCurrCode(exchangeMock, "USD")
	expectRate(exchangeMock, 115.50) // fromBuying ok
	exchangeMock.ExpectQuery("FROM daily_exchange_rates").
		WillReturnError(sql.ErrConnDone) // toSelling fails

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "EUR-ACC", ToAccount: "USD-ACC", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateTransfer: bank source account error (lines 686-688) ────────────────

func TestCreateTransfer_BankSourceAccountError(t *testing.T) {
	s, _, accountMock, exchangeMock := newTransferServer(t)
	expectFromAccount(accountMock, 1, 1, 2)
	expectToAccount(accountMock, 2, 1, 3)
	expectCurrCode(exchangeMock, "EUR")
	expectCurrCode(exchangeMock, "USD")
	expectRate(exchangeMock, 115.50) // EUR buying_rate
	expectRate(exchangeMock, 110.00) // USD selling_rate
	accountMock.ExpectQuery("SELECT account_number FROM accounts WHERE owner_id = 0").
		WillReturnError(sql.ErrConnDone)

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "EUR-ACC", ToAccount: "USD-ACC", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateTransfer: bank dest account error (lines 692-694) ──────────────────

func TestCreateTransfer_BankDestAccountError(t *testing.T) {
	s, _, accountMock, exchangeMock := newTransferServer(t)
	expectFromAccount(accountMock, 1, 1, 2)
	expectToAccount(accountMock, 2, 1, 3)
	expectCurrCode(exchangeMock, "EUR")
	expectCurrCode(exchangeMock, "USD")
	expectRate(exchangeMock, 115.50)
	expectRate(exchangeMock, 110.00)
	expectBankAcct(accountMock, "BANK-EUR") // source bank ok
	accountMock.ExpectQuery("SELECT account_number FROM accounts WHERE owner_id = 0").
		WillReturnError(sql.ErrConnDone) // dest bank fails

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "EUR-ACC", ToAccount: "USD-ACC", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateTransfer: same-currency BeginTx error (lines 699-701) ──────────────

func TestCreateTransfer_SameCurrency_BeginTxError2(t *testing.T) {
	s, _, accountMock, _ := newTransferServer(t)
	expectFromAccount(accountMock, 1, 1, 1) // both RSD
	expectToAccount(accountMock, 2, 1, 1)
	accountMock.ExpectBegin().WillReturnError(sql.ErrConnDone)

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "RSD1", ToAccount: "RSD2", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateTransfer: same-currency lock source error (lines 709-711) ──────────

func TestCreateTransfer_SameCurrency_LockSourceError(t *testing.T) {
	s, _, accountMock, _ := newTransferServer(t)
	expectFromAccount(accountMock, 1, 1, 1)
	expectToAccount(accountMock, 2, 1, 1)
	accountMock.ExpectBegin()
	accountMock.ExpectQuery("SELECT available_balance FROM accounts").
		WillReturnError(sql.ErrConnDone)
	accountMock.ExpectRollback()

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "RSD1", ToAccount: "RSD2", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateTransfer: same-currency debit error (lines 721-723) ────────────────

func TestCreateTransfer_SameCurrency_DebitError2(t *testing.T) {
	s, _, accountMock, _ := newTransferServer(t)
	expectFromAccount(accountMock, 1, 1, 1)
	expectToAccount(accountMock, 2, 1, 1)
	accountMock.ExpectBegin()
	accountMock.ExpectQuery("SELECT available_balance FROM accounts").
		WillReturnRows(sqlmock.NewRows([]string{"available_balance"}).AddRow(float64(1000)))
	accountMock.ExpectExec("UPDATE accounts SET").WillReturnError(sql.ErrConnDone) // debit fails
	accountMock.ExpectRollback()

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "RSD1", ToAccount: "RSD2", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateTransfer: same-currency credit error (lines 728-730) ───────────────

func TestCreateTransfer_SameCurrency_CreditError2(t *testing.T) {
	s, _, accountMock, _ := newTransferServer(t)
	expectFromAccount(accountMock, 1, 1, 1)
	expectToAccount(accountMock, 2, 1, 1)
	accountMock.ExpectBegin()
	accountMock.ExpectQuery("SELECT available_balance FROM accounts").
		WillReturnRows(sqlmock.NewRows([]string{"available_balance"}).AddRow(float64(1000)))
	accountMock.ExpectExec("UPDATE accounts SET").WillReturnResult(sqlmock.NewResult(1, 1)) // debit ok
	accountMock.ExpectExec("UPDATE accounts SET").WillReturnError(sql.ErrConnDone)          // credit fails
	accountMock.ExpectRollback()

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "RSD1", ToAccount: "RSD2", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// setupCrossCurrencyUpToTx sets up all steps for a EUR→USD transfer through BeginTx.
// Returns (fromAcctNum, toAcctNum) after setting up currency, rate, bank acct expectations.
// Caller should then set up the per-step UPDATE expectations.
func setupCrossCurrencyUpToTx(t *testing.T, accountMock, exchangeMock sqlmock.Sqlmock) {
	t.Helper()
	expectFromAccount(accountMock, 1, 1, 2) // EUR
	expectToAccount(accountMock, 2, 1, 3)   // USD
	expectCurrCode(exchangeMock, "EUR")
	expectCurrCode(exchangeMock, "USD")
	expectRate(exchangeMock, 115.50) // EUR buying_rate
	expectRate(exchangeMock, 110.00) // USD selling_rate
	expectBankAcct(accountMock, "BANK-EUR")
	expectBankAcct(accountMock, "BANK-USD")
	accountMock.ExpectBegin()
	accountMock.ExpectQuery("SELECT available_balance FROM accounts").
		WillReturnRows(sqlmock.NewRows([]string{"available_balance"}).AddRow(float64(1000)))
}

// ── CreateTransfer: cross-currency debit source error (lines 735-737) ────────

func TestCreateTransfer_CrossCurrency_DebitSrcError2(t *testing.T) {
	s, _, accountMock, exchangeMock := newTransferServer(t)
	setupCrossCurrencyUpToTx(t, accountMock, exchangeMock)
	accountMock.ExpectExec("UPDATE accounts").WillReturnError(sql.ErrConnDone) // 1st UPDATE fails
	accountMock.ExpectRollback()

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "EUR-ACC", ToAccount: "USD-ACC", Amount: 50,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateTransfer: cross-currency credit bank source error (lines 740-742) ──

func TestCreateTransfer_CrossCurrency_CreditBankSrcError2(t *testing.T) {
	s, _, accountMock, exchangeMock := newTransferServer(t)
	setupCrossCurrencyUpToTx(t, accountMock, exchangeMock)
	accountMock.ExpectExec("UPDATE accounts").WillReturnResult(sqlmock.NewResult(1, 1)) // debit src ok
	accountMock.ExpectExec("UPDATE accounts").WillReturnError(sql.ErrConnDone)          // credit bank src fails
	accountMock.ExpectRollback()

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "EUR-ACC", ToAccount: "USD-ACC", Amount: 50,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateTransfer: cross-currency debit bank dest error (lines 745-747) ─────

func TestCreateTransfer_CrossCurrency_DebitBankDestError2(t *testing.T) {
	s, _, accountMock, exchangeMock := newTransferServer(t)
	setupCrossCurrencyUpToTx(t, accountMock, exchangeMock)
	accountMock.ExpectExec("UPDATE accounts").WillReturnResult(sqlmock.NewResult(1, 1)) // debit src
	accountMock.ExpectExec("UPDATE accounts").WillReturnResult(sqlmock.NewResult(1, 1)) // credit bank src
	accountMock.ExpectExec("UPDATE accounts").WillReturnError(sql.ErrConnDone)          // debit bank dest fails
	accountMock.ExpectRollback()

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "EUR-ACC", ToAccount: "USD-ACC", Amount: 50,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreatePayment: lock source account scan error (line 163-165) ─────────────
// Same-currency, internal path: BeginTx ok, then lock query fails.

func TestCreatePayment_LockSourceError(t *testing.T) {
	s, _, accountMock := newPaymentServer(t)
	// FROM account
	accountMock.ExpectQuery("SELECT id, owner_id, currency_id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id", "currency_id"}).
			AddRow(int64(1), int64(1), int64(1)))
	// TO account (same currency, internal)
	accountMock.ExpectQuery("SELECT id, currency_id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "currency_id"}).
			AddRow(int64(2), int64(1)))
	// No bank accts needed (sameCurrency && toExists)
	accountMock.ExpectBegin()
	accountMock.ExpectQuery("SELECT available_balance").
		WillReturnError(sql.ErrConnDone)
	accountMock.ExpectRollback()

	_, err := s.CreatePayment(context.Background(), &pb.CreatePaymentRequest{
		ClientId: 1, FromAccount: "ACC1", RecipientAccount: "ACC2", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreatePayment: credit bank account error (lines 199-201) ─────────────────
// External recipient (toExists=false): debit ok, credit bank account fails.

func TestCreatePayment_ExternalCreditBankError(t *testing.T) {
	s, _, accountMock := newPaymentServer(t)
	// FROM account
	accountMock.ExpectQuery("SELECT id, owner_id, currency_id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id", "currency_id"}).
			AddRow(int64(1), int64(1), int64(1)))
	// TO account returns empty → toExists=false → sameCurrency=true (because !toExists)
	accountMock.ExpectQuery("SELECT id, currency_id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "currency_id"}))
	// bankFromAcct needed (!toExists)
	accountMock.ExpectQuery("SELECT account_number FROM accounts WHERE owner_id = 0").
		WillReturnRows(sqlmock.NewRows([]string{"account_number"}).AddRow("BANK-RSD"))
	accountMock.ExpectBegin()
	accountMock.ExpectQuery("SELECT available_balance").
		WillReturnRows(sqlmock.NewRows([]string{
			"available_balance", "daily_limit", "monthly_limit", "daily_spent", "monthly_spent",
		}).AddRow(float64(5000), nil, nil, float64(0), float64(0)))
	accountMock.ExpectExec("UPDATE accounts SET").WillReturnResult(sqlmock.NewResult(1, 1)) // debit ok
	accountMock.ExpectExec("UPDATE accounts SET").WillReturnError(sql.ErrConnDone)          // credit bank fails
	accountMock.ExpectRollback()

	_, err := s.CreatePayment(context.Background(), &pb.CreatePaymentRequest{
		ClientId: 1, FromAccount: "ACC1", RecipientAccount: "EXT-999", Amount: 100,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateTransfer: cross-currency credit destination error (lines 750-752) ──

func TestCreateTransfer_CrossCurrency_CreditDestError2(t *testing.T) {
	s, _, accountMock, exchangeMock := newTransferServer(t)
	setupCrossCurrencyUpToTx(t, accountMock, exchangeMock)
	accountMock.ExpectExec("UPDATE accounts").WillReturnResult(sqlmock.NewResult(1, 1)) // debit src
	accountMock.ExpectExec("UPDATE accounts").WillReturnResult(sqlmock.NewResult(1, 1)) // credit bank src
	accountMock.ExpectExec("UPDATE accounts").WillReturnResult(sqlmock.NewResult(1, 1)) // debit bank dest
	accountMock.ExpectExec("UPDATE accounts").WillReturnError(sql.ErrConnDone)          // credit dest fails
	accountMock.ExpectRollback()

	_, err := s.CreateTransfer(context.Background(), &pb.CreateTransferRequest{
		ClientId: 1, FromAccount: "EUR-ACC", ToAccount: "USD-ACC", Amount: 50,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}
