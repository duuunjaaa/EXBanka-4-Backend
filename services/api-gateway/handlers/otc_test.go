package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/otc"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---- makeEmployeeToken helper ----

func makeEmployeeToken() string {
	claims := jwt.MapClaims{
		"user_id": float64(10),
		"role":    "EMPLOYEE",
		"dozvole": []interface{}{"SUPERVISOR"},
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(""))
	return "Bearer " + tok
}

// ---- stub OTC client ----

type stubOtcClient struct {
	pingFn              func(context.Context, *pb.PingRequest, ...grpc.CallOption) (*pb.PingResponse, error)
	createNegotiationFn func(context.Context, *pb.CreateNegotiationRequest, ...grpc.CallOption) (*pb.NegotiationResponse, error)
	listNegotiationsFn  func(context.Context, *pb.ListNegotiationsRequest, ...grpc.CallOption) (*pb.ListNegotiationsResponse, error)
	getNegotiationFn    func(context.Context, *pb.GetNegotiationRequest, ...grpc.CallOption) (*pb.NegotiationResponse, error)
	counterOfferFn      func(context.Context, *pb.CounterOfferRequest, ...grpc.CallOption) (*pb.NegotiationResponse, error)
	acceptNegotiationFn func(context.Context, *pb.AcceptNegotiationRequest, ...grpc.CallOption) (*pb.NegotiationResponse, error)
	rejectNegotiationFn func(context.Context, *pb.RejectNegotiationRequest, ...grpc.CallOption) (*pb.NegotiationResponse, error)
	listContractsFn     func(context.Context, *pb.ListContractsRequest, ...grpc.CallOption) (*pb.ListContractsResponse, error)
	exerciseContractFn  func(context.Context, *pb.ExerciseContractRequest, ...grpc.CallOption) (*pb.ExerciseContractResponse, error)
	getMarketFn         func(context.Context, *pb.GetMarketRequest, ...grpc.CallOption) (*pb.GetMarketResponse, error)
}

