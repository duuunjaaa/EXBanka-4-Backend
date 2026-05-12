package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/otc"
	pb_portfolio "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/portfolio"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---- SetPublicMode tests ----

func TestSetPublicMode_NoToken(t *testing.T) {
	w := serveHandler(SetPublicMode(&stubPortfolioClient{}), "PUT", "/otc/portfolio/:ticker/public", "/otc/portfolio/AAPL/public", `{"isPublic":true}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestSetPublicMode_BadBody(t *testing.T) {
	w := serveHandlerFull(SetPublicMode(&stubPortfolioClient{}), "PUT", "/otc/portfolio/:ticker/public", "/otc/portfolio/AAPL/public", `not-json`, makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestSetPublicMode_NotFound(t *testing.T) {
	svc := &stubPortfolioClient{
		setPublicModeFn: func(_ context.Context, _ *pb_portfolio.SetPublicModeRequest, _ ...grpc.CallOption) (*pb_portfolio.SetPublicModeResponse, error) {
			return nil, status.Error(codes.NotFound, "ticker not in portfolio")
		},
	}
	w := serveHandlerFull(SetPublicMode(svc), "PUT", "/otc/portfolio/:ticker/public", "/otc/portfolio/AAPL/public", `{"isPublic":true}`, makeClientToken())
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

func TestSetPublicMode_InvalidArgument(t *testing.T) {
	svc := &stubPortfolioClient{
		setPublicModeFn: func(_ context.Context, _ *pb_portfolio.SetPublicModeRequest, _ ...grpc.CallOption) (*pb_portfolio.SetPublicModeResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "only stocks can be made public")
		},
	}
	w := serveHandlerFull(SetPublicMode(svc), "PUT", "/otc/portfolio/:ticker/public", "/otc/portfolio/AAPL/public", `{"isPublic":true}`, makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestSetPublicMode_GrpcError(t *testing.T) {
	svc := &stubPortfolioClient{
		setPublicModeFn: func(_ context.Context, _ *pb_portfolio.SetPublicModeRequest, _ ...grpc.CallOption) (*pb_portfolio.SetPublicModeResponse, error) {
			return nil, status.Error(codes.Internal, "db error")
		},
	}
	w := serveHandlerFull(SetPublicMode(svc), "PUT", "/otc/portfolio/:ticker/public", "/otc/portfolio/AAPL/public", `{"isPublic":true}`, makeClientToken())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

func TestSetPublicMode_Happy(t *testing.T) {
	svc := &stubPortfolioClient{
		setPublicModeFn: func(_ context.Context, req *pb_portfolio.SetPublicModeRequest, _ ...grpc.CallOption) (*pb_portfolio.SetPublicModeResponse, error) {
			return &pb_portfolio.SetPublicModeResponse{Ticker: req.Ticker, IsPublic: req.IsPublic}, nil
		},
	}
	w := serveHandlerFull(SetPublicMode(svc), "PUT", "/otc/portfolio/:ticker/public", "/otc/portfolio/AAPL/public", `{"isPublic":true}`, makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if resp["ticker"] != "AAPL" {
		t.Fatalf("expected ticker AAPL got %v", resp["ticker"])
	}
	if resp["isPublic"] != true {
		t.Fatalf("expected isPublic true got %v", resp["isPublic"])
	}
}

func TestSetPublicMode_SetFalse(t *testing.T) {
	svc := &stubPortfolioClient{
		setPublicModeFn: func(_ context.Context, req *pb_portfolio.SetPublicModeRequest, _ ...grpc.CallOption) (*pb_portfolio.SetPublicModeResponse, error) {
			return &pb_portfolio.SetPublicModeResponse{Ticker: req.Ticker, IsPublic: false}, nil
		},
	}
	w := serveHandlerFull(SetPublicMode(svc), "PUT", "/otc/portfolio/:ticker/public", "/otc/portfolio/MSFT/public", `{"isPublic":false}`, makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

// ---- ListNegotiations edge cases ----

func TestListNegotiations_GrpcError(t *testing.T) {
	svc := &stubOtcClient{
		listNegotiationsFn: func(_ context.Context, _ *pb.ListNegotiationsRequest, _ ...grpc.CallOption) (*pb.ListNegotiationsResponse, error) {
			return nil, status.Error(codes.Internal, "db error")
		},
	}
	w := serveHandlerFull(ListNegotiations(svc), "GET", "/otc/negotiations", "/otc/negotiations", "", makeClientToken())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

func TestListNegotiations_ForwardsCallerId(t *testing.T) {
	var capturedCallerId int64
	svc := &stubOtcClient{
		listNegotiationsFn: func(_ context.Context, req *pb.ListNegotiationsRequest, _ ...grpc.CallOption) (*pb.ListNegotiationsResponse, error) {
			capturedCallerId = req.CallerId
			return &pb.ListNegotiationsResponse{Negotiations: []*pb.NegotiationResponse{sampleNegotiation()}}, nil
		},
	}
	w := serveHandlerFull(ListNegotiations(svc), "GET", "/otc/negotiations", "/otc/negotiations", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if capturedCallerId == 0 {
		t.Fatalf("expected callerID to be forwarded, got 0")
	}
}

// ---- CounterOffer edge cases ----

func TestCounterOffer_BadID(t *testing.T) {
	w := serveHandlerFull(CounterOffer(&stubOtcClient{}), "PUT", "/otc/negotiations/:id/counter", "/otc/negotiations/abc/counter", validCounterOfferBody, makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestCounterOffer_GrpcError(t *testing.T) {
	svc := &stubOtcClient{
		counterOfferFn: func(_ context.Context, _ *pb.CounterOfferRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			return nil, status.Error(codes.Internal, "db error")
		},
	}
	w := serveHandlerFull(CounterOffer(svc), "PUT", "/otc/negotiations/:id/counter", "/otc/negotiations/1/counter", validCounterOfferBody, makeClientToken())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

// ---- AcceptNegotiation edge cases ----

func TestAcceptNegotiation_BadID(t *testing.T) {
	w := serveHandlerFull(AcceptNegotiation(&stubOtcClient{}), "PUT", "/otc/negotiations/:id/accept", "/otc/negotiations/xyz/accept", "", makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestAcceptNegotiation_TerminalState(t *testing.T) {
	svc := &stubOtcClient{
		acceptNegotiationFn: func(_ context.Context, _ *pb.AcceptNegotiationRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "negotiation is in terminal state: ACCEPTED")
		},
	}
	w := serveHandlerFull(AcceptNegotiation(svc), "PUT", "/otc/negotiations/:id/accept", "/otc/negotiations/1/accept", "", makeClientToken())
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 got %d", w.Code)
	}
}

func TestAcceptNegotiation_NotFound(t *testing.T) {
	svc := &stubOtcClient{
		acceptNegotiationFn: func(_ context.Context, _ *pb.AcceptNegotiationRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			return nil, status.Error(codes.NotFound, "negotiation not found")
		},
	}
	w := serveHandlerFull(AcceptNegotiation(svc), "PUT", "/otc/negotiations/:id/accept", "/otc/negotiations/1/accept", "", makeClientToken())
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

func TestAcceptNegotiation_InsufficientFunds(t *testing.T) {
	svc := &stubOtcClient{
		acceptNegotiationFn: func(_ context.Context, _ *pb.AcceptNegotiationRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "insufficient funds in buyer account")
		},
	}
	w := serveHandlerFull(AcceptNegotiation(svc), "PUT", "/otc/negotiations/:id/accept", "/otc/negotiations/1/accept", "", makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

// ---- RejectNegotiation edge cases ----

func TestRejectNegotiation_BadID(t *testing.T) {
	w := serveHandlerFull(RejectNegotiation(&stubOtcClient{}), "PUT", "/otc/negotiations/:id/reject", "/otc/negotiations/xyz/reject", "", makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestRejectNegotiation_TerminalState(t *testing.T) {
	svc := &stubOtcClient{
		rejectNegotiationFn: func(_ context.Context, _ *pb.RejectNegotiationRequest, _ ...grpc.CallOption) (*pb.NegotiationResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "negotiation is in terminal state: REJECTED")
		},
	}
	w := serveHandlerFull(RejectNegotiation(svc), "PUT", "/otc/negotiations/:id/reject", "/otc/negotiations/1/reject", "", makeClientToken())
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 got %d", w.Code)
	}
}

// ---- ListContracts edge cases ----

func TestListContracts_GrpcError(t *testing.T) {
	svc := &stubOtcClient{
		listContractsFn: func(_ context.Context, _ *pb.ListContractsRequest, _ ...grpc.CallOption) (*pb.ListContractsResponse, error) {
			return nil, status.Error(codes.Internal, "db error")
		},
	}
	w := serveHandlerFull(ListContracts(svc), "GET", "/otc/contracts", "/otc/contracts", "", makeClientToken())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

func TestListContracts_Happy(t *testing.T) {
	svc := &stubOtcClient{
		listContractsFn: func(_ context.Context, _ *pb.ListContractsRequest, _ ...grpc.CallOption) (*pb.ListContractsResponse, error) {
			return &pb.ListContractsResponse{Contracts: []*pb.ContractResponse{
				{Id: 1, Ticker: "AAPL", Status: "ACTIVE", SettlementDate: "2026-12-31", CreatedAt: time.Now().Format(time.RFC3339)},
			}}, nil
		},
	}
	w := serveHandlerFull(ListContracts(svc), "GET", "/otc/contracts", "/otc/contracts", "", makeEmployeeToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 item got %d", len(resp))
	}
}

// ---- GetMarket edge cases ----

func TestGetMarket_GrpcError(t *testing.T) {
	svc := &stubOtcClient{
		getMarketFn: func(_ context.Context, _ *pb.GetMarketRequest, _ ...grpc.CallOption) (*pb.GetMarketResponse, error) {
			return nil, status.Error(codes.Internal, "db error")
		},
	}
	w := serveHandlerFull(GetMarket(svc), "GET", "/otc/market", "/otc/market", "", makeClientToken())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}
