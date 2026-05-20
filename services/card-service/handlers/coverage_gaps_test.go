package handlers

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/card"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── CreateCard: invalid date_of_birth format (lines 73-75) ───────────────────

func TestCreateCard_InvalidDOB(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)
	// getAccountType → personal
	accountMock.ExpectQuery("SELECT account_type FROM accounts").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("personal"))
	// countAllCards → 0 (under limit)
	dbMock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	_, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "ACC-001",
		CardName:      "MyCard",
		ForSelf:       false,
		AuthorizedPerson: &pb.AuthorizedPersonData{
			FirstName:   "Ana",
			LastName:    "Anić",
			DateOfBirth: "not-a-date",
			Gender:      "F",
		},
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// ── CreateCard: authorized person INSERT error (lines 84-86) ─────────────────

func TestCreateCard_AuthorizedPersonInsertError(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)
	accountMock.ExpectQuery("SELECT account_type FROM accounts").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("personal"))
	dbMock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	dbMock.ExpectQuery("INSERT INTO authorized_persons").
		WillReturnError(sql.ErrConnDone)

	_, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "ACC-001",
		CardName:      "MyCard",
		ForSelf:       false,
		AuthorizedPerson: &pb.AuthorizedPersonData{
			FirstName:   "Ana",
			LastName:    "Anić",
			DateOfBirth: "2000-01-15",
			Gender:      "F",
		},
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateCard: card number uniqueness check error (lines 98-100) ────────────

func TestCreateCard_CardExistsCheckError(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)
	accountMock.ExpectQuery("SELECT account_type FROM accounts").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("personal"))
	dbMock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	// EXISTS check fails
	dbMock.ExpectQuery("SELECT EXISTS").WillReturnError(sql.ErrConnDone)

	_, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "ACC-001",
		CardName:      "MyCard",
		ForSelf:       true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── CreateCard + scanCard: valid cardLimit response (lines 143-145, 507-509) ──
// Run full happy-path for CreateCard returning a non-null card_limit,
// and GetCardsByAccount returning a non-null card_limit (covers scanCard).

func TestCreateCard_WithValidCardLimit(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)
	accountMock.ExpectQuery("SELECT account_type FROM accounts").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("personal"))
	dbMock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	dbMock.ExpectQuery("SELECT EXISTS").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	// INSERT card RETURNING: card_limit is non-null (500.0)
	dbMock.ExpectQuery("INSERT INTO cards").
		WillReturnRows(sqlmock.NewRows([]string{"id", "card_type", "created_at", "card_limit", "status"}).
			AddRow(int64(1), "DEBIT", time.Now(), sql.NullFloat64{Float64: 500.0, Valid: true}, "ACTIVE"))

	resp, err := s.CreateCard(context.Background(), &pb.CreateCardRequest{
		AccountNumber: "ACC-001",
		CardName:      "MyCard",
		ForSelf:       true,
	})
	require.NoError(t, err)
	assert.Equal(t, float64(500), resp.Card.CardLimit)
}

func TestGetCardsByAccount_WithCardLimit(t *testing.T) {
	s, dbMock, _ := newCardServer(t)
	expiry := time.Now().AddDate(3, 0, 0)
	created := time.Now()
	dbMock.ExpectQuery("SELECT id, card_number").
		WillReturnRows(sqlmock.NewRows(cardColumns()).AddRow(
			int64(2), "4111111111111111", "DEBIT", "BizCard",
			expiry, "ACC-BIZ", sql.NullFloat64{Float64: 1000.0, Valid: true}, "ACTIVE", created,
		))

	resp, err := s.GetCardsByAccount(context.Background(), &pb.GetCardsByAccountRequest{AccountNumber: "ACC-BIZ"})
	require.NoError(t, err)
	require.Len(t, resp.Cards, 1)
	assert.Equal(t, float64(1000), resp.Cards[0].CardLimit)
}

// ── DeactivateCard: fetchCardStatusAndAccount internal error (lines 265-267) ─

func TestDeactivateCard_FetchInternalError(t *testing.T) {
	s, dbMock, _ := newCardServer(t)
	dbMock.ExpectQuery("SELECT status, account_number FROM cards").
		WillReturnError(sql.ErrConnDone)

	_, err := s.DeactivateCard(context.Background(), &pb.DeactivateCardRequest{CardNumber: "4111111111111111"})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── UpdateCardLimit: fetchCardStatusAndAccount internal error (lines 284-286) ─

func TestUpdateCardLimit_FetchInternalError(t *testing.T) {
	s, dbMock, _ := newCardServer(t)
	dbMock.ExpectQuery("SELECT status, account_number FROM cards").
		WillReturnError(sql.ErrConnDone)

	_, err := s.UpdateCardLimit(context.Background(), &pb.UpdateCardLimitRequest{CardNumber: "4111111111111111", NewLimit: 500})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── InitiateCardRequest: business card limit exceeded (lines 329-331) ─────────

func TestInitiateCardRequest_BusinessLimitExceeded(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)
	accountMock.ExpectQuery("SELECT account_type FROM accounts").
		WillReturnRows(sqlmock.NewRows([]string{"account_type"}).AddRow("business"))
	// forSelf=true → countOwnerCards; return count = 2 (business self limit is 1)
	dbMock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	_, err := s.InitiateCardRequest(context.Background(), &pb.InitiateCardRequestRequest{
		AccountNumber: "BIZ-001",
		CardName:      "BizCard",
		ForSelf:       true,
	})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

// ── ConfirmCardRequest: json.Unmarshal error (lines 417-419) ─────────────────
// forSelf=false with invalid JSON in authorized_person_data.

func TestConfirmCardRequest_UnmarshalError(t *testing.T) {
	s, dbMock, _ := newCardServer(t)
	dbMock.ExpectQuery("SELECT account_number, card_name").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_number", "card_name", "caller_client_id", "for_self",
			"authorized_person_data", "confirmation_code", "expires_at", "used",
		}).AddRow(
			"ACC-001", "MyCard", int64(1), false,
			[]byte(`{invalid-json`), "123456",
			time.Now().Add(15*time.Minute), false,
		))
	dbMock.ExpectExec("UPDATE card_requests SET used = true").
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err := s.ConfirmCardRequest(context.Background(), &pb.ConfirmCardRequestRequest{
		RequestToken: "tok-001",
		Code:         "123456",
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// ── ConfirmCardRequest: CreateCard error (lines 430-432) ─────────────────────
// Valid confirmation; CreateCard fails because getAccountType returns error.

func TestConfirmCardRequest_CreateCardError(t *testing.T) {
	s, dbMock, accountMock := newCardServer(t)
	dbMock.ExpectQuery("SELECT account_number, card_name").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_number", "card_name", "caller_client_id", "for_self",
			"authorized_person_data", "confirmation_code", "expires_at", "used",
		}).AddRow(
			"ACC-001", "MyCard", int64(1), true,
			[]byte(nil), "654321",
			time.Now().Add(15*time.Minute), false,
		))
	dbMock.ExpectExec("UPDATE card_requests SET used = true").
		WillReturnResult(sqlmock.NewResult(1, 1))
	// CreateCard → getAccountType fails
	accountMock.ExpectQuery("SELECT account_type FROM accounts").
		WillReturnError(sql.ErrConnDone)

	_, err := s.ConfirmCardRequest(context.Background(), &pb.ConfirmCardRequestRequest{
		RequestToken: "tok-002",
		Code:         "654321",
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}