func (s *stubOtcClient) Ping(ctx context.Context, in *pb.PingRequest, opts ...grpc.CallOption) (*pb.PingResponse, error) {
	if s.pingFn != nil {
		return s.pingFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOtcClient) CreateNegotiation(ctx context.Context, in *pb.CreateNegotiationRequest, opts ...grpc.CallOption) (*pb.NegotiationResponse, error) {
	if s.createNegotiationFn != nil {
		return s.createNegotiationFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOtcClient) ListNegotiations(ctx context.Context, in *pb.ListNegotiationsRequest, opts ...grpc.CallOption) (*pb.ListNegotiationsResponse, error) {
	if s.listNegotiationsFn != nil {
		return s.listNegotiationsFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOtcClient) GetNegotiation(ctx context.Context, in *pb.GetNegotiationRequest, opts ...grpc.CallOption) (*pb.NegotiationResponse, error) {
	if s.getNegotiationFn != nil {
		return s.getNegotiationFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOtcClient) CounterOffer(ctx context.Context, in *pb.CounterOfferRequest, opts ...grpc.CallOption) (*pb.NegotiationResponse, error) {
	if s.counterOfferFn != nil {
		return s.counterOfferFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOtcClient) AcceptNegotiation(ctx context.Context, in *pb.AcceptNegotiationRequest, opts ...grpc.CallOption) (*pb.NegotiationResponse, error) {
	if s.acceptNegotiationFn != nil {
		return s.acceptNegotiationFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOtcClient) RejectNegotiation(ctx context.Context, in *pb.RejectNegotiationRequest, opts ...grpc.CallOption) (*pb.NegotiationResponse, error) {
	if s.rejectNegotiationFn != nil {
		return s.rejectNegotiationFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOtcClient) ListContracts(ctx context.Context, in *pb.ListContractsRequest, opts ...grpc.CallOption) (*pb.ListContractsResponse, error) {
	if s.listContractsFn != nil {
		return s.listContractsFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOtcClient) ExerciseContract(ctx context.Context, in *pb.ExerciseContractRequest, opts ...grpc.CallOption) (*pb.ExerciseContractResponse, error) {
	if s.exerciseContractFn != nil {
		return s.exerciseContractFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOtcClient) GetMarket(ctx context.Context, in *pb.GetMarketRequest, opts ...grpc.CallOption) (*pb.GetMarketResponse, error) {
	if s.getMarketFn != nil {
		return s.getMarketFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}

// sampleNegotiation returns a sample NegotiationResponse for tests.
func sampleNegotiation() *pb.NegotiationResponse {
	return &pb.NegotiationResponse{
		Id:             1,
		Ticker:         "AAPL",
		SellerId:       10,
		SellerType:     "EMPLOYEE",
		SellerName:     "Jane Doe",
		BuyerId:        1,
		BuyerType:      "CLIENT",
		BuyerName:      "John Smith",
		Amount:         100,
		PricePerStock:  150.0,
		SettlementDate: "2026-06-01",
		Premium:        0,
		Currency:       "RSD",
		LastModified:   time.Now().Format(time.RFC3339),
		ModifiedById:   1,
		ModifiedByType: "CLIENT",
		Status:         "PENDING_SELLER",
	}
}

var validCreateNegotiationBody = `{
	"sellerId": 10,
	"sellerType": "EMPLOYEE",
	"ticker": "AAPL",
	"amount": 100,
	"pricePerStock": 150.0,
	"settlementDate": "2026-06-01",
	"premium": 0,
	"currency": "RSD"
}`

// ---- CreateNegotiation tests ----

func TestCreateNegotiation_NoToken(t *testing.T) {
	w := serveHandler(CreateNegotiation(&stubOtcClient{}), "POST", "/otc/negotiations", "/otc/negotiations", validCreateNegotiationBody)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestCreateNegotiation_BadBody(t *testing.T) {
	// Missing required fields (ticker, amount, etc.)
	body := `{"sellerId": 10}`
	w := serveHandlerFull(CreateNegotiation(&stubOtcClient{}), "POST", "/otc/negotiations", "/otc/negotiations", body, makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestCreateNegotiation_ServiceError(t *testing.T) {
	svc := &stubOtcClient{
		createNegotiationFn: func(_ context.Context, _ *pb.CreateNegotiationRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			return nil, fmt.Errorf("internal service error")
		},
	}
	w := serveHandlerFull(CreateNegotiation(svc), "POST", "/otc/negotiations", "/otc/negotiations", validCreateNegotiationBody, makeClientToken())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

func TestCreateNegotiation_Happy(t *testing.T) {
	svc := &stubOtcClient{
		createNegotiationFn: func(_ context.Context, _ *pb.CreateNegotiationRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			return sampleNegotiation(), nil
		},
	}
	w := serveHandlerFull(CreateNegotiation(svc), "POST", "/otc/negotiations", "/otc/negotiations", validCreateNegotiationBody, makeClientToken())
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["ticker"] != "AAPL" {
		t.Fatalf("expected ticker AAPL got %v", resp["ticker"])
	}
}

// ---- ListNegotiations tests ----

func TestListNegotiations_NoToken(t *testing.T) {
	w := serveHandler(ListNegotiations(&stubOtcClient{}), "GET", "/otc/negotiations", "/otc/negotiations", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestListNegotiations_Empty(t *testing.T) {
	svc := &stubOtcClient{
		listNegotiationsFn: func(_ context.Context, _ *pb.ListNegotiationsRequest, _ ...grpc.CallOption) (*pb.ListNegotiationsResponse, error) {
			return &pb.ListNegotiationsResponse{Negotiations: []*pb.NegotiationResponse{}}, nil
		},
	}
	w := serveHandlerFull(ListNegotiations(svc), "GET", "/otc/negotiations", "/otc/negotiations", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp) != 0 {
		t.Fatalf("expected empty array got len %d", len(resp))
	}
}

func TestListNegotiations_Happy(t *testing.T) {
	svc := &stubOtcClient{
		listNegotiationsFn: func(_ context.Context, _ *pb.ListNegotiationsRequest, _ ...grpc.CallOption) (*pb.ListNegotiationsResponse, error) {
			return &pb.ListNegotiationsResponse{Negotiations: []*pb.NegotiationResponse{sampleNegotiation()}}, nil
		},
	}
	w := serveHandlerFull(ListNegotiations(svc), "GET", "/otc/negotiations", "/otc/negotiations", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 item got %d", len(resp))
	}
}

// ---- GetNegotiation tests ----

func TestGetNegotiation_NoToken(t *testing.T) {
	w := serveHandler(GetNegotiation(&stubOtcClient{}), "GET", "/otc/negotiations/:id", "/otc/negotiations/1", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestGetNegotiation_BadID(t *testing.T) {
	w := serveHandlerFull(GetNegotiation(&stubOtcClient{}), "GET", "/otc/negotiations/:id", "/otc/negotiations/abc", "", makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestGetNegotiation_NotFound(t *testing.T) {
	svc := &stubOtcClient{
		getNegotiationFn: func(_ context.Context, _ *pb.GetNegotiationRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			return nil, status.Error(codes.NotFound, "negotiation not found")
		},
	}
	w := serveHandlerFull(GetNegotiation(svc), "GET", "/otc/negotiations/:id", "/otc/negotiations/1", "", makeClientToken())
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

func TestGetNegotiation_Happy(t *testing.T) {
	svc := &stubOtcClient{
		getNegotiationFn: func(_ context.Context, _ *pb.GetNegotiationRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			return sampleNegotiation(), nil
		},
	}
	w := serveHandlerFull(GetNegotiation(svc), "GET", "/otc/negotiations/:id", "/otc/negotiations/1", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

// ---- CounterOffer tests ----

var validCounterOfferBody = `{
	"amount": 90,
	"pricePerStock": 155.0,
	"settlementDate": "2026-06-15",
	"premium": 0
}`

func TestCounterOffer_NoToken(t *testing.T) {
	w := serveHandler(CounterOffer(&stubOtcClient{}), "PUT", "/otc/negotiations/:id/counter", "/otc/negotiations/1/counter", validCounterOfferBody)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestCounterOffer_BadBody(t *testing.T) {
	// Missing required fields
	body := `{"amount": 90}`
	w := serveHandlerFull(CounterOffer(&stubOtcClient{}), "PUT", "/otc/negotiations/:id/counter", "/otc/negotiations/1/counter", body, makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestCounterOffer_WrongTurn(t *testing.T) {
	svc := &stubOtcClient{
		counterOfferFn: func(_ context.Context, _ *pb.CounterOfferRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			return nil, status.Error(codes.AlreadyExists, "not your turn: waiting for seller")
		},
	}
	w := serveHandlerFull(CounterOffer(svc), "PUT", "/otc/negotiations/:id/counter", "/otc/negotiations/1/counter", validCounterOfferBody, makeClientToken())
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 got %d", w.Code)
	}
}

func TestCounterOffer_NotParticipant(t *testing.T) {
	svc := &stubOtcClient{
		counterOfferFn: func(_ context.Context, _ *pb.CounterOfferRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			return nil, status.Error(codes.PermissionDenied, "caller is not a participant")
		},
	}
	w := serveHandlerFull(CounterOffer(svc), "PUT", "/otc/negotiations/:id/counter", "/otc/negotiations/1/counter", validCounterOfferBody, makeClientToken())
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", w.Code)
	}
}

func TestCounterOffer_Happy(t *testing.T) {
	svc := &stubOtcClient{
		counterOfferFn: func(_ context.Context, _ *pb.CounterOfferRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			n := sampleNegotiation()
			n.Status = "PENDING_BUYER"
			return n, nil
		},
	}
	w := serveHandlerFull(CounterOffer(svc), "PUT", "/otc/negotiations/:id/counter", "/otc/negotiations/1/counter", validCounterOfferBody, makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

// ---- AcceptNegotiation tests ----

func TestAcceptNegotiation_NoToken(t *testing.T) {
	w := serveHandler(AcceptNegotiation(&stubOtcClient{}), "PUT", "/otc/negotiations/:id/accept", "/otc/negotiations/1/accept", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestAcceptNegotiation_WrongTurn(t *testing.T) {
	svc := &stubOtcClient{
		acceptNegotiationFn: func(_ context.Context, _ *pb.AcceptNegotiationRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			return nil, status.Error(codes.AlreadyExists, "not your turn: waiting for buyer")
		},
	}
	w := serveHandlerFull(AcceptNegotiation(svc), "PUT", "/otc/negotiations/:id/accept", "/otc/negotiations/1/accept", "", makeClientToken())
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 got %d", w.Code)
	}
}

func TestAcceptNegotiation_Happy(t *testing.T) {
	svc := &stubOtcClient{
		acceptNegotiationFn: func(_ context.Context, _ *pb.AcceptNegotiationRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			n := sampleNegotiation()
			n.Status = "ACCEPTED"
			return n, nil
		},
	}
	w := serveHandlerFull(AcceptNegotiation(svc), "PUT", "/otc/negotiations/:id/accept", "/otc/negotiations/1/accept", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["status"] != "ACCEPTED" {
		t.Fatalf("expected status ACCEPTED got %v", resp["status"])
	}
}

// ---- RejectNegotiation tests ----

func TestRejectNegotiation_NoToken(t *testing.T) {
	w := serveHandler(RejectNegotiation(&stubOtcClient{}), "PUT", "/otc/negotiations/:id/reject", "/otc/negotiations/1/reject", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestRejectNegotiation_NotParticipant(t *testing.T) {
	svc := &stubOtcClient{
		rejectNegotiationFn: func(_ context.Context, _ *pb.RejectNegotiationRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			return nil, status.Error(codes.PermissionDenied, "caller is not a participant")
		},
	}
	w := serveHandlerFull(RejectNegotiation(svc), "PUT", "/otc/negotiations/:id/reject", "/otc/negotiations/1/reject", "", makeClientToken())
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", w.Code)
	}
}

func TestRejectNegotiation_Happy(t *testing.T) {
	svc := &stubOtcClient{
		rejectNegotiationFn: func(_ context.Context, _ *pb.RejectNegotiationRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			n := sampleNegotiation()
			n.Status = "REJECTED"
			return n, nil
		},
	}
	w := serveHandlerFull(RejectNegotiation(svc), "PUT", "/otc/negotiations/:id/reject", "/otc/negotiations/1/reject", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["status"] != "REJECTED" {
		t.Fatalf("expected status REJECTED got %v", resp["status"])
	}
}

// ---- ListContracts tests ----

func TestListContracts_NoToken(t *testing.T) {
	w := serveHandler(ListContracts(&stubOtcClient{}), "GET", "/otc/contracts", "/otc/contracts", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestListContracts_Empty(t *testing.T) {
	svc := &stubOtcClient{
		listContractsFn: func(_ context.Context, _ *pb.ListContractsRequest, _ ...grpc.CallOption) (*pb.ListContractsResponse, error) {
			return &pb.ListContractsResponse{Contracts: []*pb.ContractResponse{}}, nil
		},
	}
	w := serveHandlerFull(ListContracts(svc), "GET", "/otc/contracts", "/otc/contracts", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp) != 0 {
		t.Fatalf("expected empty array got len %d", len(resp))
	}
}

func TestListContracts_WithStatusFilter(t *testing.T) {
	var capturedStatus string
	svc := &stubOtcClient{
		listContractsFn: func(_ context.Context, req *pb.ListContractsRequest, _ ...grpc.CallOption) (*pb.ListContractsResponse, error) {
			capturedStatus = req.StatusFilter
			return &pb.ListContractsResponse{Contracts: []*pb.ContractResponse{
				{Id: 1, Ticker: "AAPL", Status: "ACTIVE", SettlementDate: "2026-12-31", CreatedAt: time.Now().Format(time.RFC3339)},
			}}, nil
		},
	}
	w := serveHandlerFull(ListContracts(svc), "GET", "/otc/contracts", "/otc/contracts?status=ACTIVE", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if capturedStatus != "ACTIVE" {
		t.Fatalf("expected StatusFilter=ACTIVE got %q", capturedStatus)
	}
	var resp []interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 1 {
		t.Fatalf("expected 1 contract got %d", len(resp))
	}
}

// ---- ExerciseContract tests ----

func TestExerciseContract_NoToken(t *testing.T) {
	w := serveHandler(ExerciseContract(&stubOtcClient{}), "POST", "/otc/contracts/:id/exercise", "/otc/contracts/1/exercise", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestExerciseContract_BadID(t *testing.T) {
	w := serveHandlerFull(ExerciseContract(&stubOtcClient{}), "POST", "/otc/contracts/:id/exercise", "/otc/contracts/abc/exercise", "", makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestExerciseContract_NotBuyer(t *testing.T) {
	svc := &stubOtcClient{
		exerciseContractFn: func(_ context.Context, _ *pb.ExerciseContractRequest, _ ...grpc.CallOption) (*pb.ExerciseContractResponse, error) {
			return nil, status.Error(codes.PermissionDenied, "only the buyer can exercise the contract")
		},
	}
	w := serveHandlerFull(ExerciseContract(svc), "POST", "/otc/contracts/:id/exercise", "/otc/contracts/1/exercise", `{}`, makeClientToken())
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", w.Code)
	}
}

func TestExerciseContract_InsufficientFunds(t *testing.T) {
	svc := &stubOtcClient{
		exerciseContractFn: func(_ context.Context, _ *pb.ExerciseContractRequest, _ ...grpc.CallOption) (*pb.ExerciseContractResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "Insufficient funds")
		},
	}
	w := serveHandlerFull(ExerciseContract(svc), "POST", "/otc/contracts/:id/exercise", "/otc/contracts/1/exercise", `{}`, makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestExerciseContract_Happy(t *testing.T) {
	svc := &stubOtcClient{
		exerciseContractFn: func(_ context.Context, req *pb.ExerciseContractRequest, _ ...grpc.CallOption) (*pb.ExerciseContractResponse, error) {
			return &pb.ExerciseContractResponse{
				Status:     "EXERCISED",
				ExecutedAt: time.Now().Format(time.RFC3339),
			}, nil
		},
	}
	w := serveHandlerFull(ExerciseContract(svc), "POST", "/otc/contracts/:id/exercise", "/otc/contracts/1/exercise", `{"buyerAccountId": 100}`, makeEmployeeToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["status"] != "EXERCISED" {
		t.Fatalf("expected status EXERCISED got %v", resp["status"])
	}
	if resp["executedAt"] == "" || resp["executedAt"] == nil {
		t.Fatalf("expected non-empty executedAt")
	}
}

// ---- GetMarket tests ----

func TestGetMarket_NoToken(t *testing.T) {
	w := serveHandler(GetMarket(&stubOtcClient{}), "GET", "/otc/market", "/otc/market", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestGetMarket_ClientSeesOtherItems(t *testing.T) {
	svc := &stubOtcClient{
		getMarketFn: func(_ context.Context, req *pb.GetMarketRequest, _ ...grpc.CallOption) (*pb.GetMarketResponse, error) {
			return &pb.GetMarketResponse{Items: []*pb.MarketItem{
				{Ticker: "AAPL", Name: "Apple Inc", Amount: 10, PricePerStock: 150.0, Currency: "USD",
					LastUpdated: time.Now().Format(time.RFC3339), OwnerName: "John Doe", OwnerBank: "EXBanka"},
			}}, nil
		},
	}
	w := serveHandlerFull(GetMarket(svc), "GET", "/otc/market", "/otc/market", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 item got %d", len(resp))
	}
	item := resp[0].(map[string]interface{})
	if item["currency"] != "USD" {
		t.Fatalf("expected currency USD got %v", item["currency"])
	}
}

func TestGetMarket_Empty(t *testing.T) {
	svc := &stubOtcClient{
		getMarketFn: func(_ context.Context, _ *pb.GetMarketRequest, _ ...grpc.CallOption) (*pb.GetMarketResponse, error) {
			return &pb.GetMarketResponse{Items: []*pb.MarketItem{}}, nil
		},
	}
	w := serveHandlerFull(GetMarket(svc), "GET", "/otc/market", "/otc/market", "", makeEmployeeToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp []interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 0 {
		t.Fatalf("expected empty array got %d", len(resp))
	}
}
