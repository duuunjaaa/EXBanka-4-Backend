package tax

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- CalculateTax (S78, S81) ---

func TestCalculateTax_Profit(t *testing.T) {
	// S78: tax is 15% of realized gain
	assert.InDelta(t, 1500.0, CalculateTax(10000), 0.001)
}

func TestCalculateTax_ZeroProfit(t *testing.T) {
	// S81: no tax when profit is exactly zero
	assert.Equal(t, 0.0, CalculateTax(0))
}

func TestCalculateTax_Loss(t *testing.T) {
	// S81: no tax on a realized loss
	assert.Equal(t, 0.0, CalculateTax(-500))
}

func TestCalculateTax_SmallProfit(t *testing.T) {
	assert.InDelta(t, 15.0, CalculateTax(100), 0.001)
}

// --- ConvertToRSD (S80) ---

func TestConvertToRSD_USD(t *testing.T) {
	// S80: 100 USD * 117 RSD/USD = 11700 RSD
	assert.Equal(t, 11700.0, ConvertToRSD(100, 117))
}

func TestConvertToRSD_EUR(t *testing.T) {
	assert.InDelta(t, 11750.0, ConvertToRSD(100, 117.50), 0.001)
}

func TestCalculateTax_AfterRSDConversion(t *testing.T) {
	// S80: profit in foreign currency, convert to RSD then tax
	profitRSD := ConvertToRSD(500, 117) // 500 USD → 58500 RSD
	tax := CalculateTax(profitRSD)
	assert.InDelta(t, 8775.0, tax, 0.001) // 15% of 58500
}

// --- MonthlyTaxTotal (S78) ---

func TestMonthlyTaxTotal_MixedGainsAndLoss(t *testing.T) {
	// S78: monthly calculation sums tax across all realized profits; losses contribute 0
	profits := []float64{1000, 2000, 500, -300}
	total := MonthlyTaxTotal(profits)
	// taxable: 1000+2000+500 = 3500 → tax = 525; loss (-300) → 0
	assert.InDelta(t, 525.0, total, 0.001)
}

func TestMonthlyTaxTotal_AllLosses(t *testing.T) {
	// S81: no profits in the month → zero tax
	assert.Equal(t, 0.0, MonthlyTaxTotal([]float64{-100, -200, -50}))
}

func TestMonthlyTaxTotal_Empty(t *testing.T) {
	assert.Equal(t, 0.0, MonthlyTaxTotal(nil))
}
