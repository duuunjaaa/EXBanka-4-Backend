package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/exchange"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRedisServer starts miniredis and returns a connected client.
func newRedisServer(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

// newExchangeServerWithRedis creates an ExchangeServer wired to both sqlmock and miniredis.
func newExchangeServerWithRedis(t *testing.T) (*ExchangeServer, *miniredis.Miniredis) {
	t.Helper()
	s, _, _ := newExchangeServer(t)
	mr, rdb := newRedisServer(t)
	s.Redis = rdb
	return s, mr
}

// ── loadCachedRates ──────────────────────────────────────────────────────────

func TestLoadCachedRates_NilRedis_ReturnsNil(t *testing.T) {
	s := &ExchangeServer{Redis: nil}
	assert.Nil(t, s.loadCachedRates(context.Background()))
}

func TestLoadCachedRates_KeyMissing_ReturnsNil(t *testing.T) {
	_, rdb := newRedisServer(t)
	s := &ExchangeServer{Redis: rdb}
	assert.Nil(t, s.loadCachedRates(context.Background()))
}

func TestLoadCachedRates_InvalidJSON_ReturnsNil(t *testing.T) {
	mr, rdb := newRedisServer(t)
	require.NoError(t, mr.Set(ratesCacheKey(), "not-json"))

	s := &ExchangeServer{Redis: rdb}
	assert.Nil(t, s.loadCachedRates(context.Background()))
}

func TestLoadCachedRates_ValidCache_ReturnsParsedRates(t *testing.T) {
	mr, rdb := newRedisServer(t)

	payload := map[string]cachedRate{
		"EUR": {BuyingRate: 115.0, SellingRate: 118.0, MiddleRate: 116.5},
		"USD": {BuyingRate: 107.0, SellingRate: 110.0, MiddleRate: 108.5},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, mr.Set(ratesCacheKey(), string(data)))

	s := &ExchangeServer{Redis: rdb}
	result := s.loadCachedRates(context.Background())
	require.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.Equal(t, 115.0, result["EUR"].BuyingRate)
	assert.Equal(t, 108.5, result["USD"].MiddleRate)
}

// ── storeCachedRates ─────────────────────────────────────────────────────────

func TestStoreCachedRates_NilRedis_NoOp(t *testing.T) {
	s := &ExchangeServer{Redis: nil}
	// Must not panic.
	s.storeCachedRates(context.Background(), []*pb.ExchangeRate{
		{CurrencyCode: "EUR", BuyingRate: 115.0, SellingRate: 118.0, MiddleRate: 116.5},
	})
}

func TestStoreCachedRates_WritesKeyWithTTL(t *testing.T) {
	mr, rdb := newRedisServer(t)
	s := &ExchangeServer{Redis: rdb}

	rates := []*pb.ExchangeRate{
		{CurrencyCode: "EUR", BuyingRate: 115.0, SellingRate: 118.0, MiddleRate: 116.5},
		{CurrencyCode: "USD", BuyingRate: 107.0, SellingRate: 110.0, MiddleRate: 108.5},
	}
	s.storeCachedRates(context.Background(), rates)

	// Key must exist.
	val, err := mr.Get(ratesCacheKey())
	require.NoError(t, err)
	assert.NotEmpty(t, val)

	// TTL must be set (> 0 and ≤ 24 h).
	ttl := mr.TTL(ratesCacheKey())
	assert.Greater(t, ttl, time.Duration(0))
	assert.LessOrEqual(t, ttl, 24*time.Hour)

	// Stored JSON must be readable back.
	var stored map[string]cachedRate
	require.NoError(t, json.Unmarshal([]byte(val), &stored))
	assert.Equal(t, 115.0, stored["EUR"].BuyingRate)
	assert.Equal(t, 108.5, stored["USD"].MiddleRate)
}

func TestStoreCachedRates_RoundTrip(t *testing.T) {
	_, rdb := newRedisServer(t)
	s := &ExchangeServer{Redis: rdb}

	rates := []*pb.ExchangeRate{
		{CurrencyCode: "GBP", BuyingRate: 130.0, SellingRate: 133.0, MiddleRate: 131.5},
	}
	s.storeCachedRates(context.Background(), rates)

	result := s.loadCachedRates(context.Background())
	require.NotNil(t, result)
	assert.Equal(t, 130.0, result["GBP"].BuyingRate)
	assert.Equal(t, 133.0, result["GBP"].SellingRate)
	assert.Equal(t, 131.5, result["GBP"].MiddleRate)
}

// ── GetExchangeRates with Redis cache ─────────────────────────────────────────

func TestGetExchangeRates_CacheHit_DoesNotQueryDB(t *testing.T) {
	s, _ := newExchangeServerWithRedis(t)

	// Pre-populate Redis cache.
	payload := map[string]cachedRate{
		"EUR": {BuyingRate: 115.0, SellingRate: 118.0, MiddleRate: 116.5},
	}
	data, _ := json.Marshal(payload)
	_ = s.Redis.Set(context.Background(), ratesCacheKey(), data, time.Hour).Err()

	// No sqlmock expectations set — any DB call would cause the test to fail.
	resp, err := s.GetExchangeRates(context.Background(), &pb.GetExchangeRatesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Rates, 1)
	assert.Equal(t, "EUR", resp.Rates[0].CurrencyCode)
	assert.Equal(t, 115.0, resp.Rates[0].BuyingRate)
}

func TestGetExchangeRates_CacheHit_ResponseIsSortedByCurrencyCode(t *testing.T) {
	s, _ := newExchangeServerWithRedis(t)

	payload := map[string]cachedRate{
		"USD": {BuyingRate: 107.0, SellingRate: 110.0, MiddleRate: 108.5},
		"EUR": {BuyingRate: 115.0, SellingRate: 118.0, MiddleRate: 116.5},
		"GBP": {BuyingRate: 130.0, SellingRate: 133.0, MiddleRate: 131.5},
	}
	data, _ := json.Marshal(payload)
	_ = s.Redis.Set(context.Background(), ratesCacheKey(), data, time.Hour).Err()

	resp, err := s.GetExchangeRates(context.Background(), &pb.GetExchangeRatesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Rates, 3)
	assert.Equal(t, "EUR", resp.Rates[0].CurrencyCode)
	assert.Equal(t, "GBP", resp.Rates[1].CurrencyCode)
	assert.Equal(t, "USD", resp.Rates[2].CurrencyCode)
}

func TestGetExchangeRates_CacheMiss_QueriesDBAndPopulatesCache(t *testing.T) {
	s, mr := newExchangeServerWithRedis(t)

	// Provide DB expectations (rates already exist in DB).
	_, dbMock, _ := newExchangeServer(t)
	// Replace just the DB, reuse Redis.
	realDB := s.DB
	_ = realDB

	// Use a fresh server with both sqlmock and Redis.
	s2, dbMock2, _ := newExchangeServer(t)
	_, rdb := newRedisServer(t)
	s2.Redis = rdb

	expectRatesAlreadyExist(dbMock2)
	dbMock2.ExpectQuery(`SELECT currency_code, buying_rate, selling_rate, middle_rate, date`).
		WillReturnRows(sampleRateRows())

	resp, err := s2.GetExchangeRates(context.Background(), &pb.GetExchangeRatesRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Rates, 2)

	// After the DB query the cache must be populated.
	cached := s2.loadCachedRates(context.Background())
	require.NotNil(t, cached, "cache must be populated after DB query")
	assert.Contains(t, cached, "EUR")
	assert.Contains(t, cached, "USD")

	_ = mr // used for TTL assertions in other tests
	_ = dbMock
}

func TestGetExchangeRates_CacheHit_DateFieldIsToday(t *testing.T) {
	s, _ := newExchangeServerWithRedis(t)

	payload := map[string]cachedRate{
		"CHF": {BuyingRate: 120.0, SellingRate: 123.0, MiddleRate: 121.5},
	}
	data, _ := json.Marshal(payload)
	_ = s.Redis.Set(context.Background(), ratesCacheKey(), data, time.Hour).Err()

	resp, err := s.GetExchangeRates(context.Background(), &pb.GetExchangeRatesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Rates, 1)
	assert.Equal(t, time.Now().Format("2006-01-02"), resp.Rates[0].Date)
}
