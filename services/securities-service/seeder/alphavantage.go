package seeder

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const avBase = "https://www.alphavantage.co/query"

// avRateLimit is the minimum delay between AlphaVantage calls on the free tier (5 req/min).
const avRateLimit = 12 * time.Second

var avClient = &http.Client{Timeout: 30 * time.Second}

// avGet performs a GET against AlphaVantage and decodes the JSON response into dst.
// Returns false (and logs) if the response contains a rate-limit "Note" or "Information" key.
func avGet(params map[string]string, dst interface{}) (ok bool, err error) {
	req, err := http.NewRequest(http.MethodGet, avBase, nil)
	if err != nil {
		return false, fmt.Errorf("av: build request: %w", err)
	}
	q := req.URL.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := avClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("av: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests {
		log.Printf("alphavantage: rate limit hit (429), skipping")
		return false, nil
	}

	// Decode into a raw map first to check for Note / Information fields.
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return false, fmt.Errorf("av: decode: %w", err)
	}
	if note, ok := raw["Note"]; ok {
		log.Printf("alphavantage: rate-limit note: %s", note)
		return false, nil
	}
	if info, ok := raw["Information"]; ok {
		log.Printf("alphavantage: information message: %s", info)
		return false, nil
	}

	// Re-encode and decode into the target struct.
	b, _ := json.Marshal(raw)
	if err := json.Unmarshal(b, dst); err != nil {
		return false, fmt.Errorf("av: unmarshal into target: %w", err)
	}
	return true, nil
}

// Quote holds the fields we extract from GLOBAL_QUOTE.
type Quote struct {
	Price  float64
	Ask    float64 // AlphaVantage calls this "high"
	Bid    float64 // "low"
	Change float64
	Volume int64
}

type avGlobalQuoteResp struct {
	GlobalQuote struct {
		Price  string `json:"05. price"`
		High   string `json:"03. high"`
		Low    string `json:"04. low"`
		Change string `json:"09. change"`
		Volume string `json:"06. volume"`
	} `json:"Global Quote"`
}

// FetchGlobalQuote fetches the latest price snapshot for a stock ticker.
// Sleeps avRateLimit before the call.
func FetchGlobalQuote(ticker, apiKey string) (*Quote, error) {
	time.Sleep(avRateLimit)
	var resp avGlobalQuoteResp
	ok, err := avGet(map[string]string{
		"function": "GLOBAL_QUOTE",
		"symbol":   ticker,
		"apikey":   apiKey,
	}, &resp)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	q := &Quote{}
	q.Price, _ = strconv.ParseFloat(resp.GlobalQuote.Price, 64)
	q.Ask, _ = strconv.ParseFloat(resp.GlobalQuote.High, 64)
	q.Bid, _ = strconv.ParseFloat(resp.GlobalQuote.Low, 64)
	q.Change, _ = strconv.ParseFloat(resp.GlobalQuote.Change, 64)
	q.Volume, _ = strconv.ParseInt(strings.ReplaceAll(resp.GlobalQuote.Volume, ",", ""), 10, 64)
	return q, nil
}

// Overview holds the fields we extract from COMPANY_OVERVIEW.
type Overview struct {
	Name              string
	Exchange          string
	OutstandingShares int64
	DividendYield     float64
}

type avOverviewResp struct {
	Name              string `json:"Name"`
	Exchange          string `json:"Exchange"`
	SharesOutstanding string `json:"SharesOutstanding"`
	DividendYield     string `json:"DividendYield"`
}

// FetchCompanyOverview fetches company metadata for a stock ticker.
// Sleeps avRateLimit before the call.
func FetchCompanyOverview(ticker, apiKey string) (*Overview, error) {
	time.Sleep(avRateLimit)
	var resp avOverviewResp
	ok, err := avGet(map[string]string{
		"function": "OVERVIEW",
		"symbol":   ticker,
		"apikey":   apiKey,
	}, &resp)
	if err != nil {
		return nil, err
	}
	if !ok || resp.Name == "" {
		return nil, nil
	}
	ov := &Overview{
		Name:     resp.Name,
		Exchange: resp.Exchange,
	}
	ov.OutstandingShares, _ = strconv.ParseInt(resp.SharesOutstanding, 10, 64)
	ov.DividendYield, _ = strconv.ParseFloat(resp.DividendYield, 64)
	return ov, nil
}

// DailyBar is one row of OHLCV data for a single calendar day.
type DailyBar struct {
	Date   time.Time
	Price  float64
	Ask    float64
	Bid    float64
	Change float64
	Volume int64
}

