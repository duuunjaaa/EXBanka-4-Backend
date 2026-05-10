package handlers

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/card"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func newCardServer(t *testing.T) (*CardServer, sqlmock.Sqlmock, sqlmock.Sqlmock) {
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	accountDB, accountMock, err := sqlmock.New()
	require.NoError(t, err)
	s := &CardServer{DB: db, AccountDB: accountDB}
	t.Cleanup(func() { _ = db.Close(); _ = accountDB.Close() })
	return s, dbMock, accountMock
}

func cardColumns() []string {
	return []string{"id", "card_number", "card_type", "card_name", "expiry_date", "account_number", "card_limit", "status", "created_at"}
}

func sampleCardRow(cardNumber string) *sqlmock.Rows {
	expiry := time.Now().AddDate(3, 0, 0)
	created := time.Now()
	return sqlmock.NewRows(cardColumns()).AddRow(
		int64(1), cardNumber, "DEBIT", "TestCard",
		expiry, "265-0001-9139979-78", sql.NullFloat64{Valid: false}, "ACTIVE", created,
	)
}

// ── GetCardsByAccount ─────────────────────────────────────────────────────────

func TestGetCardsByAccount_Empty(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT id, card_number`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows(cardColumns()))

	resp, err := s.GetCardsByAccount(context.Background(), &pb.GetCardsByAccountRequest{AccountNumber: "265-0001-9139979-78"})
	require.NoError(t, err)
	assert.Empty(t, resp.Cards)
}

func TestGetCardsByAccount_Happy(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT id, card_number`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sampleCardRow("5798123456785571"))

	resp, err := s.GetCardsByAccount(context.Background(), &pb.GetCardsByAccountRequest{AccountNumber: "265-0001-9139979-78"})
	require.NoError(t, err)
	require.Len(t, resp.Cards, 1)
	// Card number should be masked
	assert.Equal(t, "5798********5571", resp.Cards[0].CardNumber)
	assert.Equal(t, "ACTIVE", resp.Cards[0].Status)
}

func TestGetCardsByAccount_DBError(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT id, card_number`).
		WithArgs("265-0001-9139979-78").
		WillReturnError(sql.ErrConnDone)

	_, err := s.GetCardsByAccount(context.Background(), &pb.GetCardsByAccountRequest{AccountNumber: "265-0001-9139979-78"})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── GetCardByNumber ───────────────────────────────────────────────────────────

func TestGetCardByNumber_Happy(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT id, card_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sampleCardRow("5798123456785571"))

	resp, err := s.GetCardByNumber(context.Background(), &pb.GetCardByNumberRequest{CardNumber: "5798123456785571"})
	require.NoError(t, err)
	assert.Equal(t, "5798123456785571", resp.Card.CardNumber)
	assert.Equal(t, "ACTIVE", resp.Card.Status)
}

func TestGetCardByNumber_NotFound(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT id, card_number`).
		WithArgs("0000000000000000").
		WillReturnRows(sqlmock.NewRows(cardColumns()))

	_, err := s.GetCardByNumber(context.Background(), &pb.GetCardByNumberRequest{CardNumber: "0000000000000000"})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestGetCardByNumber_DBError(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT id, card_number`).
		WithArgs("5798123456785571").
		WillReturnError(sql.ErrConnDone)

	_, err := s.GetCardByNumber(context.Background(), &pb.GetCardByNumberRequest{CardNumber: "5798123456785571"})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── GetCardById ───────────────────────────────────────────────────────────────

func TestGetCardById_Happy(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT id, card_number`).
		WithArgs(int64(1)).
		WillReturnRows(sampleCardRow("5798123456785571"))

	resp, err := s.GetCardById(context.Background(), &pb.GetCardByIdRequest{Id: 1})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Card.Id)
}

