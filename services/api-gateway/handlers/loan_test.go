package handlers

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	loanpb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/loan"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---- stub ----

type stubLoanClient struct {
	getClientLoansFn         func(context.Context, *loanpb.GetClientLoansRequest, ...grpc.CallOption) (*loanpb.GetClientLoansResponse, error)
	getLoanDetailsFn         func(context.Context, *loanpb.GetLoanDetailsRequest, ...grpc.CallOption) (*loanpb.GetLoanDetailsResponse, error)
	getLoanInstallmentsFn    func(context.Context, *loanpb.GetLoanInstallmentsRequest, ...grpc.CallOption) (*loanpb.GetLoanInstallmentsResponse, error)
	submitLoanFn             func(context.Context, *loanpb.SubmitLoanApplicationRequest, ...grpc.CallOption) (*loanpb.SubmitLoanApplicationResponse, error)
	approveLoanFn            func(context.Context, *loanpb.ApproveLoanRequest, ...grpc.CallOption) (*loanpb.ApproveLoanResponse, error)
	rejectLoanFn             func(context.Context, *loanpb.RejectLoanRequest, ...grpc.CallOption) (*loanpb.RejectLoanResponse, error)
	getAllLoanApplicationsFn func(context.Context, *loanpb.GetAllLoanApplicationsRequest, ...grpc.CallOption) (*loanpb.GetAllLoanApplicationsResponse, error)
	getAllLoansFn            func(context.Context, *loanpb.GetAllLoansRequest, ...grpc.CallOption) (*loanpb.GetAllLoansResponse, error)
	triggerInstallmentsFn    func(context.Context, *loanpb.TriggerInstallmentsRequest, ...grpc.CallOption) (*loanpb.TriggerInstallmentsResponse, error)
}

