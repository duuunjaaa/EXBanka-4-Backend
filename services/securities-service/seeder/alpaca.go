package seeder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const alpacaMaxTickers = 50

// alpacaAsset is the subset of Alpaca's asset object we care about.
type alpacaAsset struct {
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Exchange string `json:"exchange"`
	Tradable bool   `json:"tradable"`
}

// FetchTickers fetches up to alpacaMaxTickers tradable US equity tickers from Alpaca Markets.
// Endpoint: GET https://data.alpaca.markets/v2/assets?status=active&asset_class=us_equity
func FetchTickers(apiKey, secretKey string) ([]alpacaAsset, error) {
	req, err := http.NewRequest(http.MethodGet, "https://paper-api.alpaca.markets/v2/assets", nil)
	if err != nil {
		return nil, fmt.Errorf("alpaca: build request: %w", err)
	}
	q := req.URL.Query()
	q.Set("status", "active")
	q.Set("asset_class", "us_equity")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("APCA-API-KEY-ID", apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", secretKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alpaca: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca: unexpected status %d", resp.StatusCode)
	}

	var assets []alpacaAsset
	if err := json.NewDecoder(resp.Body).Decode(&assets); err != nil {
		return nil, fmt.Errorf("alpaca: decode response: %w", err)
	}

	// Keep only tradable assets and cap to alpacaMaxTickers
	var result []alpacaAsset
	for _, a := range assets {
		if a.Tradable && len(result) < alpacaMaxTickers {
			result = append(result, a)
		}
	}
	return result, nil
}