func TestGetCardById_NotFound(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT id, card_number`).
		WithArgs(int64(999)).
		WillReturnRows(sqlmock.NewRows(cardColumns()))

	_, err := s.GetCardById(context.Background(), &pb.GetCardByIdRequest{Id: 999})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// ── BlockCard ─────────────────────────────────────────────────────────────────

func TestBlockCard_NotFound(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}))

	_, err := s.BlockCard(context.Background(), &pb.BlockCardRequest{CardNumber: "5798123456785571"})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestBlockCard_AlreadyBlocked(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("BLOCKED", "265-0001-9139979-78"))

	_, err := s.BlockCard(context.Background(), &pb.BlockCardRequest{CardNumber: "5798123456785571"})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestBlockCard_AdminHappy(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("ACTIVE", "265-0001-9139979-78"))

	dbMock.ExpectExec(`UPDATE cards SET status = 'BLOCKED'`).
		WithArgs("5798123456785571").
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err := s.BlockCard(context.Background(), &pb.BlockCardRequest{
		CardNumber: "5798123456785571",
		CallerRole: "ADMIN",
	})
	require.NoError(t, err)
}

func TestBlockCard_ClientHappy(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("ACTIVE", "265-0001-9139979-78"))

	accountMock.ExpectQuery(`SELECT owner_id FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id"}).AddRow(int64(42)))

	dbMock.ExpectExec(`UPDATE cards SET status = 'BLOCKED'`).
		WithArgs("5798123456785571").
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err := s.BlockCard(context.Background(), &pb.BlockCardRequest{
		CardNumber:     "5798123456785571",
		CallerRole:     "CLIENT",
		CallerClientId: 42,
	})
	require.NoError(t, err)
}