type avDailyResp struct {
	TimeSeries map[string]struct {
		Close  string `json:"4. close"`
		High   string `json:"2. high"`
		Low    string `json:"3. low"`
		Volume string `json:"5. volume"`
	} `json:"Time Series (Daily)"`
}

// FetchDailySeries fetches full daily price history for a stock.
// Sleeps avRateLimit before the call.
func FetchDailySeries(ticker, apiKey string) ([]DailyBar, error) {
	time.Sleep(avRateLimit)
	var resp avDailyResp
	ok, err := avGet(map[string]string{
		"function":   "TIME_SERIES_DAILY",
		"symbol":     ticker,
		"outputsize": "full",
		"apikey":     apiKey,
	}, &resp)
	if err != nil {
		return nil, err
	}
	if !ok || resp.TimeSeries == nil {
		return nil, nil
	}
	bars := make([]DailyBar, 0, len(resp.TimeSeries))
	for dateStr, v := range resp.TimeSeries {
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		bar := DailyBar{Date: t}
		bar.Price, _ = strconv.ParseFloat(v.Close, 64)
		bar.Ask, _ = strconv.ParseFloat(v.High, 64)
		bar.Bid, _ = strconv.ParseFloat(v.Low, 64)
		bar.Volume, _ = strconv.ParseInt(strings.ReplaceAll(v.Volume, ",", ""), 10, 64)
		if len(bars) > 0 {
			bar.Change = bar.Price - bars[len(bars)-1].Price
		}
		bars = append(bars, bar)
	}
	// Sort ascending by date so Change can be computed properly.
	sort.Slice(bars, func(i, j int) bool { return bars[i].Date.Before(bars[j].Date) })
	for i := 1; i < len(bars); i++ {
		bars[i].Change = bars[i].Price - bars[i-1].Price
	}
	return bars, nil
}

// FXRate holds the current exchange rate returned by CURRENCY_EXCHANGE_RATE.
type FXRate struct {
	Price float64
	Ask   float64
	Bid   float64
}

type avFXRateResp struct {
	RealtimeCurrencyExchangeRate struct {
		ExchangeRate string `json:"5. Exchange Rate"`
		AskPrice     string `json:"9. Ask Price"`
		BidPrice     string `json:"8. Bid Price"`
	} `json:"Realtime Currency Exchange Rate"`
}

// FetchFXRate fetches the current exchange rate between two ISO currency codes.
// Sleeps avRateLimit before the call.
func FetchFXRate(fromCurrency, toCurrency, apiKey string) (*FXRate, error) {
	time.Sleep(avRateLimit)
	var resp avFXRateResp
	ok, err := avGet(map[string]string{
		"function":      "CURRENCY_EXCHANGE_RATE",
		"from_currency": fromCurrency,
		"to_currency":   toCurrency,
		"apikey":        apiKey,
	}, &resp)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	r := &FXRate{}
	r.Price, _ = strconv.ParseFloat(resp.RealtimeCurrencyExchangeRate.ExchangeRate, 64)
	r.Ask, _ = strconv.ParseFloat(resp.RealtimeCurrencyExchangeRate.AskPrice, 64)
	r.Bid, _ = strconv.ParseFloat(resp.RealtimeCurrencyExchangeRate.BidPrice, 64)
	return r, nil
}

type avFXDailyResp struct {
	TimeSeries map[string]struct {
		Close string `json:"4. close"`
		High  string `json:"2. high"`
		Low   string `json:"3. low"`
	} `json:"Time Series FX (Daily)"`
}

// FetchFXDaily fetches full daily price history for a forex pair.
// Sleeps avRateLimit before the call.
func FetchFXDaily(fromCurrency, toCurrency, apiKey string) ([]DailyBar, error) {
	time.Sleep(avRateLimit)
	var resp avFXDailyResp
	ok, err := avGet(map[string]string{
		"function":    "FX_DAILY",
		"from_symbol": fromCurrency,
		"to_symbol":   toCurrency,
		"outputsize":  "full",
		"apikey":      apiKey,
	}, &resp)
	if err != nil {
		return nil, err
	}
	if !ok || resp.TimeSeries == nil {
		return nil, nil
	}
	bars := make([]DailyBar, 0, len(resp.TimeSeries))
	for dateStr, v := range resp.TimeSeries {
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		bar := DailyBar{Date: t}
		bar.Price, _ = strconv.ParseFloat(v.Close, 64)
		bar.Ask, _ = strconv.ParseFloat(v.High, 64)
		bar.Bid, _ = strconv.ParseFloat(v.Low, 64)
		bars = append(bars, bar)
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].Date.Before(bars[j].Date) })
	for i := 1; i < len(bars); i++ {
		bars[i].Change = bars[i].Price - bars[i-1].Price
	}
	return bars, nil
}
