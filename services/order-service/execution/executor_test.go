package execution

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- CalculatePrice ---

func TestMarketBuy(t *testing.T) {
	price, ok := CalculatePrice("MARKET", "BUY", 105.0, 103.0, 0, 0)
	assert.True(t, ok)
	assert.Equal(t, 105.0, price)
}

func TestMarketSell(t *testing.T) {
	price, ok := CalculatePrice("MARKET", "SELL", 105.0, 103.0, 0, 0)
	assert.True(t, ok)
	assert.Equal(t, 103.0, price)
}

func TestLimitBuyExecutes(t *testing.T) {
	// ask=100, limit=105 → ask ≤ limit → executes at min(105,100)=100
	price, ok := CalculatePrice("LIMIT", "BUY", 100.0, 99.0, 105.0, 0)
	assert.True(t, ok)
	assert.Equal(t, 100.0, price)
}

func TestLimitBuySkips(t *testing.T) {
	// ask=110, limit=105 → ask > limit → does not execute
	_, ok := CalculatePrice("LIMIT", "BUY", 110.0, 108.0, 105.0, 0)
	assert.False(t, ok)
}

func TestLimitSellExecutes(t *testing.T) {
	// bid=106, limit=105 → bid ≥ limit → executes at max(105,106)=106
	price, ok := CalculatePrice("LIMIT", "SELL", 108.0, 106.0, 105.0, 0)
	assert.True(t, ok)
	assert.Equal(t, 106.0, price)
}

func TestLimitSellSkips(t *testing.T) {
	// bid=103, limit=105 → bid < limit → does not execute
	_, ok := CalculatePrice("LIMIT", "SELL", 105.0, 103.0, 105.0, 0)
	assert.False(t, ok)
}

func TestStopBuyActivates(t *testing.T) {
	// ask=106, stop=105 → ask > stop → activates as market buy at ask
	price, ok := CalculatePrice("STOP", "BUY", 106.0, 104.0, 0, 105.0)
	assert.True(t, ok)
	assert.Equal(t, 106.0, price)
}

func TestStopBuyWaits(t *testing.T) {
	// ask=104, stop=105 → ask ≤ stop → not triggered
	_, ok := CalculatePrice("STOP", "BUY", 104.0, 102.0, 0, 105.0)
	assert.False(t, ok)
}

func TestStopSellActivates(t *testing.T) {
	// bid=99, stop=100 → bid < stop → activates as market sell at bid
	price, ok := CalculatePrice("STOP", "SELL", 101.0, 99.0, 0, 100.0)
	assert.True(t, ok)
	assert.Equal(t, 99.0, price)
}

func TestStopLimitBuyActivates(t *testing.T) {
	// ask=105, stop=105 → ask ≥ stop → executes
	price, ok := CalculatePrice("STOP_LIMIT", "BUY", 105.0, 103.0, 106.0, 105.0)
	assert.True(t, ok)
	assert.Equal(t, 105.0, price)
}

func TestStopLimitBuyWaits(t *testing.T) {
	// ask=104, stop=105 → ask < stop → not triggered
	_, ok := CalculatePrice("STOP_LIMIT", "BUY", 104.0, 102.0, 106.0, 105.0)
	assert.False(t, ok)
}

// --- CalculateCommission ---

func TestCommissionMarketBelowCap(t *testing.T) {
	// 14% of 40 = 5.6, cap is 7 → 5.6
	c := CalculateCommission("MARKET", 40.0)
	assert.InDelta(t, 5.6, c, 0.001)
}

func TestCommissionMarketCapped(t *testing.T) {
	// 14% of 100 = 14, cap is 7 → 7
	c := CalculateCommission("MARKET", 100.0)
	assert.Equal(t, 7.0, c)
}

func TestCommissionStopCapped(t *testing.T) {
	c := CalculateCommission("STOP", 200.0)
	assert.Equal(t, 7.0, c)
}

