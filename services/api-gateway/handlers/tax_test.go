package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	clientpb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/client"
	pb_emp "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/employee"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/portfolio"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

// ── GetMyTax ──────────────────────────────────────────────────────────────────

func TestGetMyTax_NoToken(t *testing.T) {
	w := serveHandlerFull(GetMyTax(&stubPortfolioClient{}, "EMPLOYEE"), "GET", "/tax/my", "/tax/my", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetMyTax_GrpcError(t *testing.T) {
	client := &stubPortfolioClient{
		getMyTaxFn: func(_ context.Context, _ *pb.GetMyTaxRequest, _ ...grpc.CallOption) (*pb.GetMyTaxResponse, error) {
			return nil, fmt.Errorf("portfolio service down")
		},
	}
	w := serveHandlerFull(GetMyTax(client, "EMPLOYEE"), "GET", "/tax/my", "/tax/my", "", makeClientToken())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetMyTax_Happy(t *testing.T) {
	client := &stubPortfolioClient{
		getMyTaxFn: func(_ context.Context, _ *pb.GetMyTaxRequest, _ ...grpc.CallOption) (*pb.GetMyTaxResponse, error) {
			return &pb.GetMyTaxResponse{PaidThisYear: 4500.0, UnpaidThisMonth: 2250.0}, nil
		},
	}
	w := serveHandlerFull(GetMyTax(client, "EMPLOYEE"), "GET", "/tax/my", "/tax/my", "", makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "paidThisYear")
	assert.Contains(t, w.Body.String(), "unpaidThisMonth")
}

// ── GetTaxList ────────────────────────────────────────────────────────────────

func TestGetTaxList_GrpcError(t *testing.T) {
	portfolioClient := &stubPortfolioClient{
		getTaxListFn: func(_ context.Context, _ *pb.GetTaxListRequest, _ ...grpc.CallOption) (*pb.GetTaxListResponse, error) {
			return nil, fmt.Errorf("service down")
		},
	}
	w := serveHandlerFull(
		GetTaxList(portfolioClient, &stubEmpClient{getByIdFn: func(_ context.Context, _ *pb_emp.GetEmployeeByIdRequest, _ ...grpc.CallOption) (*pb_emp.GetEmployeeByIdResponse, error) {
			return nil, fmt.Errorf("not found")
		}}, &stubClientSvcClient{}),
		"GET", "/tax", "/tax", "", makeClientToken(),
	)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetTaxList_NameFilter(t *testing.T) {
	portfolioClient := &stubPortfolioClient{
		getTaxListFn: func(_ context.Context, _ *pb.GetTaxListRequest, _ ...grpc.CallOption) (*pb.GetTaxListResponse, error) {
			return &pb.GetTaxListResponse{
				Entries: []*pb.TaxDebtEntry{
					{UserId: 10, Type: "CLIENT", DebtRsd: 1500.0},
					{UserId: 11, Type: "CLIENT", DebtRsd: 800.0},
				},
			}, nil
		},
	}
	empClient := &stubEmpClient{
		getByIdFn: func(_ context.Context, _ *pb_emp.GetEmployeeByIdRequest, _ ...grpc.CallOption) (*pb_emp.GetEmployeeByIdResponse, error) {
			return nil, fmt.Errorf("not found")
		},
	}
	cliClient := &stubClientSvcClient{
		getByIdFn: func(_ context.Context, req *clientpb.GetClientByIdRequest, _ ...grpc.CallOption) (*clientpb.GetClientByIdResponse, error) {
			names := map[int64]string{10: "Ana Anić", 11: "Bojan Bojić"}
			parts := strings.SplitN(names[req.Id], " ", 2)
			return &clientpb.GetClientByIdResponse{
				Client: &clientpb.Client{FirstName: parts[0], LastName: parts[1]},
			}, nil
		},
	}
	w := serveHandlerFull(GetTaxList(portfolioClient, empClient, cliClient), "GET", "/tax", "/tax?name=ana", "", makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Ana Anić")
	assert.NotContains(t, w.Body.String(), "Bojan")
}

func TestGetTaxList_Happy_ClientEntry(t *testing.T) {
	portfolioClient := &stubPortfolioClient{
		getTaxListFn: func(_ context.Context, _ *pb.GetTaxListRequest, _ ...grpc.CallOption) (*pb.GetTaxListResponse, error) {
			return &pb.GetTaxListResponse{
				Entries: []*pb.TaxDebtEntry{
					{UserId: 10, Type: "CLIENT", DebtRsd: 1500.0},
				},
			}, nil
		},
	}
	empClient := &stubEmpClient{
		getByIdFn: func(_ context.Context, _ *pb_emp.GetEmployeeByIdRequest, _ ...grpc.CallOption) (*pb_emp.GetEmployeeByIdResponse, error) {
			return nil, fmt.Errorf("not found")
		},
	}
	cliClient := &stubClientSvcClient{
		getByIdFn: func(_ context.Context, _ *clientpb.GetClientByIdRequest, _ ...grpc.CallOption) (*clientpb.GetClientByIdResponse, error) {
			return &clientpb.GetClientByIdResponse{
				Client: &clientpb.Client{FirstName: "Ana", LastName: "Anić"},
			}, nil
		},
	}
	w := serveHandlerFull(GetTaxList(portfolioClient, empClient, cliClient), "GET", "/tax", "/tax", "", makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Ana Anić")
	assert.Contains(t, w.Body.String(), "1500")
}

// ── CollectTax ────────────────────────────────────────────────────────────────

func TestCollectTax_GrpcError(t *testing.T) {
	client := &stubPortfolioClient{
		collectTaxFn: func(_ context.Context, _ *pb.CollectTaxRequest, _ ...grpc.CallOption) (*pb.CollectTaxResponse, error) {
			return nil, fmt.Errorf("collection failed")
		},
	}
	w := serveHandlerFull(CollectTax(client), "POST", "/tax/collect", "/tax/collect", "", makeClientToken())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCollectTax_Happy(t *testing.T) {
	client := &stubPortfolioClient{
		collectTaxFn: func(_ context.Context, _ *pb.CollectTaxRequest, _ ...grpc.CallOption) (*pb.CollectTaxResponse, error) {
			return &pb.CollectTaxResponse{}, nil
		},
	}
	w := serveHandlerFull(CollectTax(client), "POST", "/tax/collect", "/tax/collect", "", makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
}

// ── CollectTaxForUser ─────────────────────────────────────────────────────────

func TestCollectTaxForUser_BadID(t *testing.T) {
	w := serveHandlerFull(CollectTaxForUser(&stubPortfolioClient{}), "POST", "/tax/collect/:userId", "/tax/collect/abc", "", makeClientToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCollectTaxForUser_GrpcError(t *testing.T) {
	client := &stubPortfolioClient{
		collectTaxForUserFn: func(_ context.Context, _ *pb.CollectTaxForUserRequest, _ ...grpc.CallOption) (*pb.CollectTaxForUserResponse, error) {
			return nil, fmt.Errorf("user not found")
		},
	}
	w := serveHandlerFull(CollectTaxForUser(client), "POST", "/tax/collect/:userId", "/tax/collect/5", "", makeClientToken())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCollectTaxForUser_Happy(t *testing.T) {
	client := &stubPortfolioClient{
		collectTaxForUserFn: func(_ context.Context, req *pb.CollectTaxForUserRequest, _ ...grpc.CallOption) (*pb.CollectTaxForUserResponse, error) {
			if req.UserId != 5 {
				return nil, fmt.Errorf("wrong user")
			}
			return &pb.CollectTaxForUserResponse{}, nil
		},
	}
	w := serveHandlerFull(CollectTaxForUser(client), "POST", "/tax/collect/:userId", "/tax/collect/5", "", makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
}
