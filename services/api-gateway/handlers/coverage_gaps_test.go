package handlers

// Extra tests to cover remaining uncovered branches.

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	accountpb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/account"
	cardpb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/card"
	pb_emp "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/employee"
	pb_order "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/order"
	pb_portfolio "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/portfolio"
	pb_sec "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/securities"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---- DeleteAccount: bad ID ----

func TestDeleteAccount_BadID(t *testing.T) {
	// parseID returns error without writing response; gin defaults to 200 empty body
	w := serveHandler(DeleteAccount(&stubAccountClient{}), "DELETE", "/admin/accounts/:accountId", "/admin/accounts/abc", "")
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty body got %s", w.Body.String())
	}
}

// ---- CreateAccount: limit error (log-only, still 201) ----

func TestCreateAccount_LimitError(t *testing.T) {
	svc := &stubAccountClient{
		createFn: func(_ context.Context, _ *accountpb.CreateAccountRequest, _ ...grpc.CallOption) (*accountpb.CreateAccountResponse, error) {
			return &accountpb.CreateAccountResponse{Account: &accountpb.AccountResponse{Id: 1, AccountNumber: "ACC001"}}, nil
		},
		updateLimitsFn: func(_ context.Context, _ *accountpb.UpdateAccountLimitsRequest, _ ...grpc.CallOption) (*accountpb.UpdateAccountLimitsResponse, error) {
			return nil, fmt.Errorf("limits service down")
		},
	}
	body := `{"clientId":1,"accountType":"CURRENT","currencyCode":"RSD","dailyLimit":1000,"monthlyLimit":5000}`
	w := serveHandlerFull(CreateAccount(svc, &stubCardClient{}), "POST", "/admin/accounts", "/admin/accounts", body, makeClientToken())
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d (limit error is non-fatal)", w.Code)
	}
}

// ---- CreateAccount: card creation error (log-only, still 201) ----

func TestCreateAccount_CardCreationError(t *testing.T) {
	svc := &stubAccountClient{
		createFn: func(_ context.Context, _ *accountpb.CreateAccountRequest, _ ...grpc.CallOption) (*accountpb.CreateAccountResponse, error) {
			return &accountpb.CreateAccountResponse{Account: &accountpb.AccountResponse{Id: 1, AccountNumber: "ACC001"}}, nil
		},
	}
	cardSvc := &stubCardClient{
		createCardFn: func(_ context.Context, _ *cardpb.CreateCardRequest, _ ...grpc.CallOption) (*cardpb.CreateCardResponse, error) {
			return nil, fmt.Errorf("card service down")
		},
	}
	body := `{"clientId":1,"accountType":"CURRENT","currencyCode":"RSD","createCard":true}`
	w := serveHandlerFull(CreateAccount(svc, cardSvc), "POST", "/admin/accounts", "/admin/accounts", body, makeClientToken())
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d (card error is non-fatal)", w.Code)
	}
}

// ---- CreateAccount: card limit update error (log-only, still 201) ----

func TestCreateAccount_CardLimitError(t *testing.T) {
	svc := &stubAccountClient{
		createFn: func(_ context.Context, _ *accountpb.CreateAccountRequest, _ ...grpc.CallOption) (*accountpb.CreateAccountResponse, error) {
			return &accountpb.CreateAccountResponse{Account: &accountpb.AccountResponse{Id: 1, AccountNumber: "ACC001"}}, nil
		},
	}
	cardSvc := &stubCardClient{
		createCardFn: func(_ context.Context, _ *cardpb.CreateCardRequest, _ ...grpc.CallOption) (*cardpb.CreateCardResponse, error) {
			return &cardpb.CreateCardResponse{Card: &cardpb.CardResponse{CardNumber: "4111111111111111"}}, nil
		},
		updateCardLimitFn: func(_ context.Context, _ *cardpb.UpdateCardLimitRequest, _ ...grpc.CallOption) (*cardpb.UpdateCardLimitResponse, error) {
			return nil, fmt.Errorf("limit service down")
		},
	}
	body := `{"clientId":1,"accountType":"CURRENT","currencyCode":"RSD","createCard":true,"cardLimit":500}`
	w := serveHandlerFull(CreateAccount(svc, cardSvc), "POST", "/admin/accounts", "/admin/accounts", body, makeClientToken())
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d (card limit error is non-fatal)", w.Code)
	}
}

// ---- UpdateCardLimit: resolve returns NotFound ----

