package seeder

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"time"
)

var yahooHTTPClient = &http.Client{Timeout: 15 * time.Second}

// FetchStockBarsYahoo fetches 30 days of daily OHLCV data from Yahoo Finance (free, no key).
func FetchStockBarsYahoo(ticker string) ([]DailyBar, error) {
	url := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=1mo",
		ticker,
	)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("yahoo chart: build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := yahooHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("yahoo chart: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo chart: status %d for %s", resp.StatusCode, ticker)
	}

	var raw struct {
		Chart struct {
			Result []struct {
				Timestamps []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Close  []float64 `json:"close"`
						High   []float64 `json:"high"`
						Low    []float64 `json:"low"`
						Volume []int64   `json:"volume"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("yahoo chart: decode: %w", err)
	}
	if len(raw.Chart.Result) == 0 || len(raw.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, fmt.Errorf("yahoo chart: no data for %s", ticker)
	}

	result := raw.Chart.Result[0]
	quotes := result.Indicators.Quote[0]
	bars := make([]DailyBar, 0, len(result.Timestamps))
	for i, ts := range result.Timestamps {
		if i >= len(quotes.Close) || quotes.Close[i] == 0 {
			continue
		}
		ask := 0.0
		if i < len(quotes.High) {
			ask = quotes.High[i]
		}
		bid := 0.0
		if i < len(quotes.Low) {
			bid = quotes.Low[i]
		}
		vol := int64(0)
		if i < len(quotes.Volume) {
			vol = quotes.Volume[i]
		}
		bars = append(bars, DailyBar{
			Date:   time.Unix(ts, 0).UTC().Truncate(24 * time.Hour),
			Price:  quotes.Close[i],
			Ask:    ask,
			Bid:    bid,
			Volume: vol,
		})
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].Date.Before(bars[j].Date) })
	for i := 1; i < len(bars); i++ {
		bars[i].Change = bars[i].Price - bars[i-1].Price
	}
	return bars, nil
}

// StockFundamentals holds basic fundamental data for a stock.
type StockFundamentals struct {
	OutstandingShares int64
	DividendYield     float64
}

// FetchStockFundamentals fetches outstanding shares and dividend yield from Yahoo Finance.
func FetchStockFundamentals(ticker string) (*StockFundamentals, error) {
	url := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=defaultKeyStatistics,summaryDetail",
		ticker,
	)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("yahoo fundamentals: build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := yahooHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("yahoo fundamentals: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo fundamentals: status %d for %s", resp.StatusCode, ticker)
	}

	var raw struct {
		QuoteSummary struct {
			Result []struct {
				DefaultKeyStatistics struct {
					SharesOutstanding struct {
						Raw int64 `json:"raw"`
					} `json:"sharesOutstanding"`
				} `json:"defaultKeyStatistics"`
				SummaryDetail struct {
					DividendYield struct {
						Raw float64 `json:"raw"`
					} `json:"dividendYield"`
				} `json:"summaryDetail"`
			} `json:"result"`
		} `json:"quoteSummary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("yahoo fundamentals: decode: %w", err)
	}
	if len(raw.QuoteSummary.Result) == 0 {
		return nil, fmt.Errorf("yahoo fundamentals: no data for %s", ticker)
	}

	r := raw.QuoteSummary.Result[0]
	return &StockFundamentals{
		OutstandingShares: r.DefaultKeyStatistics.SharesOutstanding.Raw,
		DividendYield:     r.SummaryDetail.DividendYield.Raw,
	}, nil
}

// OptionRow is one option contract to be inserted.
type OptionRow struct {
	StockListingID    int64
	OptionType        string // "CALL" or "PUT"
	StrikePrice       float64
	ImpliedVolatility float64
	OpenInterest      int64
	SettlementDate    time.Time
}

// yahooOptionChain is the minimal structure we need from Yahoo Finance.
type yahooOptionChain struct {
	OptionChain struct {
		Result []struct {
			ExpirationDates []int64 `json:"expirationDates"`
			Options         []struct {
				Calls []yahooContract `json:"calls"`
				Puts  []yahooContract `json:"puts"`
			} `json:"options"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"optionChain"`
}

type yahooContract struct {
	Strike            float64 `json:"strike"`
	ImpliedVolatility float64 `json:"impliedVolatility"`
	OpenInterest      int64   `json:"openInterest"`
	Expiration        int64   `json:"expiration"` // Unix timestamp
}

// fetchYahooCrumb obtains an authenticated http.Client (with cookie jar) and a crumb
// token required by Yahoo Finance's v7 API.
func fetchYahooCrumb() (*http.Client, string, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, "", fmt.Errorf("yahoo crumb: cookiejar: %w", err)
	}
	client := &http.Client{Timeout: 15 * time.Second, Jar: jar}

	// Step 1: hit fc.yahoo.com to receive the consent cookie.
	req, _ := http.NewRequest(http.MethodGet, "https://fc.yahoo.com", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	if resp, err := client.Do(req); err == nil {
		_ = resp.Body.Close()
	}

	// Step 2: exchange the cookie for a crumb token.
	req, err = http.NewRequest(http.MethodGet, "https://query1.finance.yahoo.com/v1/test/getcrumb", nil)
	if err != nil {
		return nil, "", fmt.Errorf("yahoo crumb: build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("yahoo crumb: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("yahoo crumb: status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("yahoo crumb: read: %w", err)
	}
	crumb := strings.TrimSpace(string(b))
	if crumb == "" {
		return nil, "", fmt.Errorf("yahoo crumb: empty response")
	}
	return client, crumb, nil
}

// FetchOptions attempts to fetch option contracts for a ticker from Yahoo Finance.
// Returns nil, nil when the request fails or no data is available (caller should fall back).
// Fetches all available expiry dates by looping over the expirationDates list returned
// in the first response.
func FetchOptions(ticker string, stockListingID int64) ([]OptionRow, error) {
	client, crumb, err := fetchYahooCrumb()
	if err != nil {
		log.Printf("yahoo options: auth failed for %s: %v — falling back", ticker, err)
		return nil, nil
	}

	// fetchExpiry fetches one expiry's option chain. Pass ts=0 for the nearest expiry
	// (no date param), or a Unix timestamp to fetch a specific expiry.
	// Returns (rows, allExpiryDates, error).
	fetchExpiry := func(ts int64) ([]OptionRow, []int64, error) {
		reqURL := fmt.Sprintf(
			"https://query1.finance.yahoo.com/v7/finance/options/%s?crumb=%s",
			ticker, url.QueryEscape(crumb),
		)
		if ts != 0 {
			reqURL += fmt.Sprintf("&date=%d", ts)
		}
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			return nil, nil, fmt.Errorf("status %d", resp.StatusCode)
		}

		var chain yahooOptionChain
		if err := json.NewDecoder(resp.Body).Decode(&chain); err != nil {
			return nil, nil, err
		}
		if len(chain.OptionChain.Result) == 0 {
			return nil, nil, nil
		}

		result := chain.OptionChain.Result[0]
		var rows []OptionRow
		for _, opt := range result.Options {
			for _, c := range opt.Calls {
				rows = append(rows, OptionRow{
					StockListingID:    stockListingID,
					OptionType:        "CALL",
					StrikePrice:       c.Strike,
					ImpliedVolatility: c.ImpliedVolatility,
					OpenInterest:      c.OpenInterest,
					SettlementDate:    time.Unix(c.Expiration, 0).UTC(),
				})
			}
			for _, p := range opt.Puts {
				rows = append(rows, OptionRow{
					StockListingID:    stockListingID,
					OptionType:        "PUT",
					StrikePrice:       p.Strike,
					ImpliedVolatility: p.ImpliedVolatility,
					OpenInterest:      p.OpenInterest,
					SettlementDate:    time.Unix(p.Expiration, 0).UTC(),
				})
			}
		}
		return rows, result.ExpirationDates, nil
	}

	// First call — fetches the nearest expiry and returns the full list of expiry dates.
	rows, expiryDates, err := fetchExpiry(0)
	if err != nil {
		log.Printf("yahoo options: first fetch failed for %s: %v — falling back", ticker, err)
		return nil, nil
	}
	if rows == nil && expiryDates == nil {
		return nil, nil
	}

	// Loop over remaining expiry dates (index 0 is already fetched above).
	if len(expiryDates) > 1 {
		for _, ts := range expiryDates[1:] {
			time.Sleep(150 * time.Millisecond) // stay well under Yahoo rate limits
			moreRows, _, err := fetchExpiry(ts)
			if err != nil {
				log.Printf("yahoo options: fetch for %s date %d failed: %v — skipping", ticker, ts, err)
				continue
			}
			rows = append(rows, moreRows...)
		}
	}

	log.Printf("yahoo options: fetched %d contracts across %d expiries for %s",
		len(rows), len(expiryDates), ticker)
	return rows, nil
}

// GenerateOptions creates synthetic option contracts for a stock when Yahoo Finance is unavailable.
//
// Settlement dates:
//   - 6, 12, 18, 24, 30 days from today
//   - then 30, 60, 90, 120, 150, 180 days from today
//
// Strike prices: round(stockPrice) ± 5 at $1 increments.
// For each (settlement × strike): one CALL and one PUT with impliedVolatility = 1.0.
func GenerateOptions(stockListingID int64, stockPrice float64) []OptionRow {
	today := time.Now().UTC().Truncate(24 * time.Hour)

	// Build settlement date list.
	var settlements []time.Time
	for i := 1; i <= 5; i++ {
		settlements = append(settlements, today.Add(time.Duration(i*6)*24*time.Hour))
	}
	for i := 1; i <= 6; i++ {
		settlements = append(settlements, today.Add(time.Duration(i*30)*24*time.Hour))
	}

	baseStrike := math.Round(stockPrice)
	var rows []OptionRow
	for _, sd := range settlements {
		for delta := -5; delta <= 5; delta++ {
			strike := baseStrike + float64(delta)
			if strike <= 0 {
				continue
			}
			for _, optType := range []string{"CALL", "PUT"} {
				rows = append(rows, OptionRow{
					StockListingID:    stockListingID,
					OptionType:        optType,
					StrikePrice:       strike,
					ImpliedVolatility: 1.0,
					OpenInterest:      0,
					SettlementDate:    sd,
				})
			}
		}
	}
	return rows
}
