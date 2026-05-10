package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	pb_emp "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/employee"
	pb_fund "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/fund"
	pb_order "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── GetActuaryPerformances ─────────────────────────────────────────────────────

func TestGetActuaryPerformances_Happy(t *testing.T) {
	empSvc := &stubEmpClient{
		getActuaryPerformersFn: func(_ context.Context, _ *pb_emp.GetActuaryPerformersRequest, _ ...grpc.CallOption) (*pb_emp.GetActuaryPerformersResponse, error) {
			return &pb_emp.GetActuaryPerformersResponse{
				Performers: []*pb_emp.ActuaryPerformer{
					{UserId: 3, FirstName: "Ana", LastName: "Jovic", Position: "AGENT"},
					{UserId: 5, FirstName: "Marko", LastName: "Petrovic", Position: "SUPERVISOR"},
				},
			}, nil
		},
	}
	ordSvc := &stubOrderClient{
		getActuaryProfitsFn: func(_ context.Context, req *pb_order.GetActuaryProfitsRequest, _ ...grpc.CallOption) (*pb_order.GetActuaryProfitsResponse, error) {
			return &pb_order.GetActuaryProfitsResponse{
				Profits: []*pb_order.ActuaryProfit{
					{UserId: 3, ProfitRsd: 45230.5},
					{UserId: 5, ProfitRsd: 120000.0},
				},
			}, nil
		},
	}

	w := serveHandlerFull(GetActuaryPerformances(empSvc, ordSvc), "GET", "/bank/profit/actuaries", "/bank/profit/actuaries", "", makeSupervisorToken())
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Len(t, result, 2)

	assert.Equal(t, float64(3), result[0]["userId"])
	assert.Equal(t, "Ana", result[0]["firstName"])
	assert.Equal(t, "AGENT", result[0]["position"])
	assert.InDelta(t, 45230.5, result[0]["profit"], 0.01)

	assert.Equal(t, float64(5), result[1]["userId"])
	assert.Equal(t, "SUPERVISOR", result[1]["position"])
	assert.InDelta(t, 120000.0, result[1]["profit"], 0.01)
}

func TestGetActuaryPerformances_ZeroProfitForNoOrders(t *testing.T) {
	empSvc := &stubEmpClient{
		getActuaryPerformersFn: func(_ context.Context, _ *pb_emp.GetActuaryPerformersRequest, _ ...grpc.CallOption) (*pb_emp.GetActuaryPerformersResponse, error) {
			return &pb_emp.GetActuaryPerformersResponse{
				Performers: []*pb_emp.ActuaryPerformer{
					{UserId: 7, FirstName: "Luka", LastName: "Nikic", Position: "AGENT"},
				},
			}, nil
		},
	}
	ordSvc := &stubOrderClient{
		getActuaryProfitsFn: func(_ context.Context, _ *pb_order.GetActuaryProfitsRequest, _ ...grpc.CallOption) (*pb_order.GetActuaryProfitsResponse, error) {
			// No profits returned — user 7 has no completed orders
			return &pb_order.GetActuaryProfitsResponse{Profits: []*pb_order.ActuaryProfit{}}, nil
		},
	}

	w := serveHandlerFull(GetActuaryPerformances(empSvc, ordSvc), "GET", "/bank/profit/actuaries", "/bank/profit/actuaries", "", makeSupervisorToken())
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Len(t, result, 1)
	assert.Equal(t, float64(0), result[0]["profit"])
}

