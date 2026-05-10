package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/fund"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---- makeSupervisorToken helper ----

func makeSupervisorToken() string {
	claims := jwt.MapClaims{
		"user_id": float64(5),
		"role":    "EMPLOYEE",
		"dozvole": []interface{}{"SUPERVISOR"},
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(""))
	return "Bearer " + tok
}

// ---- stub fund client ----

type stubFundClient struct {
	pingFn                   func(context.Context, *pb.PingRequest, ...grpc.CallOption) (*pb.PingResponse, error)
	createFundFn             func(context.Context, *pb.CreateFundRequest, ...grpc.CallOption) (*pb.FundResponse, error)
	listFundsFn              func(context.Context, *pb.ListFundsRequest, ...grpc.CallOption) (*pb.ListFundsResponse, error)
	getFundFn                func(context.Context, *pb.GetFundRequest, ...grpc.CallOption) (*pb.FundResponse, error)
	updateFundFn             func(context.Context, *pb.UpdateFundRequest, ...grpc.CallOption) (*pb.FundResponse, error)
	deleteFundFn             func(context.Context, *pb.DeleteFundRequest, ...grpc.CallOption) (*pb.DeleteFundResponse, error)
	investFundFn             func(context.Context, *pb.InvestFundRequest, ...grpc.CallOption) (*pb.FundResponse, error)
	withdrawFundFn           func(context.Context, *pb.WithdrawFundRequest, ...grpc.CallOption) (*pb.FundResponse, error)
	getBankPositionsFn       func(context.Context, *pb.GetBankPositionsRequest, ...grpc.CallOption) (*pb.GetBankPositionsResponse, error)
	validateFundAccountFn    func(context.Context, *pb.ValidateFundAccountRequest, ...grpc.CallOption) (*pb.ValidateFundAccountResponse, error)
	updateFundHoldingFn      func(context.Context, *pb.UpdateFundHoldingRequest, ...grpc.CallOption) (*pb.UpdateFundHoldingResponse, error)
	getMyPositionsFn         func(context.Context, *pb.GetMyPositionsRequest, ...grpc.CallOption) (*pb.GetMyPositionsResponse, error)
	transferFundsByManagerFn func(context.Context, *pb.TransferFundsByManagerRequest, ...grpc.CallOption) (*pb.TransferFundsByManagerResponse, error)
}

