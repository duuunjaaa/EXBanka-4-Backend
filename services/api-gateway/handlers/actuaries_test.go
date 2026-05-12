package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/employee"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func sampleActuaryInfo() *pb.ActuaryInfo {
	return &pb.ActuaryInfo{
		EmployeeId:   1,
		FirstName:    "Marko",
		LastName:     "Markovic",
		Email:        "marko@banka.rs",
		Position:     "Agent",
		LimitAmount:  100000,
		UsedLimit:    15000,
		NeedApproval: false,
	}
}

// ---- GetActuaries ----

func TestGetActuaries_Happy(t *testing.T) {
	client := &stubEmpClient{
		getActuariesFn: func(_ context.Context, _ *pb.GetActuariesRequest, _ ...grpc.CallOption) (*pb.GetActuariesResponse, error) {
			return &pb.GetActuariesResponse{Actuaries: []*pb.ActuaryInfo{sampleActuaryInfo()}}, nil
		},
	}
	w := serveHandler(GetActuaries(client), "GET", "/api/actuaries", "/api/actuaries", "")
	require.Equal(t, http.StatusOK, w.Code)
	var resp []actuaryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, int64(1), resp[0].EmployeeId)
	assert.Equal(t, "Marko", resp[0].FirstName)
	assert.Equal(t, 100000.0, resp[0].LimitAmount)
}

func TestGetActuaries_Empty(t *testing.T) {
	client := &stubEmpClient{
		getActuariesFn: func(_ context.Context, _ *pb.GetActuariesRequest, _ ...grpc.CallOption) (*pb.GetActuariesResponse, error) {
			return &pb.GetActuariesResponse{Actuaries: []*pb.ActuaryInfo{}}, nil
		},
	}
	w := serveHandler(GetActuaries(client), "GET", "/api/actuaries", "/api/actuaries", "")
	require.Equal(t, http.StatusOK, w.Code)
	var resp []actuaryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 0)
}

func TestGetActuaries_ForwardsQueryParams(t *testing.T) {
	var capturedReq *pb.GetActuariesRequest
	client := &stubEmpClient{
		getActuariesFn: func(_ context.Context, in *pb.GetActuariesRequest, _ ...grpc.CallOption) (*pb.GetActuariesResponse, error) {
			capturedReq = in
			return &pb.GetActuariesResponse{Actuaries: []*pb.ActuaryInfo{}}, nil
		},
	}
	w := serveHandler(GetActuaries(client), "GET", "/api/actuaries", "/api/actuaries?email=marko&first_name=Marko", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, capturedReq)
	assert.Equal(t, "marko", capturedReq.Email)
	assert.Equal(t, "Marko", capturedReq.FirstName)
}

