package tax

const taxRate = 0.15 // Serbian capital gains tax: 15%

// CalculateTax returns the tax owed on a realized capital gain.
// Returns 0 for zero or negative profit (loss — no tax due).
func CalculateTax(profit float64) float64 {
	if profit <= 0 {
		return 0
	}
	return profit * taxRate
}

// ConvertToRSD converts an amount from a foreign currency to RSD.
// rsdRate is 1 unit of foreign currency expressed in RSD (e.g. 117 for USD).
func ConvertToRSD(amount, rsdRate float64) float64 {
	return amount * rsdRate
}

// MonthlyTaxTotal returns the total tax due for a slice of realized profits.
// Each profit is taxed independently; losses do not offset gains.
func MonthlyTaxTotal(profits []float64) float64 {
	var total float64
	for _, p := range profits {
		total += CalculateTax(p)
	}
	return total
}