func (s *stubFundClient) Ping(ctx context.Context, in *pb.PingRequest, opts ...grpc.CallOption) (*pb.PingResponse, error) {
	if s.pingFn != nil {
		return s.pingFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubFundClient) CreateFund(ctx context.Context, in *pb.CreateFundRequest, opts ...grpc.CallOption) (*pb.FundResponse, error) {
	if s.createFundFn != nil {
		return s.createFundFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubFundClient) ListFunds(ctx context.Context, in *pb.ListFundsRequest, opts ...grpc.CallOption) (*pb.ListFundsResponse, error) {
	if s.listFundsFn != nil {
		return s.listFundsFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubFundClient) GetFund(ctx context.Context, in *pb.GetFundRequest, opts ...grpc.CallOption) (*pb.FundResponse, error) {
	if s.getFundFn != nil {
		return s.getFundFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubFundClient) UpdateFund(ctx context.Context, in *pb.UpdateFundRequest, opts ...grpc.CallOption) (*pb.FundResponse, error) {
	if s.updateFundFn != nil {
		return s.updateFundFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubFundClient) DeleteFund(ctx context.Context, in *pb.DeleteFundRequest, opts ...grpc.CallOption) (*pb.DeleteFundResponse, error) {
	if s.deleteFundFn != nil {
		return s.deleteFundFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubFundClient) InvestFund(ctx context.Context, in *pb.InvestFundRequest, opts ...grpc.CallOption) (*pb.FundResponse, error) {
	if s.investFundFn != nil {
		return s.investFundFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubFundClient) WithdrawFund(ctx context.Context, in *pb.WithdrawFundRequest, opts ...grpc.CallOption) (*pb.FundResponse, error) {
	if s.withdrawFundFn != nil {
		return s.withdrawFundFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubFundClient) GetBankPositions(ctx context.Context, in *pb.GetBankPositionsRequest, opts ...grpc.CallOption) (*pb.GetBankPositionsResponse, error) {
	if s.getBankPositionsFn != nil {
		return s.getBankPositionsFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubFundClient) ValidateFundAccount(ctx context.Context, in *pb.ValidateFundAccountRequest, opts ...grpc.CallOption) (*pb.ValidateFundAccountResponse, error) {
	if s.validateFundAccountFn != nil {
		return s.validateFundAccountFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubFundClient) UpdateFundHolding(ctx context.Context, in *pb.UpdateFundHoldingRequest, opts ...grpc.CallOption) (*pb.UpdateFundHoldingResponse, error) {
	if s.updateFundHoldingFn != nil {
		return s.updateFundHoldingFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubFundClient) GetMyPositions(ctx context.Context, in *pb.GetMyPositionsRequest, opts ...grpc.CallOption) (*pb.GetMyPositionsResponse, error) {
	if s.getMyPositionsFn != nil {
		return s.getMyPositionsFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubFundClient) TransferFundsByManager(ctx context.Context, in *pb.TransferFundsByManagerRequest, opts ...grpc.CallOption) (*pb.TransferFundsByManagerResponse, error) {
	if s.transferFundsByManagerFn != nil {
		return s.transferFundsByManagerFn(ctx, in, opts...)
	}
	return &pb.TransferFundsByManagerResponse{}, nil
}

// sampleFund returns a sample FundResponse.
func sampleFund() *pb.FundResponse {
	return &pb.FundResponse{
		Id:                  1,
		Name:                "Test Fund",
		Description:         "A test investment fund",
		MinimumContribution: 1000.0,
		ManagerId:           5,
		ManagerName:         "Jane Manager",
		LiquidAssets:        500000.0,
		FundValue:           500000.0,
		Profit:              0.0,
		AccountNumber:       "123-456-78",
		AccountId:           100,
		CreatedAt:           time.Now().Format(time.RFC3339),
		Active:              true,
	}
}

var validCreateFundBody = `{
	"name": "Test Fund",
	"description": "A test fund",
	"minimumContribution": 1000.0,
	"managerId": 5
}`

// ---- CreateFund tests ----

func TestCreateFund_NoToken(t *testing.T) {
	w := serveHandler(CreateFund(&stubFundClient{}), "POST", "/investment/funds", "/investment/funds", validCreateFundBody)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestCreateFund_BadBody(t *testing.T) {
	// Missing name
	body := `{"description": "no name fund"}`
	w := serveHandlerFull(CreateFund(&stubFundClient{}), "POST", "/investment/funds", "/investment/funds", body, makeSupervisorToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestCreateFund_Duplicate(t *testing.T) {
	svc := &stubFundClient{
		createFundFn: func(_ context.Context, _ *pb.CreateFundRequest, _ ...grpc.CallOption) (*pb.FundResponse, error) {
			return nil, status.Error(codes.AlreadyExists, "fund with name already exists")
		},
	}
	w := serveHandlerFull(CreateFund(svc), "POST", "/investment/funds", "/investment/funds", validCreateFundBody, makeSupervisorToken())
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 got %d", w.Code)
	}
}

func TestCreateFund_Happy(t *testing.T) {
	svc := &stubFundClient{
		createFundFn: func(_ context.Context, _ *pb.CreateFundRequest, _ ...grpc.CallOption) (*pb.FundResponse, error) {
			return sampleFund(), nil
		},
	}
	w := serveHandlerFull(CreateFund(svc), "POST", "/investment/funds", "/investment/funds", validCreateFundBody, makeSupervisorToken())
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["name"] != "Test Fund" {
		t.Fatalf("expected name 'Test Fund' got %v", resp["name"])
	}
}

// ---- ListFunds tests ----

func TestListFunds_NoToken(t *testing.T) {
	w := serveHandler(ListFunds(&stubFundClient{}), "GET", "/investment/funds", "/investment/funds", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestListFunds_Empty(t *testing.T) {
	svc := &stubFundClient{
		listFundsFn: func(_ context.Context, _ *pb.ListFundsRequest, _ ...grpc.CallOption) (*pb.ListFundsResponse, error) {
			return &pb.ListFundsResponse{Funds: []*pb.FundResponse{}}, nil
		},
	}
	w := serveHandlerFull(ListFunds(svc), "GET", "/investment/funds", "/investment/funds", "", makeClientToken())
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

func TestListFunds_WithManagerFilter(t *testing.T) {
	var capturedManagerID int64
	svc := &stubFundClient{
		listFundsFn: func(_ context.Context, req *pb.ListFundsRequest, _ ...grpc.CallOption) (*pb.ListFundsResponse, error) {
			capturedManagerID = req.ManagerIdFilter
			return &pb.ListFundsResponse{Funds: []*pb.FundResponse{sampleFund()}}, nil
		},
	}
	w := serveHandlerFull(ListFunds(svc), "GET", "/investment/funds", "/investment/funds?managerId=1", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if capturedManagerID != 1 {
		t.Fatalf("expected managerId filter 1 got %d", capturedManagerID)
	}
}

func TestListFunds_Happy(t *testing.T) {
	svc := &stubFundClient{
		listFundsFn: func(_ context.Context, _ *pb.ListFundsRequest, _ ...grpc.CallOption) (*pb.ListFundsResponse, error) {
			f2 := sampleFund()
			f2.Id = 2
			f2.Name = "Second Fund"
			return &pb.ListFundsResponse{Funds: []*pb.FundResponse{sampleFund(), f2}}, nil
		},
	}
	w := serveHandlerFull(ListFunds(svc), "GET", "/investment/funds", "/investment/funds", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("expected 2 funds got %d", len(resp))
	}
}

// ---- GetFund tests ----

func TestGetFund_NoToken(t *testing.T) {
	w := serveHandler(GetFund(&stubFundClient{}), "GET", "/investment/funds/:id", "/investment/funds/1", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestGetFund_BadID(t *testing.T) {
	w := serveHandlerFull(GetFund(&stubFundClient{}), "GET", "/investment/funds/:id", "/investment/funds/abc", "", makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestGetFund_NotFound(t *testing.T) {
	svc := &stubFundClient{
		getFundFn: func(_ context.Context, _ *pb.GetFundRequest, _ ...grpc.CallOption) (*pb.FundResponse, error) {
			return nil, status.Error(codes.NotFound, "fund not found")
		},
	}
	w := serveHandlerFull(GetFund(svc), "GET", "/investment/funds/:id", "/investment/funds/99", "", makeClientToken())
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

func TestGetFund_Happy(t *testing.T) {
	svc := &stubFundClient{
		getFundFn: func(_ context.Context, _ *pb.GetFundRequest, _ ...grpc.CallOption) (*pb.FundResponse, error) {
			return sampleFund(), nil
		},
	}
	w := serveHandlerFull(GetFund(svc), "GET", "/investment/funds/:id", "/investment/funds/1", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if _, ok := resp["fundValue"]; !ok {
		t.Fatalf("expected fundValue in response, got keys: %v", resp)
	}
}

// ---- UpdateFund tests ----

var validUpdateFundBody = `{
	"name": "Updated Fund",
	"description": "Updated description",
	"minimumContribution": 2000.0,
	"managerId": 5
}`

func TestUpdateFund_NoToken(t *testing.T) {
	w := serveHandler(UpdateFund(&stubFundClient{}), "PUT", "/investment/funds/:id", "/investment/funds/1", validUpdateFundBody)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestUpdateFund_BadID(t *testing.T) {
	w := serveHandlerFull(UpdateFund(&stubFundClient{}), "PUT", "/investment/funds/:id", "/investment/funds/abc", validUpdateFundBody, makeSupervisorToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestUpdateFund_NotFound(t *testing.T) {
	svc := &stubFundClient{
		updateFundFn: func(_ context.Context, _ *pb.UpdateFundRequest, _ ...grpc.CallOption) (*pb.FundResponse, error) {
			return nil, status.Error(codes.NotFound, "fund not found")
		},
	}
	w := serveHandlerFull(UpdateFund(svc), "PUT", "/investment/funds/:id", "/investment/funds/99", validUpdateFundBody, makeSupervisorToken())
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

func TestUpdateFund_Happy(t *testing.T) {
	svc := &stubFundClient{
		updateFundFn: func(_ context.Context, _ *pb.UpdateFundRequest, _ ...grpc.CallOption) (*pb.FundResponse, error) {
			f := sampleFund()
			f.Name = "Updated Fund"
			return f, nil
		},
	}
	w := serveHandlerFull(UpdateFund(svc), "PUT", "/investment/funds/:id", "/investment/funds/1", validUpdateFundBody, makeSupervisorToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

// ---- DeleteFund tests ----

func TestDeleteFund_NoToken(t *testing.T) {
	w := serveHandler(DeleteFund(&stubFundClient{}), "DELETE", "/investment/funds/:id", "/investment/funds/1", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestDeleteFund_HasPositions(t *testing.T) {
	svc := &stubFundClient{
		deleteFundFn: func(_ context.Context, _ *pb.DeleteFundRequest, _ ...grpc.CallOption) (*pb.DeleteFundResponse, error) {
			return nil, status.Error(codes.PermissionDenied, "cannot delete fund with active client positions")
		},
	}
	w := serveHandlerFull(DeleteFund(svc), "DELETE", "/investment/funds/:id", "/investment/funds/1", "", makeSupervisorToken())
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", w.Code)
	}
}

func TestDeleteFund_NotFound(t *testing.T) {
	svc := &stubFundClient{
		deleteFundFn: func(_ context.Context, _ *pb.DeleteFundRequest, _ ...grpc.CallOption) (*pb.DeleteFundResponse, error) {
			return nil, status.Error(codes.NotFound, "fund not found")
		},
	}
	w := serveHandlerFull(DeleteFund(svc), "DELETE", "/investment/funds/:id", "/investment/funds/99", "", makeSupervisorToken())
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

func TestDeleteFund_Happy(t *testing.T) {
	svc := &stubFundClient{
		deleteFundFn: func(_ context.Context, _ *pb.DeleteFundRequest, _ ...grpc.CallOption) (*pb.DeleteFundResponse, error) {
			return &pb.DeleteFundResponse{}, nil
		},
	}
	w := serveHandlerFull(DeleteFund(svc), "DELETE", "/investment/funds/:id", "/investment/funds/1", "", makeSupervisorToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["message"] != "fund deleted" {
		t.Fatalf("expected message 'fund deleted' got %v", resp["message"])
	}
}

func TestGetMyPositions_NoToken(t *testing.T) {
	w := serveHandlerFull(GetMyPositions(&stubFundClient{}), "GET", "/client/funds/positions", "/client/funds/positions", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestGetMyPositions_Empty(t *testing.T) {
	svc := &stubFundClient{
		getMyPositionsFn: func(_ context.Context, _ *pb.GetMyPositionsRequest, _ ...grpc.CallOption) (*pb.GetMyPositionsResponse, error) {
			return &pb.GetMyPositionsResponse{Positions: []*pb.ClientFundPosition{}}, nil
		},
	}
	w := serveHandlerFull(GetMyPositions(svc), "GET", "/client/funds/positions", "/client/funds/positions", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp) != 0 {
		t.Fatalf("expected empty array, got %d items", len(resp))
	}
}

func TestGetMyPositions_Happy(t *testing.T) {
	svc := &stubFundClient{
		getMyPositionsFn: func(_ context.Context, req *pb.GetMyPositionsRequest, _ ...grpc.CallOption) (*pb.GetMyPositionsResponse, error) {
			return &pb.GetMyPositionsResponse{
				Positions: []*pb.ClientFundPosition{
					{
						FundId:               7,
						FundName:             "RAF Growth Fund",
						Description:          "A growth fund",
						FundValue:            50000,
						FundPercentage:       100,
						CurrentPositionValue: 50000,
						TotalInvestedAmount:  10000,
						Profit:               40000,
						MinimumContribution:  1000,
					},
				},
			}, nil
		},
	}
	w := serveHandlerFull(GetMyPositions(svc), "GET", "/client/funds/positions", "/client/funds/positions", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	var resp []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 position, got %d", len(resp))
	}
	if resp[0]["fundName"] != "RAF Growth Fund" {
		t.Fatalf("unexpected fundName: %v", resp[0]["fundName"])
	}
	if resp[0]["profit"] != float64(40000) {
		t.Fatalf("unexpected profit: %v", resp[0]["profit"])
	}
}