func TestBlockCard_ClientPermissionDenied(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("ACTIVE", "265-0001-9139979-78"))

	accountMock.ExpectQuery(`SELECT owner_id FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id"}).AddRow(int64(99)))

	_, err := s.BlockCard(context.Background(), &pb.BlockCardRequest{
		CardNumber:     "5798123456785571",
		CallerRole:     "CLIENT",
		CallerClientId: 42, // owner is 99
	})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

// ── UnblockCard ───────────────────────────────────────────────────────────────

func TestUnblockCard_NotFound(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}))

	_, err := s.UnblockCard(context.Background(), &pb.UnblockCardRequest{CardNumber: "5798123456785571"})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestUnblockCard_Deactivated(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("DEACTIVATED", "265-0001-9139979-78"))

	_, err := s.UnblockCard(context.Background(), &pb.UnblockCardRequest{CardNumber: "5798123456785571"})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestUnblockCard_NotBlocked(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("ACTIVE", "265-0001-9139979-78"))

	_, err := s.UnblockCard(context.Background(), &pb.UnblockCardRequest{CardNumber: "5798123456785571"})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestUnblockCard_Happy(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("BLOCKED", "265-0001-9139979-78"))

	dbMock.ExpectExec(`UPDATE cards SET status = 'ACTIVE'`).
		WithArgs("5798123456785571").
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err := s.UnblockCard(context.Background(), &pb.UnblockCardRequest{CardNumber: "5798123456785571"})
	require.NoError(t, err)
}

// ── DeactivateCard ────────────────────────────────────────────────────────────

func TestDeactivateCard_AlreadyDeactivated(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("DEACTIVATED", "265-0001-9139979-78"))

	_, err := s.DeactivateCard(context.Background(), &pb.DeactivateCardRequest{CardNumber: "5798123456785571"})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestDeactivateCard_Happy(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("ACTIVE", "265-0001-9139979-78"))

	dbMock.ExpectExec(`UPDATE cards SET status = 'DEACTIVATED'`).
		WithArgs("5798123456785571").
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err := s.DeactivateCard(context.Background(), &pb.DeactivateCardRequest{CardNumber: "5798123456785571"})
	require.NoError(t, err)
}

// ── UpdateCardLimit ───────────────────────────────────────────────────────────

func TestUpdateCardLimit_Deactivated(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("DEACTIVATED", "265-0001-9139979-78"))

	_, err := s.UpdateCardLimit(context.Background(), &pb.UpdateCardLimitRequest{
		CardNumber: "5798123456785571",
		NewLimit:   500.0,
	})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestUpdateCardLimit_Happy(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("ACTIVE", "265-0001-9139979-78"))

	dbMock.ExpectExec(`UPDATE cards SET card_limit`).
		WithArgs(500.0, "5798123456785571").
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err := s.UpdateCardLimit(context.Background(), &pb.UpdateCardLimitRequest{
		CardNumber: "5798123456785571",
		NewLimit:   500.0,
	})
	require.NoError(t, err)
}

// ── InitiateCardRequest ───────────────────────────────────────────────────────

func TestInitiateCardRequest_MissingFields(t *testing.T) {
	s, _, _ := newCardServer(t)

	_, err := s.InitiateCardRequest(context.Background(), &pb.InitiateCardRequestRequest{
		AccountNumber: "",
		CardName:      "",
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestInitiateCardRequest_AccountNotFound(t *testing.T) {
	s, _, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}))

	_, err := s.InitiateCardRequest(context.Background(), &pb.InitiateCardRequestRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "Moja kartica",
		ForSelf:       true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestInitiateCardRequest_PersonalLimitReached(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("personal"))

	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM cards`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2)) // limit is 2

	_, err := s.InitiateCardRequest(context.Background(), &pb.InitiateCardRequestRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "Moja kartica",
		ForSelf:       true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

func TestInitiateCardRequest_PersonalHappy(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("personal"))

	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM cards`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1)) // under limit

	dbMock.ExpectExec(`INSERT INTO card_requests`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	resp, err := s.InitiateCardRequest(context.Background(), &pb.InitiateCardRequestRequest{
		AccountNumber:  "265-0001-9139979-78",
		CardName:       "Moja kartica",
		ForSelf:        true,
		CallerClientId: 42,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.RequestToken)
	assert.Len(t, resp.ConfirmationCode, 6)
}

func TestInitiateCardRequest_BusinessForSelf_Happy(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("business"))

	// countOwnerCards (forSelf=true, business)
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM cards`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	dbMock.ExpectExec(`INSERT INTO card_requests`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	resp, err := s.InitiateCardRequest(context.Background(), &pb.InitiateCardRequestRequest{
		AccountNumber:  "265-0001-9139979-78",
		CardName:       "Business Card",
		ForSelf:        true,
		CallerClientId: 42,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.RequestToken)
}

func TestInitiateCardRequest_BusinessNotForSelf_Happy(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("business"))

	// forSelf=false: no countOwnerCards; authorized person JSON is serialized
	dbMock.ExpectExec(`INSERT INTO card_requests`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	resp, err := s.InitiateCardRequest(context.Background(), &pb.InitiateCardRequestRequest{
		AccountNumber:  "265-0001-9139979-78",
		CardName:       "Auth Card",
		ForSelf:        false,
		CallerClientId: 42,
		AuthorizedPerson: &pb.AuthorizedPersonData{
			FirstName:   "Jane",
			LastName:    "Doe",
			DateOfBirth: "1985-06-15",
			Gender:      "F",
			Email:       "jane@example.com",
			PhoneNumber: "987654321",
			Address:     "Second St 2",
		},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.RequestToken)
}

func TestInitiateCardRequest_InsertError(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("personal"))

	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM cards`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	dbMock.ExpectExec(`INSERT INTO card_requests`).
		WillReturnError(sql.ErrConnDone)

	_, err := s.InitiateCardRequest(context.Background(), &pb.InitiateCardRequestRequest{
		AccountNumber:  "265-0001-9139979-78",
		CardName:       "My Card",
		ForSelf:        true,
		CallerClientId: 42,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── maskCardNumber ────────────────────────────────────────────────────────────

func TestMaskCardNumber(t *testing.T) {
	assert.Equal(t, "5798********5571", maskCardNumber("5798123456785571"))
}

func TestMaskCardNumber_Short(t *testing.T) {
	assert.Equal(t, "1234", maskCardNumber("1234"))
}

// ── CreateCard ────────────────────────────────────────────────────────────────

func TestCreateCard_MissingFields(t *testing.T) {
	s, _, _ := newCardServer(t)

	_, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "",
		CardName:      "",
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateCard_AccountNotFound(t *testing.T) {
	s, _, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}))

	_, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "My Card",
		ForSelf:       true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestCreateCard_PersonalLimitReached(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("personal"))

	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM cards`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	_, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "My Card",
		ForSelf:       true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

func TestCreateCard_PersonalHappy(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("personal"))

	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM cards`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	dbMock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	dbMock.ExpectQuery(`INSERT INTO cards`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "card_type", "created_at", "card_limit", "status"}).
			AddRow(int64(1), "DEBIT", time.Now(), sql.NullFloat64{}, "ACTIVE"))

	resp, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "My Card",
		ForSelf:       true,
	})
	require.NoError(t, err)
	assert.Equal(t, "ACTIVE", resp.Card.Status)
}

func TestCreateCard_BusinessForSelf_LimitReached(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("business"))

	// countOwnerCards — owner already has 1 card
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM cards`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	_, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "My Card",
		ForSelf:       true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

func TestCreateCard_BusinessForSelf_Happy(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("business"))

	// countOwnerCards — 0 owner cards so far
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM cards`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	dbMock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	dbMock.ExpectQuery(`INSERT INTO cards`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "card_type", "created_at", "card_limit", "status"}).
			AddRow(int64(1), "DEBIT", time.Now(), sql.NullFloat64{}, "ACTIVE"))

	resp, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "My Card",
		ForSelf:       true,
	})
	require.NoError(t, err)
	assert.NotNil(t, resp.Card)
}

func TestCreateCard_NotForSelf_MissingAP(t *testing.T) {
	s, _, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("business"))

	_, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber:    "265-0001-9139979-78",
		CardName:         "My Card",
		ForSelf:          false,
		AuthorizedPerson: nil,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateCard_NotForSelf_Happy(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("business"))

	// forSelf=false: no countOwnerCards call; INSERT authorized_person first
	dbMock.ExpectQuery(`INSERT INTO authorized_persons`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	dbMock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	dbMock.ExpectQuery(`INSERT INTO cards`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "card_type", "created_at", "card_limit", "status"}).
			AddRow(int64(1), "DEBIT", time.Now(), sql.NullFloat64{}, "ACTIVE"))

	resp, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "My Card",
		ForSelf:       false,
		AuthorizedPerson: &pb.AuthorizedPersonData{
			FirstName:   "John",
			LastName:    "Doe",
			DateOfBirth: "1990-01-01",
			Gender:      "M",
			Email:       "john@example.com",
			PhoneNumber: "123456789",
			Address:     "Main St 1",
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp.Card)
}

// ── ConfirmCardRequest ────────────────────────────────────────────────────────

func TestConfirmCardRequest_MissingFields(t *testing.T) {
	s, _, _ := newCardServer(t)

	_, err := s.ConfirmCardRequest(context.Background(), &pb.ConfirmCardRequestRequest{
		RequestToken: "",
		Code:         "",
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestConfirmCardRequest_NotFound(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT account_number, card_name`).
		WithArgs("bad-token").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_number", "card_name", "caller_client_id", "for_self",
			"authorized_person_data", "confirmation_code", "expires_at", "used",
		}))

	_, err := s.ConfirmCardRequest(context.Background(), &pb.ConfirmCardRequestRequest{
		RequestToken: "bad-token",
		Code:         "123456",
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestConfirmCardRequest_AlreadyUsed(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT account_number, card_name`).
		WithArgs("used-token").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_number", "card_name", "caller_client_id", "for_self",
			"authorized_person_data", "confirmation_code", "expires_at", "used",
		}).AddRow("265-0001-9139979-78", "My Card", int64(1), true, []byte(nil), "123456", time.Now().Add(15*time.Minute), true))

	_, err := s.ConfirmCardRequest(context.Background(), &pb.ConfirmCardRequestRequest{
		RequestToken: "used-token",
		Code:         "123456",
	})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestConfirmCardRequest_Expired(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT account_number, card_name`).
		WithArgs("expired-token").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_number", "card_name", "caller_client_id", "for_self",
			"authorized_person_data", "confirmation_code", "expires_at", "used",
		}).AddRow("265-0001-9139979-78", "My Card", int64(1), true, []byte(nil), "123456", time.Now().Add(-time.Minute), false))

	_, err := s.ConfirmCardRequest(context.Background(), &pb.ConfirmCardRequestRequest{
		RequestToken: "expired-token",
		Code:         "123456",
	})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestConfirmCardRequest_WrongCode(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT account_number, card_name`).
		WithArgs("valid-token").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_number", "card_name", "caller_client_id", "for_self",
			"authorized_person_data", "confirmation_code", "expires_at", "used",
		}).AddRow("265-0001-9139979-78", "My Card", int64(1), true, []byte(nil), "123456", time.Now().Add(15*time.Minute), false))

	_, err := s.ConfirmCardRequest(context.Background(), &pb.ConfirmCardRequestRequest{
		RequestToken: "valid-token",
		Code:         "000000",
	})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestConfirmCardRequest_Happy(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	expiresAt := time.Now().Add(15 * time.Minute)

	// 1. Fetch card request
	dbMock.ExpectQuery(`SELECT account_number, card_name`).
		WithArgs("valid-token").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_number", "card_name", "caller_client_id", "for_self",
			"authorized_person_data", "confirmation_code", "expires_at", "used",
		}).AddRow("265-0001-9139979-78", "My Card", int64(1), true, []byte(nil), "123456", expiresAt, false))

	// 2. Mark request as used
	dbMock.ExpectExec(`UPDATE card_requests SET used = true`).
		WithArgs("valid-token").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 3. CreateCard: getAccountType
	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("personal"))

	// 4. countAllCards
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM cards`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// 5. Card number uniqueness check
	dbMock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// 6. INSERT card
	dbMock.ExpectQuery(`INSERT INTO cards`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "card_type", "created_at", "card_limit", "status"}).
			AddRow(int64(1), "DEBIT", time.Now(), sql.NullFloat64{}, "ACTIVE"))

	resp, err := s.ConfirmCardRequest(context.Background(), &pb.ConfirmCardRequestRequest{
		RequestToken: "valid-token",
		Code:         "123456",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Card)
	assert.Equal(t, "ACTIVE", resp.Card.Status)
}