func TestUpdateCardLimit_ResolveNotFound(t *testing.T) {
	svc := &stubCardClient{
		getCardByIdFn: func(_ context.Context, _ *cardpb.GetCardByIdRequest, _ ...grpc.CallOption) (*cardpb.GetCardByIdResponse, error) {
			return nil, status.Error(codes.NotFound, "card not found")
		},
	}
	w := serveHandler(UpdateCardLimit(svc), "PUT", "/cards/:id/limit", "/cards/1/limit", `{"newLimit":100}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

// ---- Logout: auth.go:364 ----

func TestLogout_Happy(t *testing.T) {
	w := serveHandler(Logout(&stubAuthClient{}), "POST", "/auth/logout", "/auth/logout", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

// ---- SetWorkingHours: securities.go:321 ----

func TestSetWorkingHours_Happy(t *testing.T) {
	client := &stubSecuritiesClient{
		setHoursFn: func(_ context.Context, _ *pb_sec.SetWorkingHoursRequest, _ ...grpc.CallOption) (*pb_sec.SetWorkingHoursResponse, error) {
			return &pb_sec.SetWorkingHoursResponse{
				Hours: &pb_sec.ExchangeWorkingHours{
					Id: 1, Polity: "United States", Segment: "regular", OpenTime: "09:30", CloseTime: "16:00",
				},
			}, nil
		},
	}
	body := `{"polity":"United States","segment":"regular","openTime":"09:30","closeTime":"16:00"}`
	w := serveHandler(SetWorkingHours(client), "POST", "/stock-exchanges/hours", "/stock-exchanges/hours", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetWorkingHours_BadBody(t *testing.T) {
	w := serveHandler(SetWorkingHours(&stubSecuritiesClient{}), "POST", "/stock-exchanges/hours", "/stock-exchanges/hours", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

// ---- GetHolidays: securities.go:371 ----
// resolveExchangeMIC calls getByMICFn once, then GetHolidays calls it again for polity.

func TestGetHolidays_Happy(t *testing.T) {
	client := &stubSecuritiesClient{
		getByMICFn: func(_ context.Context, _ *pb_sec.GetStockExchangeByMICRequest, _ ...grpc.CallOption) (*pb_sec.GetStockExchangeByMICResponse, error) {
			return &pb_sec.GetStockExchangeByMICResponse{Exchange: &pb_sec.StockExchange{
				Id: 1, MicCode: "XNAS", Polity: "United States",
			}}, nil
		},
		getHolidaysFn: func(_ context.Context, _ *pb_sec.GetHolidaysRequest, _ ...grpc.CallOption) (*pb_sec.GetHolidaysResponse, error) {
			return &pb_sec.GetHolidaysResponse{Holidays: []*pb_sec.ExchangeHoliday{
				{Id: 1, Polity: "United States", HolidayDate: "2025-07-04", Description: "Independence Day"},
			}}, nil
		},
	}
	w := serveHandler(GetHolidays(client), "GET", "/stock-exchanges/:id/holidays", "/stock-exchanges/XNAS/holidays", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Independence Day") {
		t.Errorf("expected holiday in response, got: %s", w.Body.String())
	}
}

// ---- AddHoliday: securities.go:411 ----

func TestAddHoliday_Happy(t *testing.T) {
	client := &stubSecuritiesClient{
		addHolidayFn: func(_ context.Context, _ *pb_sec.AddHolidayRequest, _ ...grpc.CallOption) (*pb_sec.AddHolidayResponse, error) {
			return &pb_sec.AddHolidayResponse{Holiday: &pb_sec.ExchangeHoliday{
				Id: 1, Polity: "United States", HolidayDate: "2025-07-04", Description: "Independence Day",
			}}, nil
		},
	}
	body := `{"polity":"United States","holidayDate":"2025-07-04","description":"Independence Day"}`
	w := serveHandler(AddHoliday(client), "POST", "/stock-exchanges/holidays", "/stock-exchanges/holidays", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddHoliday_BadBody(t *testing.T) {
	w := serveHandler(AddHoliday(&stubSecuritiesClient{}), "POST", "/stock-exchanges/holidays", "/stock-exchanges/holidays", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

// ---- DeleteHoliday: securities.go:449 ----

func TestDeleteHoliday_Happy(t *testing.T) {
	client := &stubSecuritiesClient{
		deleteHolidayFn: func(_ context.Context, _ *pb_sec.DeleteHolidayRequest, _ ...grpc.CallOption) (*pb_sec.DeleteHolidayResponse, error) {
			return &pb_sec.DeleteHolidayResponse{}, nil
		},
	}
	w := serveHandler(DeleteHoliday(client), "DELETE", "/stock-exchanges/holidays/:polity/:date", "/stock-exchanges/holidays/US/2025-07-04", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 got %d", w.Code)
	}
}

// ---- IsExchangeOpen: securities.go:526 ----

func TestIsExchangeOpen_Happy(t *testing.T) {
	client := &stubSecuritiesClient{
		getByMICFn: func(_ context.Context, _ *pb_sec.GetStockExchangeByMICRequest, _ ...grpc.CallOption) (*pb_sec.GetStockExchangeByMICResponse, error) {
			return &pb_sec.GetStockExchangeByMICResponse{Exchange: &pb_sec.StockExchange{
				MicCode: "XNAS",
			}}, nil
		},
		isOpenFn: func(_ context.Context, _ *pb_sec.IsExchangeOpenRequest, _ ...grpc.CallOption) (*pb_sec.IsExchangeOpenResponse, error) {
			return &pb_sec.IsExchangeOpenResponse{
				MicCode: "XNAS", IsOpen: true, Segment: "regular", CurrentTimeLocal: "09:45",
			}, nil
		},
	}
	w := serveHandler(IsExchangeOpen(client), "GET", "/stock-exchanges/:id/is-open", "/stock-exchanges/XNAS/is-open", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"isOpen":true`) {
		t.Errorf("expected isOpen in response, got: %s", w.Body.String())
	}
}

