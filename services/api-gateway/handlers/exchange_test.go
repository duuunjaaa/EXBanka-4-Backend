package handlers

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	exchangepb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/exchange"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---- stub exchange service client ----

type stubExchangeClient struct {
	getRatesFn func(context.Context, *exchangepb.GetExchangeRatesRequest, ...grpc.CallOption) (*exchangepb.GetExchangeRatesResponse, error)
	convertFn  func(context.Context, *exchangepb.ConvertAmountRequest, ...grpc.CallOption) (*exchangepb.ConvertAmountResponse, error)
	historyFn  func(context.Context, *exchangepb.GetExchangeHistoryRequest, ...grpc.CallOption) (*exchangepb.GetExchangeHistoryResponse, error)
	previewFn  func(context.Context, *exchangepb.PreviewConversionRequest, ...grpc.CallOption) (*exchangepb.PreviewConversionResponse, error)
}

func (s *stubExchangeClient) GetExchangeRates(ctx context.Context, in *exchangepb.GetExchangeRatesRequest, opts ...grpc.CallOption) (*exchangepb.GetExchangeRatesResponse, error) {
	if s.getRatesFn != nil {
		return s.getRatesFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubExchangeClient) ConvertAmount(ctx context.Context, in *exchangepb.ConvertAmountRequest, opts ...grpc.CallOption) (*exchangepb.ConvertAmountResponse, error) {
	if s.convertFn != nil {
		return s.convertFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubExchangeClient) GetExchangeHistory(ctx context.Context, in *exchangepb.GetExchangeHistoryRequest, opts ...grpc.CallOption) (*exchangepb.GetExchangeHistoryResponse, error) {
	if s.historyFn != nil {
		return s.historyFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *stubExchangeClient) PreviewConversion(ctx context.Context, in *exchangepb.PreviewConversionRequest, opts ...grpc.CallOption) (*exchangepb.PreviewConversionResponse, error) {
	if s.previewFn != nil {
		return s.previewFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}

// ---- helper: sample rates ----

func sampleRates() []*exchangepb.ExchangeRate {
	return []*exchangepb.ExchangeRate{
		{CurrencyCode: "EUR", BuyingRate: 117.0, SellingRate: 119.0, MiddleRate: 118.0},
		{CurrencyCode: "USD", BuyingRate: 108.0, SellingRate: 110.0, MiddleRate: 109.0},
	}
}

// ---- GetExchangeRates ----

func TestGetExchangeRates_NoToken(t *testing.T) {
	svc := &stubExchangeClient{}
	w := serveHandlerFull(GetExchangeRates(svc), "GET", "/exchange/rates", "/exchange/rates", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetExchangeRates_Error(t *testing.T) {
	svc := &stubExchangeClient{
		getRatesFn: func(ctx context.Context, in *exchangepb.GetExchangeRatesRequest, opts ...grpc.CallOption) (*exchangepb.GetExchangeRatesResponse, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	w := serveHandlerFull(GetExchangeRates(svc), "GET", "/exchange/rates", "/exchange/rates", "", makeClientToken())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetExchangeRates_Happy(t *testing.T) {
	svc := &stubExchangeClient{
		getRatesFn: func(ctx context.Context, in *exchangepb.GetExchangeRatesRequest, opts ...grpc.CallOption) (*exchangepb.GetExchangeRatesResponse, error) {
			return &exchangepb.GetExchangeRatesResponse{Rates: sampleRates()}, nil
		},
	}
	w := serveHandlerFull(GetExchangeRates(svc), "GET", "/exchange/rates", "/exchange/rates", "", makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- GetExchangeRate ----

func TestGetExchangeRate_NoToken(t *testing.T) {
	svc := &stubExchangeClient{}
	w := serveHandlerFull(GetExchangeRate(svc), "GET", "/exchange/rate", "/exchange/rate?from=EUR&to=USD", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetExchangeRate_MissingParams(t *testing.T) {
	svc := &stubExchangeClient{}
	w := serveHandlerFull(GetExchangeRate(svc), "GET", "/exchange/rate", "/exchange/rate?from=EUR", "", makeClientToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetExchangeRate_FetchError(t *testing.T) {
	svc := &stubExchangeClient{
		getRatesFn: func(ctx context.Context, in *exchangepb.GetExchangeRatesRequest, opts ...grpc.CallOption) (*exchangepb.GetExchangeRatesResponse, error) {
			return nil, fmt.Errorf("error")
		},
	}
	w := serveHandlerFull(GetExchangeRate(svc), "GET", "/exchange/rate", "/exchange/rate?from=EUR&to=USD", "", makeClientToken())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetExchangeRate_RSDtoEUR(t *testing.T) {
	svc := &stubExchangeClient{
		getRatesFn: func(ctx context.Context, in *exchangepb.GetExchangeRatesRequest, opts ...grpc.CallOption) (*exchangepb.GetExchangeRatesResponse, error) {
			return &exchangepb.GetExchangeRatesResponse{Rates: sampleRates()}, nil
		},
	}
	w := serveHandlerFull(GetExchangeRate(svc), "GET", "/exchange/rate", "/exchange/rate?from=RSD&to=EUR", "", makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetExchangeRate_RSDtoUnknown(t *testing.T) {
	svc := &stubExchangeClient{
		getRatesFn: func(ctx context.Context, in *exchangepb.GetExchangeRatesRequest, opts ...grpc.CallOption) (*exchangepb.GetExchangeRatesResponse, error) {
			return &exchangepb.GetExchangeRatesResponse{Rates: sampleRates()}, nil
		},
	}
	w := serveHandlerFull(GetExchangeRate(svc), "GET", "/exchange/rate", "/exchange/rate?from=RSD&to=XXX", "", makeClientToken())
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetExchangeRate_EURtoRSD(t *testing.T) {
	svc := &stubExchangeClient{
		getRatesFn: func(ctx context.Context, in *exchangepb.GetExchangeRatesRequest, opts ...grpc.CallOption) (*exchangepb.GetExchangeRatesResponse, error) {
			return &exchangepb.GetExchangeRatesResponse{Rates: sampleRates()}, nil
		},
	}
	w := serveHandlerFull(GetExchangeRate(svc), "GET", "/exchange/rate", "/exchange/rate?from=EUR&to=RSD", "", makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetExchangeRate_UnknownToRSD(t *testing.T) {
	svc := &stubExchangeClient{
		getRatesFn: func(ctx context.Context, in *exchangepb.GetExchangeRatesRequest, opts ...grpc.CallOption) (*exchangepb.GetExchangeRatesResponse, error) {
			return &exchangepb.GetExchangeRatesResponse{Rates: sampleRates()}, nil
		},
	}
	w := serveHandlerFull(GetExchangeRate(svc), "GET", "/exchange/rate", "/exchange/rate?from=XXX&to=RSD", "", makeClientToken())
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetExchangeRate_EURtoUSD(t *testing.T) {
	svc := &stubExchangeClient{
		getRatesFn: func(ctx context.Context, in *exchangepb.GetExchangeRatesRequest, opts ...grpc.CallOption) (*exchangepb.GetExchangeRatesResponse, error) {
			return &exchangepb.GetExchangeRatesResponse{Rates: sampleRates()}, nil
		},
	}
	w := serveHandlerFull(GetExchangeRate(svc), "GET", "/exchange/rate", "/exchange/rate?from=EUR&to=USD", "", makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetExchangeRate_UnknownCross(t *testing.T) {
	svc := &stubExchangeClient{
		getRatesFn: func(ctx context.Context, in *exchangepb.GetExchangeRatesRequest, opts ...grpc.CallOption) (*exchangepb.GetExchangeRatesResponse, error) {
			return &exchangepb.GetExchangeRatesResponse{Rates: sampleRates()}, nil
		},
	}
	w := serveHandlerFull(GetExchangeRate(svc), "GET", "/exchange/rate", "/exchange/rate?from=XXX&to=YYY", "", makeClientToken())
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---- ConvertAmount ----

func TestConvertAmount_BadJSON(t *testing.T) {
	svc := &stubExchangeClient{}
	w := serveHandlerFull(ConvertAmount(svc), "POST", "/exchange/convert", "/exchange/convert", `{bad}`, makeClientToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestConvertAmount_NoToken(t *testing.T) {
	svc := &stubExchangeClient{}
	body := `{"fromAccount":"A","toAccount":"B","amount":100}`
	w := serveHandlerFull(ConvertAmount(svc), "POST", "/exchange/convert", "/exchange/convert", body, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestConvertAmount_NotFound(t *testing.T) {
	svc := &stubExchangeClient{
		convertFn: func(ctx context.Context, in *exchangepb.ConvertAmountRequest, opts ...grpc.CallOption) (*exchangepb.ConvertAmountResponse, error) {
			return nil, status.Error(codes.NotFound, "account not found")
		},
	}
	body := `{"fromAccount":"A","toAccount":"B","amount":100}`
	w := serveHandlerFull(ConvertAmount(svc), "POST", "/exchange/convert", "/exchange/convert", body, makeClientToken())
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestConvertAmount_PermissionDenied(t *testing.T) {
	svc := &stubExchangeClient{
		convertFn: func(ctx context.Context, in *exchangepb.ConvertAmountRequest, opts ...grpc.CallOption) (*exchangepb.ConvertAmountResponse, error) {
			return nil, status.Error(codes.PermissionDenied, "not yours")
		},
	}
	body := `{"fromAccount":"A","toAccount":"B","amount":100}`
	w := serveHandlerFull(ConvertAmount(svc), "POST", "/exchange/convert", "/exchange/convert", body, makeClientToken())
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestConvertAmount_FailedPrecondition(t *testing.T) {
	svc := &stubExchangeClient{
		convertFn: func(ctx context.Context, in *exchangepb.ConvertAmountRequest, opts ...grpc.CallOption) (*exchangepb.ConvertAmountResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "insufficient funds")
		},
	}
	body := `{"fromAccount":"A","toAccount":"B","amount":100}`
	w := serveHandlerFull(ConvertAmount(svc), "POST", "/exchange/convert", "/exchange/convert", body, makeClientToken())
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestConvertAmount_InvalidArgument(t *testing.T) {
	svc := &stubExchangeClient{
		convertFn: func(ctx context.Context, in *exchangepb.ConvertAmountRequest, opts ...grpc.CallOption) (*exchangepb.ConvertAmountResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "same currency")
		},
	}
	body := `{"fromAccount":"A","toAccount":"B","amount":100}`
	w := serveHandlerFull(ConvertAmount(svc), "POST", "/exchange/convert", "/exchange/convert", body, makeClientToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestConvertAmount_InternalError(t *testing.T) {
	svc := &stubExchangeClient{
		convertFn: func(ctx context.Context, in *exchangepb.ConvertAmountRequest, opts ...grpc.CallOption) (*exchangepb.ConvertAmountResponse, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	body := `{"fromAccount":"A","toAccount":"B","amount":100}`
	w := serveHandlerFull(ConvertAmount(svc), "POST", "/exchange/convert", "/exchange/convert", body, makeClientToken())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestConvertAmount_Happy(t *testing.T) {
	svc := &stubExchangeClient{
		convertFn: func(ctx context.Context, in *exchangepb.ConvertAmountRequest, opts ...grpc.CallOption) (*exchangepb.ConvertAmountResponse, error) {
			return &exchangepb.ConvertAmountResponse{
				FromCurrency: "EUR", ToCurrency: "RSD", FromAmount: 100, ToAmount: 11800, Rate: 118,
			}, nil
		},
	}
	body := `{"fromAccount":"A","toAccount":"B","amount":100}`
	w := serveHandlerFull(ConvertAmount(svc), "POST", "/exchange/convert", "/exchange/convert", body, makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- PreviewConversion ----

func TestPreviewConversion_NoToken(t *testing.T) {
	svc := &stubExchangeClient{}
	body := `{"fromCurrency":"EUR","toCurrency":"RSD","amount":100}`
	w := serveHandlerFull(PreviewConversion(svc), "POST", "/exchange/preview", "/exchange/preview", body, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPreviewConversion_BadJSON(t *testing.T) {
	svc := &stubExchangeClient{}
	w := serveHandlerFull(PreviewConversion(svc), "POST", "/exchange/preview", "/exchange/preview", `{bad}`, makeClientToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreviewConversion_InvalidArgument(t *testing.T) {
	svc := &stubExchangeClient{
		previewFn: func(ctx context.Context, in *exchangepb.PreviewConversionRequest, opts ...grpc.CallOption) (*exchangepb.PreviewConversionResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "same currency")
		},
	}
	body := `{"fromCurrency":"EUR","toCurrency":"EUR","amount":100}`
	w := serveHandlerFull(PreviewConversion(svc), "POST", "/exchange/preview", "/exchange/preview", body, makeClientToken())
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreviewConversion_NotFound(t *testing.T) {
	svc := &stubExchangeClient{
		previewFn: func(ctx context.Context, in *exchangepb.PreviewConversionRequest, opts ...grpc.CallOption) (*exchangepb.PreviewConversionResponse, error) {
			return nil, status.Error(codes.NotFound, "rate not found")
		},
	}
	body := `{"fromCurrency":"EUR","toCurrency":"XXX","amount":100}`
	w := serveHandlerFull(PreviewConversion(svc), "POST", "/exchange/preview", "/exchange/preview", body, makeClientToken())
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPreviewConversion_InternalError(t *testing.T) {
	svc := &stubExchangeClient{
		previewFn: func(ctx context.Context, in *exchangepb.PreviewConversionRequest, opts ...grpc.CallOption) (*exchangepb.PreviewConversionResponse, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	body := `{"fromCurrency":"EUR","toCurrency":"USD","amount":100}`
	w := serveHandlerFull(PreviewConversion(svc), "POST", "/exchange/preview", "/exchange/preview", body, makeClientToken())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPreviewConversion_Happy(t *testing.T) {
	svc := &stubExchangeClient{
		previewFn: func(ctx context.Context, in *exchangepb.PreviewConversionRequest, opts ...grpc.CallOption) (*exchangepb.PreviewConversionResponse, error) {
			return &exchangepb.PreviewConversionResponse{
				FromCurrency: "EUR", ToCurrency: "USD", FromAmount: 100, ToAmount: 108, Rate: 1.08,
			}, nil
		},
	}
	body := `{"fromCurrency":"EUR","toCurrency":"USD","amount":100}`
	w := serveHandlerFull(PreviewConversion(svc), "POST", "/exchange/preview", "/exchange/preview", body, makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- GetExchangeHistory ----

func TestGetExchangeHistory_NoToken(t *testing.T) {
	svc := &stubExchangeClient{}
	w := serveHandlerFull(GetExchangeHistory(svc), "GET", "/exchange/history", "/exchange/history", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetExchangeHistory_Error(t *testing.T) {
	svc := &stubExchangeClient{
		historyFn: func(ctx context.Context, in *exchangepb.GetExchangeHistoryRequest, opts ...grpc.CallOption) (*exchangepb.GetExchangeHistoryResponse, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	w := serveHandlerFull(GetExchangeHistory(svc), "GET", "/exchange/history", "/exchange/history", "", makeClientToken())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetExchangeHistory_Happy(t *testing.T) {
	svc := &stubExchangeClient{
		historyFn: func(ctx context.Context, in *exchangepb.GetExchangeHistoryRequest, opts ...grpc.CallOption) (*exchangepb.GetExchangeHistoryResponse, error) {
			return &exchangepb.GetExchangeHistoryResponse{Transactions: []*exchangepb.ExchangeTransaction{
				{Id: 1, FromCurrency: "EUR", ToCurrency: "RSD", FromAmount: 100, ToAmount: 11800},
			}}, nil
		},
	}
	w := serveHandlerFull(GetExchangeHistory(svc), "GET", "/exchange/history", "/exchange/history", "", makeClientToken())
	assert.Equal(t, http.StatusOK, w.Code)
}