func (s *stubLoanClient) GetClientLoans(ctx context.Context, in *loanpb.GetClientLoansRequest, opts ...grpc.CallOption) (*loanpb.GetClientLoansResponse, error) {
	if s.getClientLoansFn != nil {
		return s.getClientLoansFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubLoanClient) GetLoanDetails(ctx context.Context, in *loanpb.GetLoanDetailsRequest, opts ...grpc.CallOption) (*loanpb.GetLoanDetailsResponse, error) {
	if s.getLoanDetailsFn != nil {
		return s.getLoanDetailsFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubLoanClient) GetLoanInstallments(ctx context.Context, in *loanpb.GetLoanInstallmentsRequest, opts ...grpc.CallOption) (*loanpb.GetLoanInstallmentsResponse, error) {
	if s.getLoanInstallmentsFn != nil {
		return s.getLoanInstallmentsFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubLoanClient) SubmitLoanApplication(ctx context.Context, in *loanpb.SubmitLoanApplicationRequest, opts ...grpc.CallOption) (*loanpb.SubmitLoanApplicationResponse, error) {
	if s.submitLoanFn != nil {
		return s.submitLoanFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubLoanClient) ApproveLoan(ctx context.Context, in *loanpb.ApproveLoanRequest, opts ...grpc.CallOption) (*loanpb.ApproveLoanResponse, error) {
	if s.approveLoanFn != nil {
		return s.approveLoanFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubLoanClient) RejectLoan(ctx context.Context, in *loanpb.RejectLoanRequest, opts ...grpc.CallOption) (*loanpb.RejectLoanResponse, error) {
	if s.rejectLoanFn != nil {
		return s.rejectLoanFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubLoanClient) GetAllLoanApplications(ctx context.Context, in *loanpb.GetAllLoanApplicationsRequest, opts ...grpc.CallOption) (*loanpb.GetAllLoanApplicationsResponse, error) {
	if s.getAllLoanApplicationsFn != nil {
		return s.getAllLoanApplicationsFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubLoanClient) GetAllLoans(ctx context.Context, in *loanpb.GetAllLoansRequest, opts ...grpc.CallOption) (*loanpb.GetAllLoansResponse, error) {
	if s.getAllLoansFn != nil {
		return s.getAllLoansFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubLoanClient) TriggerInstallments(ctx context.Context, in *loanpb.TriggerInstallmentsRequest, opts ...grpc.CallOption) (*loanpb.TriggerInstallmentsResponse, error) {
	if s.triggerInstallmentsFn != nil {
		return s.triggerInstallmentsFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}

// ---- GetMyLoans ----

func TestGetMyLoans_NoToken(t *testing.T) {
	w := serveHandler(GetMyLoans(&stubLoanClient{}), "GET", "/loans", "/loans", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestGetMyLoans_Error(t *testing.T) {
	svc := &stubLoanClient{getClientLoansFn: func(_ context.Context, _ *loanpb.GetClientLoansRequest, _ ...grpc.CallOption) (*loanpb.GetClientLoansResponse, error) {
		return nil, fmt.Errorf("db error")
	}}
	w := serveHandlerFull(GetMyLoans(svc), "GET", "/loans", "/loans", "", makeClientToken())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

func TestGetMyLoans_Happy(t *testing.T) {
	svc := &stubLoanClient{getClientLoansFn: func(_ context.Context, _ *loanpb.GetClientLoansRequest, _ ...grpc.CallOption) (*loanpb.GetClientLoansResponse, error) {
		return &loanpb.GetClientLoansResponse{Loans: []*loanpb.LoanSummary{{Id: 1, LoanType: "CASH"}}}, nil
	}}
	w := serveHandlerFull(GetMyLoans(svc), "GET", "/loans", "/loans", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

// ---- GetLoanDetails ----

func TestGetLoanDetails_NoToken(t *testing.T) {
	w := serveHandler(GetLoanDetails(&stubLoanClient{}), "GET", "/loans/:id", "/loans/1", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestGetLoanDetails_BadID(t *testing.T) {
	w := serveHandlerFull(GetLoanDetails(&stubLoanClient{}), "GET", "/loans/:id", "/loans/abc", "", makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestGetLoanDetails_NotFound(t *testing.T) {
	svc := &stubLoanClient{getLoanDetailsFn: func(_ context.Context, _ *loanpb.GetLoanDetailsRequest, _ ...grpc.CallOption) (*loanpb.GetLoanDetailsResponse, error) {
		return nil, status.Error(codes.NotFound, "not found")
	}}
	w := serveHandlerFull(GetLoanDetails(svc), "GET", "/loans/:id", "/loans/1", "", makeClientToken())
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

func TestGetLoanDetails_Error(t *testing.T) {
	svc := &stubLoanClient{getLoanDetailsFn: func(_ context.Context, _ *loanpb.GetLoanDetailsRequest, _ ...grpc.CallOption) (*loanpb.GetLoanDetailsResponse, error) {
		return nil, fmt.Errorf("internal")
	}}
	w := serveHandlerFull(GetLoanDetails(svc), "GET", "/loans/:id", "/loans/1", "", makeClientToken())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

func TestGetLoanDetails_Happy(t *testing.T) {
	svc := &stubLoanClient{getLoanDetailsFn: func(_ context.Context, _ *loanpb.GetLoanDetailsRequest, _ ...grpc.CallOption) (*loanpb.GetLoanDetailsResponse, error) {
		return &loanpb.GetLoanDetailsResponse{
			Loan:         &loanpb.LoanDetail{Id: 1, LoanType: "CASH"},
			Installments: []*loanpb.Installment{{Id: 1, Status: "PENDING"}},
		}, nil
	}}
	w := serveHandlerFull(GetLoanDetails(svc), "GET", "/loans/:id", "/loans/1", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

// ---- GetLoanInstallments ----

func TestGetLoanInstallments_NoToken(t *testing.T) {
	w := serveHandler(GetLoanInstallments(&stubLoanClient{}), "GET", "/loans/:id/installments", "/loans/1/installments", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestGetLoanInstallments_BadID(t *testing.T) {
	w := serveHandlerFull(GetLoanInstallments(&stubLoanClient{}), "GET", "/loans/:id/installments", "/loans/abc/installments", "", makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestGetLoanInstallments_Error(t *testing.T) {
	svc := &stubLoanClient{getLoanInstallmentsFn: func(_ context.Context, _ *loanpb.GetLoanInstallmentsRequest, _ ...grpc.CallOption) (*loanpb.GetLoanInstallmentsResponse, error) {
		return nil, fmt.Errorf("db error")
	}}
	w := serveHandlerFull(GetLoanInstallments(svc), "GET", "/loans/:id/installments", "/loans/1/installments", "", makeClientToken())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

func TestGetLoanInstallments_Happy(t *testing.T) {
	svc := &stubLoanClient{getLoanInstallmentsFn: func(_ context.Context, _ *loanpb.GetLoanInstallmentsRequest, _ ...grpc.CallOption) (*loanpb.GetLoanInstallmentsResponse, error) {
		return &loanpb.GetLoanInstallmentsResponse{Installments: []*loanpb.Installment{{Id: 1}}}, nil
	}}
	w := serveHandlerFull(GetLoanInstallments(svc), "GET", "/loans/:id/installments", "/loans/1/installments", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

// ---- ApplyForLoan ----

func TestApplyForLoan_NoToken(t *testing.T) {
	body := `{"loanType":"CASH","interestRateType":"FIXED","amount":1000,"currency":"RSD","repaymentPeriod":12,"accountNumber":"acc"}`
	w := serveHandler(ApplyForLoan(&stubLoanClient{}), "POST", "/loans/apply", "/loans/apply", body)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestApplyForLoan_BadJSON(t *testing.T) {
	w := serveHandlerFull(ApplyForLoan(&stubLoanClient{}), "POST", "/loans/apply", "/loans/apply", `{bad}`, makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestApplyForLoan_InvalidArgument(t *testing.T) {
	svc := &stubLoanClient{submitLoanFn: func(_ context.Context, _ *loanpb.SubmitLoanApplicationRequest, _ ...grpc.CallOption) (*loanpb.SubmitLoanApplicationResponse, error) {
		return nil, status.Error(codes.InvalidArgument, "invalid loan type")
	}}
	body := `{"loanType":"CASH","interestRateType":"FIXED","amount":1000,"currency":"RSD","repaymentPeriod":12,"accountNumber":"acc"}`
	w := serveHandlerFull(ApplyForLoan(svc), "POST", "/loans/apply", "/loans/apply", body, makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestApplyForLoan_Error(t *testing.T) {
	svc := &stubLoanClient{submitLoanFn: func(_ context.Context, _ *loanpb.SubmitLoanApplicationRequest, _ ...grpc.CallOption) (*loanpb.SubmitLoanApplicationResponse, error) {
		return nil, fmt.Errorf("internal")
	}}
	body := `{"loanType":"CASH","interestRateType":"FIXED","amount":1000,"currency":"RSD","repaymentPeriod":12,"accountNumber":"acc"}`
	w := serveHandlerFull(ApplyForLoan(svc), "POST", "/loans/apply", "/loans/apply", body, makeClientToken())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

func TestApplyForLoan_Happy(t *testing.T) {
	svc := &stubLoanClient{submitLoanFn: func(_ context.Context, _ *loanpb.SubmitLoanApplicationRequest, _ ...grpc.CallOption) (*loanpb.SubmitLoanApplicationResponse, error) {
		return &loanpb.SubmitLoanApplicationResponse{LoanId: 1, LoanNumber: 100, MonthlyInstallment: 50}, nil
	}}
	body := `{"loanType":"CASH","interestRateType":"FIXED","amount":1000,"currency":"RSD","repaymentPeriod":12,"accountNumber":"acc"}`
	w := serveHandlerFull(ApplyForLoan(svc), "POST", "/loans/apply", "/loans/apply", body, makeClientToken())
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d", w.Code)
	}
}

// ---- GetAllLoanApplications ----

func TestGetAllLoanApplications_Error(t *testing.T) {
	svc := &stubLoanClient{getAllLoanApplicationsFn: func(_ context.Context, _ *loanpb.GetAllLoanApplicationsRequest, _ ...grpc.CallOption) (*loanpb.GetAllLoanApplicationsResponse, error) {
		return nil, fmt.Errorf("db error")
	}}
	w := serveHandler(GetAllLoanApplications(svc), "GET", "/admin/loans/applications", "/admin/loans/applications", "")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

func TestGetAllLoanApplications_Happy(t *testing.T) {
	svc := &stubLoanClient{getAllLoanApplicationsFn: func(_ context.Context, _ *loanpb.GetAllLoanApplicationsRequest, _ ...grpc.CallOption) (*loanpb.GetAllLoanApplicationsResponse, error) {
		return &loanpb.GetAllLoanApplicationsResponse{Applications: []*loanpb.LoanDetail{{Id: 1}}}, nil
	}}
	w := serveHandler(GetAllLoanApplications(svc), "GET", "/admin/loans/applications", "/admin/loans/applications?loanType=CASH", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

// ---- ApproveLoan ----

func TestApproveLoan_NoToken(t *testing.T) {
	w := serveHandler(ApproveLoan(&stubLoanClient{}), "PUT", "/admin/loans/:id/approve", "/admin/loans/1/approve", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestApproveLoan_BadID(t *testing.T) {
	w := serveHandlerFull(ApproveLoan(&stubLoanClient{}), "PUT", "/admin/loans/:id/approve", "/admin/loans/abc/approve", "", makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestApproveLoan_NotFound(t *testing.T) {
	svc := &stubLoanClient{approveLoanFn: func(_ context.Context, _ *loanpb.ApproveLoanRequest, _ ...grpc.CallOption) (*loanpb.ApproveLoanResponse, error) {
		return nil, status.Error(codes.NotFound, "not found")
	}}
	w := serveHandlerFull(ApproveLoan(svc), "PUT", "/admin/loans/:id/approve", "/admin/loans/1/approve", "", makeClientToken())
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

func TestApproveLoan_InvalidArgument(t *testing.T) {
	svc := &stubLoanClient{approveLoanFn: func(_ context.Context, _ *loanpb.ApproveLoanRequest, _ ...grpc.CallOption) (*loanpb.ApproveLoanResponse, error) {
		return nil, status.Error(codes.InvalidArgument, "already approved")
	}}
	w := serveHandlerFull(ApproveLoan(svc), "PUT", "/admin/loans/:id/approve", "/admin/loans/1/approve", "", makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestApproveLoan_Error(t *testing.T) {
	svc := &stubLoanClient{approveLoanFn: func(_ context.Context, _ *loanpb.ApproveLoanRequest, _ ...grpc.CallOption) (*loanpb.ApproveLoanResponse, error) {
		return nil, fmt.Errorf("internal")
	}}
	w := serveHandlerFull(ApproveLoan(svc), "PUT", "/admin/loans/:id/approve", "/admin/loans/1/approve", "", makeClientToken())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

func TestApproveLoan_Happy(t *testing.T) {
	svc := &stubLoanClient{approveLoanFn: func(_ context.Context, _ *loanpb.ApproveLoanRequest, _ ...grpc.CallOption) (*loanpb.ApproveLoanResponse, error) {
		return &loanpb.ApproveLoanResponse{}, nil
	}}
	w := serveHandlerFull(ApproveLoan(svc), "PUT", "/admin/loans/:id/approve", "/admin/loans/1/approve", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

// ---- RejectLoan ----

func TestRejectLoan_NoToken(t *testing.T) {
	w := serveHandler(RejectLoan(&stubLoanClient{}), "PUT", "/admin/loans/:id/reject", "/admin/loans/1/reject", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestRejectLoan_BadID(t *testing.T) {
	w := serveHandlerFull(RejectLoan(&stubLoanClient{}), "PUT", "/admin/loans/:id/reject", "/admin/loans/abc/reject", "", makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestRejectLoan_NotFound(t *testing.T) {
	svc := &stubLoanClient{rejectLoanFn: func(_ context.Context, _ *loanpb.RejectLoanRequest, _ ...grpc.CallOption) (*loanpb.RejectLoanResponse, error) {
		return nil, status.Error(codes.NotFound, "not found")
	}}
	w := serveHandlerFull(RejectLoan(svc), "PUT", "/admin/loans/:id/reject", "/admin/loans/1/reject", "", makeClientToken())
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

func TestRejectLoan_InvalidArgument(t *testing.T) {
	svc := &stubLoanClient{rejectLoanFn: func(_ context.Context, _ *loanpb.RejectLoanRequest, _ ...grpc.CallOption) (*loanpb.RejectLoanResponse, error) {
		return nil, status.Error(codes.InvalidArgument, "already rejected")
	}}
	w := serveHandlerFull(RejectLoan(svc), "PUT", "/admin/loans/:id/reject", "/admin/loans/1/reject", "", makeClientToken())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestRejectLoan_Error(t *testing.T) {
	svc := &stubLoanClient{rejectLoanFn: func(_ context.Context, _ *loanpb.RejectLoanRequest, _ ...grpc.CallOption) (*loanpb.RejectLoanResponse, error) {
		return nil, fmt.Errorf("internal")
	}}
	w := serveHandlerFull(RejectLoan(svc), "PUT", "/admin/loans/:id/reject", "/admin/loans/1/reject", "", makeClientToken())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

func TestRejectLoan_Happy(t *testing.T) {
	svc := &stubLoanClient{rejectLoanFn: func(_ context.Context, _ *loanpb.RejectLoanRequest, _ ...grpc.CallOption) (*loanpb.RejectLoanResponse, error) {
		return &loanpb.RejectLoanResponse{}, nil
	}}
	w := serveHandlerFull(RejectLoan(svc), "PUT", "/admin/loans/:id/reject", "/admin/loans/1/reject", "", makeClientToken())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

// ---- GetAllLoans ----

func TestGetAllLoans_Error(t *testing.T) {
	svc := &stubLoanClient{getAllLoansFn: func(_ context.Context, _ *loanpb.GetAllLoansRequest, _ ...grpc.CallOption) (*loanpb.GetAllLoansResponse, error) {
		return nil, fmt.Errorf("db error")
	}}
	w := serveHandler(GetAllLoans(svc), "GET", "/admin/loans", "/admin/loans", "")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

func TestGetAllLoans_Happy(t *testing.T) {
	svc := &stubLoanClient{getAllLoansFn: func(_ context.Context, _ *loanpb.GetAllLoansRequest, _ ...grpc.CallOption) (*loanpb.GetAllLoansResponse, error) {
		return &loanpb.GetAllLoansResponse{Loans: []*loanpb.LoanDetail{{Id: 1}}}, nil
	}}
	w := serveHandler(GetAllLoans(svc), "GET", "/admin/loans", "/admin/loans?loanType=CASH&status=ACTIVE", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

// ---- loanDetailsToJSON ----

func TestLoanDetailsToJSON(t *testing.T) {
	loans := []*loanpb.LoanDetail{{Id: 1, LoanType: "CASH", Amount: 1000}}
	result := loanDetailsToJSON(loans)
	if len(result) != 1 {
		t.Fatalf("expected 1 got %d", len(result))
	}
	if result[0]["id"] != int64(1) {
		t.Fatalf("unexpected id: %v", result[0]["id"])
	}
}

// ---- TriggerInstallments ----

func TestTriggerInstallments_Error(t *testing.T) {
	svc := &stubLoanClient{triggerInstallmentsFn: func(_ context.Context, _ *loanpb.TriggerInstallmentsRequest, _ ...grpc.CallOption) (*loanpb.TriggerInstallmentsResponse, error) {
		return nil, fmt.Errorf("internal")
	}}
	w := serveHandler(TriggerInstallments(svc), "POST", "/admin/loans/trigger", "/admin/loans/trigger", "")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

func TestTriggerInstallments_Happy(t *testing.T) {
	svc := &stubLoanClient{triggerInstallmentsFn: func(_ context.Context, _ *loanpb.TriggerInstallmentsRequest, _ ...grpc.CallOption) (*loanpb.TriggerInstallmentsResponse, error) {
		return &loanpb.TriggerInstallmentsResponse{Processed: 5}, nil
	}}
	w := serveHandler(TriggerInstallments(svc), "POST", "/admin/loans/trigger", "/admin/loans/trigger", `{"forceLoanId":1}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}
