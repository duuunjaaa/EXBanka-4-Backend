package seeder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

const alpacaDataBase = "https://data.alpaca.markets/v2/stocks"

var alpacaHTTPClient = &http.Client{Timeout: 30 * time.Second}

func alpacaGet(url, apiKey, secretKey string, dst interface{}) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("alpaca data: build request: %w", err)
	}
	req.Header.Set("APCA-API-KEY-ID", apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", secretKey)

	resp, err := alpacaHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("alpaca data: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("alpaca data: status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

// FetchStockSnapshot returns the latest price snapshot for a stock ticker.
func FetchStockSnapshot(symbol, apiKey, secretKey string) (*Quote, error) {
	url := fmt.Sprintf("%s/snapshots?symbols=%s&feed=iex", alpacaDataBase, symbol)

	var raw map[string]struct {
		LatestTrade struct {
			Price float64 `json:"p"`
		} `json:"latestTrade"`
		LatestQuote struct {
			AskPrice float64 `json:"ap"`
			BidPrice float64 `json:"bp"`
		} `json:"latestQuote"`
		DailyBar struct {
			Close  float64 `json:"c"`
			Volume int64   `json:"v"`
		} `json:"dailyBar"`
		PrevDailyBar struct {
			Close float64 `json:"c"`
		} `json:"prevDailyBar"`
	}

	if err := alpacaGet(url, apiKey, secretKey, &raw); err != nil {
		return nil, err
	}
	snap, ok := raw[symbol]
	if !ok {
		return nil, nil
	}

	price := snap.LatestTrade.Price
	if price == 0 {
		price = snap.DailyBar.Close
	}
	return &Quote{
		Price:  price,
		Ask:    snap.LatestQuote.AskPrice,
		Bid:    snap.LatestQuote.BidPrice,
		Volume: snap.DailyBar.Volume,
		Change: snap.DailyBar.Close - snap.PrevDailyBar.Close,
	}, nil
}

// FetchStockBars returns up to 30 daily bars for a stock ticker.
func FetchStockBars(symbol, apiKey, secretKey string) ([]DailyBar, error) {
	start := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	url := fmt.Sprintf("%s/bars?symbols=%s&timeframe=1Day&limit=30&sort=asc&feed=sip&start=%s", alpacaDataBase, symbol, start)

	var raw struct {
		Bars map[string][]struct {
			Time   time.Time `json:"t"`
			High   float64   `json:"h"`
			Low    float64   `json:"l"`
			Close  float64   `json:"c"`
			Volume int64     `json:"v"`
		} `json:"bars"`
	}

	if err := alpacaGet(url, apiKey, secretKey, &raw); err != nil {
		return nil, err
	}
	rawBars, ok := raw.Bars[symbol]
	if !ok || len(rawBars) == 0 {
		return nil, nil
	}

	bars := make([]DailyBar, len(rawBars))
	for i, b := range rawBars {
		bars[i] = DailyBar{
			Date:   b.Time,
			Price:  b.Close,
			Ask:    b.High,
			Bid:    b.Low,
			Volume: b.Volume,
		}
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].Date.Before(bars[j].Date) })
	for i := 1; i < len(bars); i++ {
		bars[i].Change = bars[i].Price - bars[i-1].Price
	}
	return bars, nil
}