func TestCommissionLimitBelowCap(t *testing.T) {
	// 24% of 40 = 9.6, cap is 12 → 9.6
	c := CalculateCommission("LIMIT", 40.0)
	assert.InDelta(t, 9.6, c, 0.001)
}

func TestCommissionLimitCapped(t *testing.T) {
	// 24% of 100 = 24, cap is 12 → 12
	c := CalculateCommission("LIMIT", 100.0)
	assert.Equal(t, 12.0, c)
}

func TestCommissionStopLimit(t *testing.T) {
	c := CalculateCommission("STOP_LIMIT", 200.0)
	assert.Equal(t, 12.0, c)
}

// --- ApproximatePrice ---

func TestApproximatePrice(t *testing.T) {
	// contractSize=2, pricePerUnit=50, quantity=3 → 300
	p := ApproximatePrice(2, 50.0, 3)
	assert.Equal(t, 300.0, p)
}

// --- ValidateMargin ---

func TestValidateMarginByLoan(t *testing.T) {
	assert.True(t, ValidateMargin(1000.0, 1500.0, 500.0))
}

func TestValidateMarginByBalance(t *testing.T) {
	assert.True(t, ValidateMargin(1000.0, 200.0, 1500.0))
}

func TestValidateMarginFails(t *testing.T) {
	assert.False(t, ValidateMargin(1000.0, 500.0, 800.0))
}

func TestValidateMarginBothMet(t *testing.T) {
	assert.True(t, ValidateMargin(1000.0, 2000.0, 2000.0))
}

// --- IsAfterHours ---

func TestIsAfterHoursTrue(t *testing.T) {
	// NYSE closes at 16:00 America/New_York; 2 hours before close = 14:00 NY = within 4h window
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, 4, 8, 14, 30, 0, 0, loc)
	assert.True(t, IsAfterHours("16:00", "America/New_York", now))
}

func TestIsAfterHoursFalse_BeforeWindow(t *testing.T) {
	// 10:00 NY is 6 hours before 16:00 → not in the 4h window
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, 4, 8, 10, 0, 0, 0, loc)
	assert.False(t, IsAfterHours("16:00", "America/New_York", now))
}

func TestIsAfterHoursFalse_AfterClose(t *testing.T) {
	// 17:00 NY is after close → not in the window
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, 4, 8, 17, 0, 0, 0, loc)
	assert.False(t, IsAfterHours("16:00", "America/New_York", now))
}

// --- FillInterval ---

func TestFillIntervalAfterHoursAdds30Min(t *testing.T) {
	// With a very high volume and small remaining, seconds ≈ 0, so result ≈ 30 min if afterHours
	d := FillInterval(1_000_000_000, 1, true)
	assert.GreaterOrEqual(t, d, 30*time.Minute)
	assert.Less(t, d, 31*time.Minute)
}

func TestFillIntervalNoAfterHours(t *testing.T) {
	d := FillInterval(1_000_000_000, 1, false)
	assert.Less(t, d, 30*time.Minute)
}

func TestFillIntervalZeroVolumeFallback(t *testing.T) {
	d := FillInterval(0, 10, false)
	assert.Equal(t, 5*time.Second, d)
}

func TestCommissionNeverNegative(t *testing.T) {
	assert.True(t, CalculateCommission("MARKET", 0) >= 0)
	assert.True(t, CalculateCommission("LIMIT", 0) >= 0)
}

func TestApproximatePriceZero(t *testing.T) {
	assert.Equal(t, 0.0, ApproximatePrice(0, 100.0, 10))
}

func TestCommissionMarketExactCap(t *testing.T) {
	// 14% of 50 = 7.0, exactly at cap
	c := CalculateCommission("MARKET", 50.0)
	assert.Equal(t, 7.0, math.Round(c*1000)/1000)
}

func TestCommissionUnknownType(t *testing.T) {
	assert.Equal(t, 0.0, CalculateCommission("UNKNOWN", 100.0))
}