func TestConfirmCardRequest_WithAuthorizedPerson(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	expiresAt := time.Now().Add(15 * time.Minute)
	apJSON := []byte(`{"first_name":"Jane","last_name":"Doe","date_of_birth":"1985-06-15","gender":"F","email":"jane@example.com","phone_number":"987654321","address":"Second St 2"}`)

	// 1. Fetch card request (forSelf=false, authorized_person_data set)
	dbMock.ExpectQuery(`SELECT account_number, card_name`).
		WithArgs("ap-token").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_number", "card_name", "caller_client_id", "for_self",
			"authorized_person_data", "confirmation_code", "expires_at", "used",
		}).AddRow("265-0001-9139979-78", "Auth Card", int64(1), false, apJSON, "654321", expiresAt, false))

	// 2. Mark as used
	dbMock.ExpectExec(`UPDATE card_requests SET used = true`).
		WithArgs("ap-token").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 3. CreateCard: getAccountType
	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("business"))

	// 4. INSERT authorized_person
	dbMock.ExpectQuery(`INSERT INTO authorized_persons`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))

	// 5. Card number uniqueness
	dbMock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// 6. INSERT card
	dbMock.ExpectQuery(`INSERT INTO cards`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "card_type", "created_at", "card_limit", "status"}).
			AddRow(int64(2), "DEBIT", time.Now(), sql.NullFloat64{}, "ACTIVE"))

	resp, err := s.ConfirmCardRequest(context.Background(), &pb.ConfirmCardRequestRequest{
		RequestToken: "ap-token",
		Code:         "654321",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Card)
}

