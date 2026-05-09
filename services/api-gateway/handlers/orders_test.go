package handlers

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	pb_emp "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/employee"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/order"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---- stub client ----

type stubOrderClient struct {
	createOrderFn    func(context.Context, *pb.CreateOrderRequest, ...grpc.CallOption) (*pb.CreateOrderResponse, error)
	listOrdersFn     func(context.Context, *pb.ListOrdersRequest, ...grpc.CallOption) (*pb.ListOrdersResponse, error)
	getOrderByIdFn   func(context.Context, *pb.GetOrderByIdRequest, ...grpc.CallOption) (*pb.GetOrderByIdResponse, error)
	approveOrderFn   func(context.Context, *pb.ApproveOrderRequest, ...grpc.CallOption) (*pb.ApproveOrderResponse, error)
	declineOrderFn   func(context.Context, *pb.DeclineOrderRequest, ...grpc.CallOption) (*pb.DeclineOrderResponse, error)
	cancelOrderFn    func(context.Context, *pb.CancelOrderRequest, ...grpc.CallOption) (*pb.CancelOrderResponse, error)
	cancelPortionsFn func(context.Context, *pb.CancelOrderPortionsRequest, ...grpc.CallOption) (*pb.CancelOrderPortionsResponse, error)
}

func (s *stubOrderClient) Ping(ctx context.Context, in *pb.PingRequest, opts ...grpc.CallOption) (*pb.PingResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOrderClient) CreateOrder(ctx context.Context, in *pb.CreateOrderRequest, opts ...grpc.CallOption) (*pb.CreateOrderResponse, error) {
	if s.createOrderFn != nil {
		return s.createOrderFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOrderClient) ListOrders(ctx context.Context, in *pb.ListOrdersRequest, opts ...grpc.CallOption) (*pb.ListOrdersResponse, error) {
	if s.listOrdersFn != nil {
		return s.listOrdersFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOrderClient) GetOrderById(ctx context.Context, in *pb.GetOrderByIdRequest, opts ...grpc.CallOption) (*pb.GetOrderByIdResponse, error) {
	if s.getOrderByIdFn != nil {
		return s.getOrderByIdFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOrderClient) ApproveOrder(ctx context.Context, in *pb.ApproveOrderRequest, opts ...grpc.CallOption) (*pb.ApproveOrderResponse, error) {
	if s.approveOrderFn != nil {
		return s.approveOrderFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOrderClient) DeclineOrder(ctx context.Context, in *pb.DeclineOrderRequest, opts ...grpc.CallOption) (*pb.DeclineOrderResponse, error) {
	if s.declineOrderFn != nil {
		return s.declineOrderFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOrderClient) CancelOrder(ctx context.Context, in *pb.CancelOrderRequest, opts ...grpc.CallOption) (*pb.CancelOrderResponse, error) {
	if s.cancelOrderFn != nil {
		return s.cancelOrderFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOrderClient) CancelOrderPortions(ctx context.Context, in *pb.CancelOrderPortionsRequest, opts ...grpc.CallOption) (*pb.CancelOrderPortionsResponse, error) {
	if s.cancelPortionsFn != nil {
		return s.cancelPortionsFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubOrderClient) GetActuaryProfits(_ context.Context, _ *pb.GetActuaryProfitsRequest, _ ...grpc.CallOption) (*pb.GetActuaryProfitsResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// ---- stub employee client ----

type stubEmployeeClient struct{}

func (s *stubEmployeeClient) GetAllEmployees(_ context.Context, _ *pb_emp.GetAllEmployeesRequest, _ ...grpc.CallOption) (*pb_emp.GetAllEmployeesResponse, error) {
	return nil, nil
}
func (s *stubEmployeeClient) SearchEmployees(_ context.Context, _ *pb_emp.SearchEmployeesRequest, _ ...grpc.CallOption) (*pb_emp.SearchEmployeesResponse, error) {
	return nil, nil
}
func (s *stubEmployeeClient) GetEmployeeCredentials(_ context.Context, _ *pb_emp.GetEmployeeCredentialsRequest, _ ...grpc.CallOption) (*pb_emp.GetEmployeeCredentialsResponse, error) {
	return nil, nil
}
func (s *stubEmployeeClient) CreateEmployee(_ context.Context, _ *pb_emp.CreateEmployeeRequest, _ ...grpc.CallOption) (*pb_emp.CreateEmployeeResponse, error) {
	return nil, nil
}
func (s *stubEmployeeClient) GetEmployeeById(_ context.Context, _ *pb_emp.GetEmployeeByIdRequest, _ ...grpc.CallOption) (*pb_emp.GetEmployeeByIdResponse, error) {
	return nil, nil
}
func (s *stubEmployeeClient) UpdateEmployee(_ context.Context, _ *pb_emp.UpdateEmployeeRequest, _ ...grpc.CallOption) (*pb_emp.UpdateEmployeeResponse, error) {
	return nil, nil
}
func (s *stubEmployeeClient) ActivateEmployee(_ context.Context, _ *pb_emp.ActivateEmployeeRequest, _ ...grpc.CallOption) (*pb_emp.ActivateEmployeeResponse, error) {
	return nil, nil
}
func (s *stubEmployeeClient) GetEmployeeByEmail(_ context.Context, _ *pb_emp.GetEmployeeByEmailRequest, _ ...grpc.CallOption) (*pb_emp.GetEmployeeByEmailResponse, error) {
	return nil, nil
}
func (s *stubEmployeeClient) UpdatePassword(_ context.Context, _ *pb_emp.UpdatePasswordRequest, _ ...grpc.CallOption) (*pb_emp.UpdatePasswordResponse, error) {
	return nil, nil
}
func (s *stubEmployeeClient) GetActuaries(_ context.Context, _ *pb_emp.GetActuariesRequest, _ ...grpc.CallOption) (*pb_emp.GetActuariesResponse, error) {
	return nil, nil
}
func (s *stubEmployeeClient) SetAgentLimit(_ context.Context, _ *pb_emp.SetAgentLimitRequest, _ ...grpc.CallOption) (*pb_emp.SetAgentLimitResponse, error) {
	return nil, nil
}
func (s *stubEmployeeClient) ResetAgentUsedLimit(_ context.Context, _ *pb_emp.ResetAgentUsedLimitRequest, _ ...grpc.CallOption) (*pb_emp.ResetAgentUsedLimitResponse, error) {
	return nil, nil
}
func (s *stubEmployeeClient) SetNeedApproval(_ context.Context, _ *pb_emp.SetNeedApprovalRequest, _ ...grpc.CallOption) (*pb_emp.SetNeedApprovalResponse, error) {
	return nil, nil
}
func (s *stubEmployeeClient) ResetAllActuaryUsedLimits(_ context.Context, _ *pb_emp.ResetAllActuaryUsedLimitsRequest, _ ...grpc.CallOption) (*pb_emp.ResetAllActuaryUsedLimitsResponse, error) {
	return nil, nil
}
func (s *stubEmployeeClient) GetActuaryPerformers(_ context.Context, _ *pb_emp.GetActuaryPerformersRequest, _ ...grpc.CallOption) (*pb_emp.GetActuaryPerformersResponse, error) {
	return nil, nil
}

// ---- CreateOrder ----

func TestCreateOrder_BadJSON(t *testing.T) {
	w := serveHandlerFull(CreateOrder(&stubOrderClient{}), "POST", "/orders", "/orders", `{bad}`, makeClientToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateOrder_NoToken(t *testing.T) {
	body := `{"assetId":1,"quantity":10,"accountId":42,"direction":"BUY"}`
	w := serveHandlerFull(CreateOrder(&stubOrderClient{}), "POST", "/orders", "/orders", body, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateOrder_GrpcError(t *testing.T) {
	client := &stubOrderClient{
		createOrderFn: func(_ context.Context, _ *pb.CreateOrderRequest, _ ...grpc.CallOption) (*pb.CreateOrderResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "invalid direction")
		},
	}
	body := `{"assetId":1,"quantity":10,"accountId":42,"direction":"BUY"}`
	w := serveHandlerFull(CreateOrder(client), "POST", "/orders", "/orders", body, makeClientToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateOrder_Happy(t *testing.T) {
	client := &stubOrderClient{
		createOrderFn: func(_ context.Context, _ *pb.CreateOrderRequest, _ ...grpc.CallOption) (*pb.CreateOrderResponse, error) {
			return &pb.CreateOrderResponse{OrderId: 7, OrderType: "MARKET", Status: "PENDING", ApproximatePrice: 1050.0}, nil
		},
	}
	body := `{"assetId":1,"quantity":10,"accountId":42,"direction":"BUY"}`
	w := serveHandlerFull(CreateOrder(client), "POST", "/orders", "/orders", body, makeClientToken())
	assert.Equal(t, http.StatusCreated, w.Code)
}

// ---- ListOrders ----

func TestListOrders_GrpcError(t *testing.T) {
	client := &stubOrderClient{
		listOrdersFn: func(_ context.Context, _ *pb.ListOrdersRequest, _ ...grpc.CallOption) (*pb.ListOrdersResponse, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	w := serveHandler(ListOrders(client, &stubEmployeeClient{}, &stubSecuritiesClient{}), "GET", "/orders", "/orders", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListOrders_Happy(t *testing.T) {
	client := &stubOrderClient{
		listOrdersFn: func(_ context.Context, _ *pb.ListOrdersRequest, _ ...grpc.CallOption) (*pb.ListOrdersResponse, error) {
			return &pb.ListOrdersResponse{Orders: []*pb.Order{}}, nil
		},
	}
	w := serveHandler(ListOrders(client, &stubEmployeeClient{}, &stubSecuritiesClient{}), "GET", "/orders", "/orders?status=PENDING", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- GetOrderById ----

func TestGetOrderById_BadId(t *testing.T) {
	w := serveHandler(GetOrderById(&stubOrderClient{}), "GET", "/orders/:id", "/orders/abc", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetOrderById_NotFound(t *testing.T) {
	client := &stubOrderClient{
		getOrderByIdFn: func(_ context.Context, _ *pb.GetOrderByIdRequest, _ ...grpc.CallOption) (*pb.GetOrderByIdResponse, error) {
			return nil, status.Error(codes.NotFound, "order not found")
		},
	}
	w := serveHandler(GetOrderById(client), "GET", "/orders/:id", "/orders/99", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetOrderById_Happy(t *testing.T) {
	client := &stubOrderClient{
		getOrderByIdFn: func(_ context.Context, _ *pb.GetOrderByIdRequest, _ ...grpc.CallOption) (*pb.GetOrderByIdResponse, error) {
			return &pb.GetOrderByIdResponse{Order: &pb.Order{Id: 1}}, nil
		},
	}
	w := serveHandler(GetOrderById(client), "GET", "/orders/:id", "/orders/1", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- ApproveOrder ----

func TestApproveOrder_BadId(t *testing.T) {
	w := serveHandlerFull(ApproveOrder(&stubOrderClient{}), "PUT", "/orders/:id/approve", "/orders/abc/approve", "", makeClientToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestApproveOrder_NoToken(t *testing.T) {
	w := serveHandlerFull(ApproveOrder(&stubOrderClient{}), "PUT", "/orders/:id/approve", "/orders/1/approve", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestApproveOrder_GrpcError(t *testing.T) {
	client := &stubOrderClient{
		approveOrderFn: func(_ context.Context, _ *pb.ApproveOrderRequest, _ ...grpc.CallOption) (*pb.ApproveOrderResponse, error) {
			return nil, status.Error(codes.PermissionDenied, "not supervisor")
		},
	}
	w := serveHandlerFull(ApproveOrder(client), "PUT", "/orders/:id/approve", "/orders/1/approve", "", makeClientToken())
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestApproveOrder_Happy(t *testing.T) {
	client := &stubOrderClient{
		approveOrderFn: func(_ context.Context, _ *pb.ApproveOrderRequest, _ ...grpc.CallOption) (*pb.ApproveOrderResponse, error) {
			return &pb.ApproveOrderResponse{}, nil
		},
	}
	w := serveHandlerFull(ApproveOrder(client), "PUT", "/orders/:id/approve", "/orders/1/approve", "", makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- DeclineOrder ----

func TestDeclineOrder_BadId(t *testing.T) {
	w := serveHandlerFull(DeclineOrder(&stubOrderClient{}), "PUT", "/orders/:id/decline", "/orders/abc/decline", "", makeClientToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeclineOrder_NoToken(t *testing.T) {
	w := serveHandlerFull(DeclineOrder(&stubOrderClient{}), "PUT", "/orders/:id/decline", "/orders/1/decline", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDeclineOrder_GrpcError(t *testing.T) {
	client := &stubOrderClient{
		declineOrderFn: func(_ context.Context, _ *pb.DeclineOrderRequest, _ ...grpc.CallOption) (*pb.DeclineOrderResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "not pending")
		},
	}
	w := serveHandlerFull(DeclineOrder(client), "PUT", "/orders/:id/decline", "/orders/1/decline", "", makeClientToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeclineOrder_Happy(t *testing.T) {
	client := &stubOrderClient{
		declineOrderFn: func(_ context.Context, _ *pb.DeclineOrderRequest, _ ...grpc.CallOption) (*pb.DeclineOrderResponse, error) {
			return &pb.DeclineOrderResponse{}, nil
		},
	}
	w := serveHandlerFull(DeclineOrder(client), "PUT", "/orders/:id/decline", "/orders/1/decline", "", makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- CancelOrder ----

func TestCancelOrder_BadId(t *testing.T) {
	w := serveHandlerFull(CancelOrder(&stubOrderClient{}), "DELETE", "/orders/:id", "/orders/abc", "", makeClientToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCancelOrder_NoToken(t *testing.T) {
	w := serveHandlerFull(CancelOrder(&stubOrderClient{}), "DELETE", "/orders/:id", "/orders/1", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCancelOrder_GrpcError(t *testing.T) {
	client := &stubOrderClient{
		cancelOrderFn: func(_ context.Context, _ *pb.CancelOrderRequest, _ ...grpc.CallOption) (*pb.CancelOrderResponse, error) {
			return nil, fmt.Errorf("internal error")
		},
	}
	w := serveHandlerFull(CancelOrder(client), "DELETE", "/orders/:id", "/orders/1", "", makeClientToken())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCancelOrder_Happy(t *testing.T) {
	client := &stubOrderClient{
		cancelOrderFn: func(_ context.Context, _ *pb.CancelOrderRequest, _ ...grpc.CallOption) (*pb.CancelOrderResponse, error) {
			return &pb.CancelOrderResponse{}, nil
		},
	}
	w := serveHandlerFull(CancelOrder(client), "DELETE", "/orders/:id", "/orders/1", "", makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- CancelOrderPortions ----

func TestCancelOrderPortions_BadId(t *testing.T) {
	w := serveHandlerFull(CancelOrderPortions(&stubOrderClient{}), "DELETE", "/orders/:id/portions", "/orders/abc/portions", "", makeClientToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCancelOrderPortions_NoToken(t *testing.T) {
	w := serveHandlerFull(CancelOrderPortions(&stubOrderClient{}), "DELETE", "/orders/:id/portions", "/orders/1/portions", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCancelOrderPortions_GrpcError(t *testing.T) {
	client := &stubOrderClient{
		cancelPortionsFn: func(_ context.Context, _ *pb.CancelOrderPortionsRequest, _ ...grpc.CallOption) (*pb.CancelOrderPortionsResponse, error) {
			return nil, status.Error(codes.NotFound, "order not found")
		},
	}
	w := serveHandlerFull(CancelOrderPortions(client), "DELETE", "/orders/:id/portions", "/orders/1/portions", "", makeClientToken())
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCancelOrderPortions_Happy(t *testing.T) {
	client := &stubOrderClient{
		cancelPortionsFn: func(_ context.Context, _ *pb.CancelOrderPortionsRequest, _ ...grpc.CallOption) (*pb.CancelOrderPortionsResponse, error) {
			return &pb.CancelOrderPortionsResponse{}, nil
		},
	}
	w := serveHandlerFull(CancelOrderPortions(client), "DELETE", "/orders/:id/portions", "/orders/1/portions", "", makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- orderError: all branches ----

func TestOrderError_NotFound(t *testing.T) {
	client := &stubOrderClient{
		listOrdersFn: func(_ context.Context, _ *pb.ListOrdersRequest, _ ...grpc.CallOption) (*pb.ListOrdersResponse, error) {
			return nil, status.Error(codes.NotFound, "not found")
		},
	}
	w := serveHandler(ListOrders(client, &stubEmployeeClient{}, &stubSecuritiesClient{}), "GET", "/orders", "/orders", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOrderError_PermissionDenied(t *testing.T) {
	client := &stubOrderClient{
		listOrdersFn: func(_ context.Context, _ *pb.ListOrdersRequest, _ ...grpc.CallOption) (*pb.ListOrdersResponse, error) {
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		},
	}
	w := serveHandler(ListOrders(client, &stubEmployeeClient{}, &stubSecuritiesClient{}), "GET", "/orders", "/orders", "")
	assert.Equal(t, http.StatusForbidden, w.Code)
}