func TestCalculatePriceUnknownType(t *testing.T) {
	price, ok := CalculatePrice("UNKNOWN", "BUY", 100.0, 99.0, 0, 0)
	assert.False(t, ok)
	assert.Equal(t, 0.0, price)
}

func TestStopSellDoesNotActivate(t *testing.T) {
	// bid=101 ≥ stop=100 → does not trigger
	_, ok := CalculatePrice("STOP", "SELL", 103.0, 101.0, 0, 100.0)
	assert.False(t, ok)
}

func TestStopLimitSellActivates(t *testing.T) {
	// bid=99, stop=100 → bid < stop → executes
	price, ok := CalculatePrice("STOP_LIMIT", "SELL", 101.0, 99.0, 98.0, 100.0)
	assert.True(t, ok)
	assert.Equal(t, 99.0, price)
}

func TestStopLimitSellDoesNotActivate(t *testing.T) {
	// bid=101 ≥ stop=100 → not triggered
	_, ok := CalculatePrice("STOP_LIMIT", "SELL", 103.0, 101.0, 98.0, 100.0)
	assert.False(t, ok)
}

func TestIsAfterHours_InvalidTimezone(t *testing.T) {
	now := time.Now()
	assert.False(t, IsAfterHours("16:00", "Invalid/Zone", now))
}

func TestIsAfterHours_InvalidTimeFormat(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, 4, 8, 15, 0, 0, 0, loc)
	assert.False(t, IsAfterHours("bad-time", "America/New_York", now))
}

func TestFillIntervalZeroRemaining(t *testing.T) {
	d := FillInterval(1_000_000, 0, false)
	assert.Equal(t, 5*time.Second, d)
}

// --- STOP_LIMIT limit guard (S62) ---

func TestStopLimitBuyAboveLimit(t *testing.T) {
	// ask=110 >= stop=105 (triggered), but ask=110 > limit=108 → should NOT execute
	_, ok := CalculatePrice("STOP_LIMIT", "BUY", 110.0, 108.0, 108.0, 105.0)
	assert.False(t, ok)
}

func TestStopLimitBuyAtLimit(t *testing.T) {
	// ask=108 >= stop=105, ask=108 == limit=108 → executes at ask
	price, ok := CalculatePrice("STOP_LIMIT", "BUY", 108.0, 106.0, 108.0, 105.0)
	assert.True(t, ok)
	assert.Equal(t, 108.0, price)
}

func TestStopLimitSellBelowLimit(t *testing.T) {
	// bid=94 < stop=100 (triggered), but bid=94 < limit=96 → should NOT execute
	_, ok := CalculatePrice("STOP_LIMIT", "SELL", 97.0, 94.0, 96.0, 100.0)
	assert.False(t, ok)
}

func TestStopLimitSellAtLimit(t *testing.T) {
	// bid=96 < stop=100, bid=96 == limit=96 → executes at bid
	price, ok := CalculatePrice("STOP_LIMIT", "SELL", 98.0, 96.0, 96.0, 100.0)
	assert.True(t, ok)
	assert.Equal(t, 96.0, price)
}

// --- DetermineAONFillQty (S60) ---

func TestAONFillQty_NotAON(t *testing.T) {
	qty, ok := DetermineAONFillQty(false, 5, 10)
	assert.True(t, ok)
	assert.Equal(t, int32(5), qty)
}

func TestAONFillQty_FullFill(t *testing.T) {
	// AON order, remaining == total → must fill all at once
	qty, ok := DetermineAONFillQty(true, 10, 10)
	assert.True(t, ok)
	assert.Equal(t, int32(10), qty)
}

func TestAONFillQty_PartialFillBlocked(t *testing.T) {
	// AON order where a prior partial fill occurred — must wait
	_, ok := DetermineAONFillQty(true, 7, 10)
	assert.False(t, ok)
}