// ── DeactivateCard / UpdateCardLimit DB exec error paths ──────────────────────

func TestDeactivateCard_ExecError(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("ACTIVE", "265-0001-9139979-78"))

	dbMock.ExpectExec(`UPDATE cards SET status = 'DEACTIVATED'`).
		WithArgs("5798123456785571").
		WillReturnError(sql.ErrConnDone)

	_, err := s.DeactivateCard(context.Background(), &pb.DeactivateCardRequest{CardNumber: "5798123456785571"})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestUpdateCardLimit_ExecError(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("ACTIVE", "265-0001-9139979-78"))

	dbMock.ExpectExec(`UPDATE cards SET card_limit`).
		WithArgs(500.0, "5798123456785571").
		WillReturnError(sql.ErrConnDone)

	_, err := s.UpdateCardLimit(context.Background(), &pb.UpdateCardLimitRequest{
		CardNumber: "5798123456785571",
		NewLimit:   500.0,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── fetchCardStatusAndAccount internal DB error ───────────────────────────────

func TestBlockCard_FetchDBError(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnError(sql.ErrConnDone)

	_, err := s.BlockCard(context.Background(), &pb.BlockCardRequest{CardNumber: "5798123456785571"})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── getAccountOwnerID internal DB error ──────────────────────────────────────

func TestBlockCard_OwnerIDDBError(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("ACTIVE", "265-0001-9139979-78"))

	accountMock.ExpectQuery(`SELECT owner_id FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnError(sql.ErrConnDone)

	_, err := s.BlockCard(context.Background(), &pb.BlockCardRequest{
		CardNumber:     "5798123456785571",
		CallerRole:     "CLIENT",
		CallerClientId: 42,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── countAllCards DB error ────────────────────────────────────────────────────

func TestCreateCard_CountAllCardsError(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("personal"))

	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM cards`).
		WithArgs("265-0001-9139979-78").
		WillReturnError(sql.ErrConnDone)

	_, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "My Card",
		ForSelf:       true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── countOwnerCards DB error ──────────────────────────────────────────────────

func TestCreateCard_CountOwnerCardsError(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("business"))

	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM cards`).
		WithArgs("265-0001-9139979-78").
		WillReturnError(sql.ErrConnDone)

	_, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "My Card",
		ForSelf:       true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── getAccountType internal DB error ─────────────────────────────────────────

func TestCreateCard_GetAccountTypeDBError(t *testing.T) {
	s, _, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnError(sql.ErrConnDone)

	_, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "My Card",
		ForSelf:       true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateCard INSERT error ───────────────────────────────────────────────────

func TestCreateCard_InsertError(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("personal"))

	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM cards`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	dbMock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	dbMock.ExpectQuery(`INSERT INTO cards`).
		WillReturnError(sql.ErrConnDone)

	_, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "My Card",
		ForSelf:       true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── ConfirmCardRequest mark-as-used error ─────────────────────────────────────

func TestConfirmCardRequest_MarkUsedError(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT account_number, card_name`).
		WithArgs("valid-token2").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_number", "card_name", "caller_client_id", "for_self",
			"authorized_person_data", "confirmation_code", "expires_at", "used",
		}).AddRow("265-0001-9139979-78", "My Card", int64(1), true, []byte(nil), "123456", time.Now().Add(15*time.Minute), false))

	dbMock.ExpectExec(`UPDATE card_requests SET used = true`).
		WithArgs("valid-token2").
		WillReturnError(sql.ErrConnDone)

	_, err := s.ConfirmCardRequest(context.Background(), &pb.ConfirmCardRequestRequest{
		RequestToken: "valid-token2",
		Code:         "123456",
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── GetCardById: internal DB error ───────────────────────────────────────────

func TestGetCardById_DBError(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT id, card_number`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	_, err := s.GetCardById(context.Background(), &pb.GetCardByIdRequest{Id: 1})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── ConfirmCardRequest: fetch error (non-ErrNoRows) ───────────────────────────

func TestConfirmCardRequest_FetchError(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT account_number, card_name`).
		WithArgs("error-token").
		WillReturnError(sql.ErrConnDone)

	_, err := s.ConfirmCardRequest(context.Background(), &pb.ConfirmCardRequestRequest{
		RequestToken: "error-token",
		Code:         "123456",
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── UnblockCard: exec error ───────────────────────────────────────────────────

func TestUnblockCard_ExecError(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("BLOCKED", "265-0001-9139979-78"))

	dbMock.ExpectExec(`UPDATE cards SET status = 'ACTIVE'`).
		WithArgs("5798123456785571").
		WillReturnError(sql.ErrConnDone)

	_, err := s.UnblockCard(context.Background(), &pb.UnblockCardRequest{CardNumber: "5798123456785571"})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── InitiateCardRequest: missing error paths ──────────────────────────────────

func TestInitiateCardRequest_AccountTypeDBError(t *testing.T) {
	s, _, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnError(sql.ErrConnDone)

	_, err := s.InitiateCardRequest(context.Background(), &pb.InitiateCardRequestRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "My Card",
		ForSelf:       true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestInitiateCardRequest_CountAllCardsError(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("personal"))
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM cards`).
		WithArgs("265-0001-9139979-78").
		WillReturnError(sql.ErrConnDone)

	_, err := s.InitiateCardRequest(context.Background(), &pb.InitiateCardRequestRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "My Card",
		ForSelf:       true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestInitiateCardRequest_BusinessForSelf_CountError(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	accountMock.ExpectQuery(`SELECT account_type FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("business"))
	dbMock.ExpectQuery(`SELECT COUNT\(\*\) FROM cards`).
		WithArgs("265-0001-9139979-78").
		WillReturnError(sql.ErrConnDone)

	_, err := s.InitiateCardRequest(context.Background(), &pb.InitiateCardRequestRequest{
		AccountNumber: "265-0001-9139979-78",
		CardName:      "My Card",
		ForSelf:       true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── getAccountOwnerID: account not found in account_db ────────────────────────

func TestBlockCard_AccountNotInAccountDB(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("ACTIVE", "265-0001-9139979-78"))
	// getAccountOwnerID: account not found → ErrNoRows → codes.NotFound
	accountMock.ExpectQuery(`SELECT owner_id FROM accounts`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"owner_id"}))

	_, err := s.BlockCard(context.Background(), &pb.BlockCardRequest{
		CardNumber:     "5798123456785571",
		CallerRole:     "CLIENT",
		CallerClientId: 42,
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// ── scanCard: scan error ──────────────────────────────────────────────────────

func TestGetCardsByAccount_ScanError(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	// Return only 1 column — scanCard expects 9 → Scan returns "expected N destination arguments" error
	dbMock.ExpectQuery(`SELECT id, card_number, card_type, card_name`).
		WithArgs("265-0001-9139979-78").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	_, err := s.GetCardsByAccount(context.Background(), &pb.GetCardsByAccountRequest{
		AccountNumber: "265-0001-9139979-78",
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── BlockCard: exec error ─────────────────────────────────────────────────────

func TestBlockCard_ExecError(t *testing.T) {
	s, dbMock, _ := newCardServer(t)

	dbMock.ExpectQuery(`SELECT status, account_number`).
		WithArgs("5798123456785571").
		WillReturnRows(sqlmock.NewRows([]string{"status", "account_number"}).
			AddRow("ACTIVE", "265-0001-9139979-78"))

	dbMock.ExpectExec(`UPDATE cards SET status = 'BLOCKED'`).
		WithArgs("5798123456785571").
		WillReturnError(sql.ErrConnDone)

	_, err := s.BlockCard(context.Background(), &pb.BlockCardRequest{
		CardNumber: "5798123456785571",
		CallerRole: "ADMIN",
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}