func TestGetActuaryPerformances_EmpServiceError(t *testing.T) {
	empSvc := &stubEmpClient{
		getActuaryPerformersFn: func(_ context.Context, _ *pb_emp.GetActuaryPerformersRequest, _ ...grpc.CallOption) (*pb_emp.GetActuaryPerformersResponse, error) {
			return nil, status.Error(codes.Internal, "db down")
		},
	}
	w := serveHandlerFull(GetActuaryPerformances(empSvc, &stubOrderClient{}), "GET", "/bank/profit/actuaries", "/bank/profit/actuaries", "", makeSupervisorToken())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetActuaryPerformances_OrderServiceError(t *testing.T) {
	empSvc := &stubEmpClient{
		getActuaryPerformersFn: func(_ context.Context, _ *pb_emp.GetActuaryPerformersRequest, _ ...grpc.CallOption) (*pb_emp.GetActuaryPerformersResponse, error) {
			return &pb_emp.GetActuaryPerformersResponse{
				Performers: []*pb_emp.ActuaryPerformer{{UserId: 1, FirstName: "A", LastName: "B", Position: "AGENT"}},
			}, nil
		},
	}
	ordSvc := &stubOrderClient{
		getActuaryProfitsFn: func(_ context.Context, _ *pb_order.GetActuaryProfitsRequest, _ ...grpc.CallOption) (*pb_order.GetActuaryProfitsResponse, error) {
			return nil, status.Error(codes.Internal, "order db down")
		},
	}
	w := serveHandlerFull(GetActuaryPerformances(empSvc, ordSvc), "GET", "/bank/profit/actuaries", "/bank/profit/actuaries", "", makeSupervisorToken())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── GetBankFundPositions ───────────────────────────────────────────────────────

func TestGetBankFundPositions_Happy(t *testing.T) {
	svc := &stubFundClient{
		getBankPositionsFn: func(_ context.Context, _ *pb_fund.GetBankPositionsRequest, _ ...grpc.CallOption) (*pb_fund.GetBankPositionsResponse, error) {
			return &pb_fund.GetBankPositionsResponse{
				Positions: []*pb_fund.BankFundPosition{
					{
						FundId:           1,
						FundName:         "RAF Growth Fund",
						ManagerName:      "Ana Jovanovic",
						BankSharePercent: 12.5,
						BankShareRsd:     325000.0,
						ProfitRsd:        25000.0,
					},
				},
			}, nil
		},
	}

	w := serveHandlerFull(GetBankFundPositions(svc), "GET", "/bank/profit/fund-positions", "/bank/profit/fund-positions", "", makeSupervisorToken())
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Len(t, result, 1)
	assert.Equal(t, float64(1), result[0]["fundId"])
	assert.Equal(t, "RAF Growth Fund", result[0]["fundName"])
	assert.Equal(t, "Ana Jovanovic", result[0]["managerName"])
	assert.InDelta(t, 12.5, result[0]["bankSharePercent"], 0.001)
	assert.InDelta(t, 325000.0, result[0]["bankShareRSD"], 0.01)
	assert.InDelta(t, 25000.0, result[0]["profitRSD"], 0.01)
}

func TestGetBankFundPositions_Empty(t *testing.T) {
	svc := &stubFundClient{
		getBankPositionsFn: func(_ context.Context, _ *pb_fund.GetBankPositionsRequest, _ ...grpc.CallOption) (*pb_fund.GetBankPositionsResponse, error) {
			return &pb_fund.GetBankPositionsResponse{Positions: []*pb_fund.BankFundPosition{}}, nil
		},
	}
	w := serveHandlerFull(GetBankFundPositions(svc), "GET", "/bank/profit/fund-positions", "/bank/profit/fund-positions", "", makeSupervisorToken())
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Len(t, result, 0)
}

func TestGetBankFundPositions_GrpcError(t *testing.T) {
	svc := &stubFundClient{
		getBankPositionsFn: func(_ context.Context, _ *pb_fund.GetBankPositionsRequest, _ ...grpc.CallOption) (*pb_fund.GetBankPositionsResponse, error) {
			return nil, status.Error(codes.Internal, "fund db down")
		},
	}
	w := serveHandlerFull(GetBankFundPositions(svc), "GET", "/bank/profit/fund-positions", "/bank/profit/fund-positions", "", makeSupervisorToken())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── BankInvestFund ─────────────────────────────────────────────────────────────

func TestBankInvestFund_Happy(t *testing.T) {
	var capturedReq *pb_fund.InvestFundRequest
	svc := &stubFundClient{
		investFundFn: func(_ context.Context, req *pb_fund.InvestFundRequest, _ ...grpc.CallOption) (*pb_fund.FundResponse, error) {
			capturedReq = req
			return sampleFund(), nil
		},
	}

	body := `{"amount": 100000.0, "sourceAccountId": 3}`
	w := serveHandlerFull(BankInvestFund(svc), "POST", "/bank/profit/fund-positions/:fundId/invest", "/bank/profit/fund-positions/1/invest", body, makeSupervisorToken())
	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, capturedReq)
	assert.Equal(t, int64(0), capturedReq.ClientId)
	assert.Equal(t, "BANK", capturedReq.ClientType)
	assert.Equal(t, int64(1), capturedReq.FundId)
	assert.InDelta(t, 100000.0, capturedReq.Amount, 0.01)
	assert.Equal(t, int64(3), capturedReq.SourceAccountId)
}

func TestBankInvestFund_BadFundId(t *testing.T) {
	w := serveHandlerFull(BankInvestFund(&stubFundClient{}), "POST", "/bank/profit/fund-positions/:fundId/invest", "/bank/profit/fund-positions/abc/invest", `{"amount":1.0,"sourceAccountId":1}`, makeSupervisorToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBankInvestFund_BadBody(t *testing.T) {
	w := serveHandlerFull(BankInvestFund(&stubFundClient{}), "POST", "/bank/profit/fund-positions/:fundId/invest", "/bank/profit/fund-positions/1/invest", `{"sourceAccountId":1}`, makeSupervisorToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBankInvestFund_GrpcError(t *testing.T) {
	svc := &stubFundClient{
		investFundFn: func(_ context.Context, _ *pb_fund.InvestFundRequest, _ ...grpc.CallOption) (*pb_fund.FundResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "insufficient balance")
		},
	}
	body := `{"amount": 9999999.0, "sourceAccountId": 3}`
	w := serveHandlerFull(BankInvestFund(svc), "POST", "/bank/profit/fund-positions/:fundId/invest", "/bank/profit/fund-positions/1/invest", body, makeSupervisorToken())
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ── BankRedeemFund ─────────────────────────────────────────────────────────────

func TestBankRedeemFund_Happy(t *testing.T) {
	var capturedReq *pb_fund.WithdrawFundRequest
	svc := &stubFundClient{
		withdrawFundFn: func(_ context.Context, req *pb_fund.WithdrawFundRequest, _ ...grpc.CallOption) (*pb_fund.WithdrawFundResponse, error) {
			capturedReq = req
			return &pb_fund.WithdrawFundResponse{Pending: false, Fund: sampleFund()}, nil
		},
	}

	body := `{"amount": 50000.0, "destinationAccountId": 3}`
	w := serveHandlerFull(BankRedeemFund(svc), "POST", "/bank/profit/fund-positions/:fundId/redeem", "/bank/profit/fund-positions/1/redeem", body, makeSupervisorToken())
	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, capturedReq)
	assert.Equal(t, int64(0), capturedReq.ClientId)
	assert.Equal(t, "BANK", capturedReq.ClientType)
	assert.Equal(t, int64(1), capturedReq.FundId)
	assert.InDelta(t, 50000.0, capturedReq.Amount, 0.01)
	assert.Equal(t, int64(3), capturedReq.DestinationAccountId)
}

func TestBankRedeemFund_BadFundId(t *testing.T) {
	w := serveHandlerFull(BankRedeemFund(&stubFundClient{}), "POST", "/bank/profit/fund-positions/:fundId/redeem", "/bank/profit/fund-positions/xyz/redeem", `{"amount":1.0,"destinationAccountId":1}`, makeSupervisorToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBankRedeemFund_BadBody(t *testing.T) {
	w := serveHandlerFull(BankRedeemFund(&stubFundClient{}), "POST", "/bank/profit/fund-positions/:fundId/redeem", "/bank/profit/fund-positions/1/redeem", `{"destinationAccountId":1}`, makeSupervisorToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBankRedeemFund_GrpcError(t *testing.T) {
	svc := &stubFundClient{
		withdrawFundFn: func(_ context.Context, _ *pb_fund.WithdrawFundRequest, _ ...grpc.CallOption) (*pb_fund.WithdrawFundResponse, error) {
			return nil, status.Error(codes.NotFound, "position not found")
		},
	}
	body := `{"amount": 50000.0, "destinationAccountId": 3}`
	w := serveHandlerFull(BankRedeemFund(svc), "POST", "/bank/profit/fund-positions/:fundId/redeem", "/bank/profit/fund-positions/1/redeem", body, makeSupervisorToken())
	assert.Equal(t, http.StatusNotFound, w.Code)
}