// ---- ListOrders enrichment: orders.go:165-222 ----
// Covers agent-name lookup (empClient) and ticker lookup (secClient) branches.

func TestListOrders_Enrichment(t *testing.T) {
	orderClient := &stubOrderClient{
		listOrdersFn: func(_ context.Context, _ *pb_order.ListOrdersRequest, _ ...grpc.CallOption) (*pb_order.ListOrdersResponse, error) {
			return &pb_order.ListOrdersResponse{Orders: []*pb_order.Order{
				{Id: 1, UserId: 5, AssetId: 10, Status: "PENDING", Direction: "BUY"},
			}}, nil
		},
	}
	empClient := &stubEmpClient{
		getByIdFn: func(_ context.Context, _ *pb_emp.GetEmployeeByIdRequest, _ ...grpc.CallOption) (*pb_emp.GetEmployeeByIdResponse, error) {
			return &pb_emp.GetEmployeeByIdResponse{Employee: &pb_emp.Employee{FirstName: "Ana", LastName: "Anić"}}, nil
		},
	}
	secClient := &stubSecuritiesClient{
		getListingByIdFn: func(_ context.Context, _ *pb_sec.GetListingByIdRequest, _ ...grpc.CallOption) (*pb_sec.GetListingByIdResponse, error) {
			return &pb_sec.GetListingByIdResponse{Summary: &pb_sec.ListingSummary{Ticker: "MSFT"}}, nil
		},
	}
	w := serveHandler(ListOrders(orderClient, empClient, secClient), "GET", "/orders", "/orders", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "Ana") {
		t.Errorf("expected agent name in response, got: %s", body)
	}
	if !strings.Contains(body, "MSFT") {
		t.Errorf("expected ticker in response, got: %s", body)
	}
}

// ---- resolveName: EMPLOYEE branch (tax.go:139-143) ----

func TestGetTaxList_EmployeeEntry(t *testing.T) {
	portfolioClient := &stubPortfolioClient{
		getTaxListFn: func(_ context.Context, _ *pb_portfolio.GetTaxListRequest, _ ...grpc.CallOption) (*pb_portfolio.GetTaxListResponse, error) {
			return &pb_portfolio.GetTaxListResponse{
				Entries: []*pb_portfolio.TaxDebtEntry{
					{UserId: 1, Type: "EMPLOYEE", DebtRsd: 500.0},
				},
			}, nil
		},
	}
	empClient := &stubEmpClient{
		getByIdFn: func(_ context.Context, _ *pb_emp.GetEmployeeByIdRequest, _ ...grpc.CallOption) (*pb_emp.GetEmployeeByIdResponse, error) {
			return &pb_emp.GetEmployeeByIdResponse{Employee: &pb_emp.Employee{FirstName: "Dragan", LastName: "Dragić"}}, nil
		},
	}
	w := serveHandlerFull(
		GetTaxList(portfolioClient, empClient, &stubClientSvcClient{}),
		"GET", "/tax", "/tax", "", makeClientToken(),
	)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Dragan") {
		t.Errorf("expected employee name in response, got: %s", w.Body.String())
	}
}

// ---- resolveName: return "" (tax.go:150) — CLIENT lookup fails ----

func TestGetTaxList_ResolveNameFails(t *testing.T) {
	portfolioClient := &stubPortfolioClient{
		getTaxListFn: func(_ context.Context, _ *pb_portfolio.GetTaxListRequest, _ ...grpc.CallOption) (*pb_portfolio.GetTaxListResponse, error) {
			return &pb_portfolio.GetTaxListResponse{
				Entries: []*pb_portfolio.TaxDebtEntry{
					{UserId: 2, Type: "CLIENT", DebtRsd: 200.0},
				},
			}, nil
		},
	}
	// stubClientSvcClient with nil getByIdFn returns error → resolveName returns ""
	w := serveHandlerFull(
		GetTaxList(portfolioClient, &stubEmployeeClient{}, &stubClientSvcClient{}),
		"GET", "/tax", "/tax", "", makeClientToken(),
	)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", w.Code, w.Body.String())
	}
}