func TestGetActuaries_Error(t *testing.T) {
	client := &stubEmpClient{
		getActuariesFn: func(_ context.Context, _ *pb.GetActuariesRequest, _ ...grpc.CallOption) (*pb.GetActuariesResponse, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	w := serveHandler(GetActuaries(client), "GET", "/api/actuaries", "/api/actuaries", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ---- SetAgentLimit ----

func TestSetAgentLimit_InvalidId(t *testing.T) {
	client := &stubEmpClient{}
	w := serveHandler(SetAgentLimit(client), "PUT", "/api/actuaries/:id/limit", "/api/actuaries/abc/limit", `{"limit":5000}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetAgentLimit_BadJSON(t *testing.T) {
	client := &stubEmpClient{}
	w := serveHandler(SetAgentLimit(client), "PUT", "/api/actuaries/:id/limit", "/api/actuaries/1/limit", `{bad}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetAgentLimit_NegativeLimit(t *testing.T) {
	client := &stubEmpClient{}
	w := serveHandler(SetAgentLimit(client), "PUT", "/api/actuaries/:id/limit", "/api/actuaries/1/limit", `{"limit":-1}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetAgentLimit_NotFound(t *testing.T) {
	client := &stubEmpClient{
		setAgentLimitFn: func(_ context.Context, _ *pb.SetAgentLimitRequest, _ ...grpc.CallOption) (*pb.SetAgentLimitResponse, error) {
			return nil, status.Error(codes.NotFound, "employee not found")
		},
	}
	w := serveHandler(SetAgentLimit(client), "PUT", "/api/actuaries/:id/limit", "/api/actuaries/99/limit", `{"limit":5000}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSetAgentLimit_NotAgent(t *testing.T) {
	client := &stubEmpClient{
		setAgentLimitFn: func(_ context.Context, _ *pb.SetAgentLimitRequest, _ ...grpc.CallOption) (*pb.SetAgentLimitResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "employee is not an agent")
		},
	}
	w := serveHandler(SetAgentLimit(client), "PUT", "/api/actuaries/:id/limit", "/api/actuaries/1/limit", `{"limit":5000}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetAgentLimit_Happy(t *testing.T) {
	client := &stubEmpClient{
		setAgentLimitFn: func(_ context.Context, in *pb.SetAgentLimitRequest, _ ...grpc.CallOption) (*pb.SetAgentLimitResponse, error) {
			assert.Equal(t, int64(1), in.EmployeeId)
			assert.Equal(t, 50000.0, in.LimitAmount)
			return &pb.SetAgentLimitResponse{}, nil
		},
	}
	w := serveHandler(SetAgentLimit(client), "PUT", "/api/actuaries/:id/limit", "/api/actuaries/1/limit", `{"limit":50000}`)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "limit updated", resp["message"])
}

// ---- ResetAgentUsedLimit ----

func TestResetAgentUsedLimit_InvalidId(t *testing.T) {
	client := &stubEmpClient{}
	w := serveHandler(ResetAgentUsedLimit(client), "POST", "/api/actuaries/:id/reset-used-limit", "/api/actuaries/abc/reset-used-limit", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResetAgentUsedLimit_NotFound(t *testing.T) {
	client := &stubEmpClient{
		resetUsedLimitFn: func(_ context.Context, _ *pb.ResetAgentUsedLimitRequest, _ ...grpc.CallOption) (*pb.ResetAgentUsedLimitResponse, error) {
			return nil, status.Error(codes.NotFound, "not found")
		},
	}
	w := serveHandler(ResetAgentUsedLimit(client), "POST", "/api/actuaries/:id/reset-used-limit", "/api/actuaries/99/reset-used-limit", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestResetAgentUsedLimit_Happy(t *testing.T) {
	client := &stubEmpClient{
		resetUsedLimitFn: func(_ context.Context, in *pb.ResetAgentUsedLimitRequest, _ ...grpc.CallOption) (*pb.ResetAgentUsedLimitResponse, error) {
			assert.Equal(t, int64(5), in.EmployeeId)
			return &pb.ResetAgentUsedLimitResponse{}, nil
		},
	}
	w := serveHandler(ResetAgentUsedLimit(client), "POST", "/api/actuaries/:id/reset-used-limit", "/api/actuaries/5/reset-used-limit", "")
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "used_limit reset", resp["message"])
}

// ---- SetNeedApproval ----

func TestSetNeedApproval_InvalidId(t *testing.T) {
	client := &stubEmpClient{}
	w := serveHandler(SetNeedApproval(client), "PUT", "/api/actuaries/:id/need-approval", "/api/actuaries/abc/need-approval", `{"need_approval":true}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetNeedApproval_BadJSON(t *testing.T) {
	client := &stubEmpClient{}
	w := serveHandler(SetNeedApproval(client), "PUT", "/api/actuaries/:id/need-approval", "/api/actuaries/1/need-approval", `{bad}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetNeedApproval_NotFound(t *testing.T) {
	client := &stubEmpClient{
		setNeedApprovalFn: func(_ context.Context, _ *pb.SetNeedApprovalRequest, _ ...grpc.CallOption) (*pb.SetNeedApprovalResponse, error) {
			return nil, status.Error(codes.NotFound, "not found")
		},
	}
	w := serveHandler(SetNeedApproval(client), "PUT", "/api/actuaries/:id/need-approval", "/api/actuaries/99/need-approval", `{"need_approval":true}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSetNeedApproval_Happy(t *testing.T) {
	client := &stubEmpClient{
		setNeedApprovalFn: func(_ context.Context, in *pb.SetNeedApprovalRequest, _ ...grpc.CallOption) (*pb.SetNeedApprovalResponse, error) {
			assert.Equal(t, int64(3), in.EmployeeId)
			assert.True(t, in.NeedApproval)
			return &pb.SetNeedApprovalResponse{}, nil
		},
	}
	w := serveHandler(SetNeedApproval(client), "PUT", "/api/actuaries/:id/need-approval", "/api/actuaries/3/need-approval", `{"need_approval":true}`)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "need_approval updated", resp["message"])
}

func TestSetNeedApproval_SetFalse(t *testing.T) {
	client := &stubEmpClient{
		setNeedApprovalFn: func(_ context.Context, in *pb.SetNeedApprovalRequest, _ ...grpc.CallOption) (*pb.SetNeedApprovalResponse, error) {
			assert.False(t, in.NeedApproval)
			return &pb.SetNeedApprovalResponse{}, nil
		},
	}
	w := serveHandler(SetNeedApproval(client), "PUT", "/api/actuaries/:id/need-approval", "/api/actuaries/1/need-approval", `{"need_approval":false}`)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestResetAgentUsedLimit_GrpcError(t *testing.T) {
	client := &stubEmpClient{
		resetUsedLimitFn: func(_ context.Context, _ *pb.ResetAgentUsedLimitRequest, _ ...grpc.CallOption) (*pb.ResetAgentUsedLimitResponse, error) {
			return nil, status.Error(codes.Internal, "db error")
		},
	}
	w := serveHandler(ResetAgentUsedLimit(client), "POST", "/api/actuaries/:id/reset-used-limit", "/api/actuaries/5/reset-used-limit", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSetNeedApproval_GrpcError(t *testing.T) {
	client := &stubEmpClient{
		setNeedApprovalFn: func(_ context.Context, _ *pb.SetNeedApprovalRequest, _ ...grpc.CallOption) (*pb.SetNeedApprovalResponse, error) {
			return nil, status.Error(codes.Internal, "db error")
		},
	}
	w := serveHandler(SetNeedApproval(client), "PUT", "/api/actuaries/:id/need-approval", "/api/actuaries/1/need-approval", `{"need_approval":true}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSetAgentLimit_GrpcError(t *testing.T) {
	client := &stubEmpClient{
		setAgentLimitFn: func(_ context.Context, _ *pb.SetAgentLimitRequest, _ ...grpc.CallOption) (*pb.SetAgentLimitResponse, error) {
			return nil, status.Error(codes.Internal, "db error")
		},
	}
	w := serveHandler(SetAgentLimit(client), "PUT", "/api/actuaries/:id/limit", "/api/actuaries/1/limit", `{"limit":1000}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestResetAgentUsedLimit_InvalidArgument(t *testing.T) {
	client := &stubEmpClient{
		resetUsedLimitFn: func(_ context.Context, _ *pb.ResetAgentUsedLimitRequest, _ ...grpc.CallOption) (*pb.ResetAgentUsedLimitResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "employee is not an agent")
		},
	}
	w := serveHandler(ResetAgentUsedLimit(client), "POST", "/api/actuaries/:id/reset-used-limit", "/api/actuaries/1/reset-used-limit", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetNeedApproval_InvalidArgument(t *testing.T) {
	client := &stubEmpClient{
		setNeedApprovalFn: func(_ context.Context, _ *pb.SetNeedApprovalRequest, _ ...grpc.CallOption) (*pb.SetNeedApprovalResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "employee is not an agent")
		},
	}
	w := serveHandler(SetNeedApproval(client), "PUT", "/api/actuaries/:id/need-approval", "/api/actuaries/1/need-approval", `{"need_approval":true}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
