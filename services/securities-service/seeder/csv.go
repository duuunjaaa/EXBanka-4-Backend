package seeder

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
)

// ExchangeRow holds one row from exchange_1.csv.
type ExchangeRow struct {
	Name      string
	Acronym   string
	MICCode   string
	Country   string
	Currency  string
	Timezone  string
	OpenTime  string
	CloseTime string
}

// FutureRow holds one row from future_data.csv.
type FutureRow struct {
	ContractName      string
	ContractSize      float64
	ContractUnit      string
	MaintenanceMargin float64
	FutureType        string
}

// ParseExchanges parses exchange_1.csv bytes into ExchangeRow slice.
// Expected header: Exchange Name,Exchange Acronym,Exchange MIC Code,Country,Currency,Time Zone,Open Time,Close Time
func ParseExchanges(data []byte) ([]ExchangeRow, error) {
	r := csv.NewReader(bytes.NewReader(data))
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read exchange CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("exchange CSV has no data rows")
	}
	rows := make([]ExchangeRow, 0, len(records)-1)
	for i, rec := range records[1:] {
		if len(rec) < 8 {
			return nil, fmt.Errorf("exchange CSV row %d: expected 8 columns, got %d", i+2, len(rec))
		}
		rows = append(rows, ExchangeRow{
			Name:      rec[0],
			Acronym:   rec[1],
			MICCode:   rec[2],
			Country:   rec[3],
			Currency:  rec[4],
			Timezone:  rec[5],
			OpenTime:  rec[6],
			CloseTime: rec[7],
		})
	}
	return rows, nil
}

// ParseFutures parses future_data.csv bytes into FutureRow slice.
// Expected header: contract_name,contract_size,contract_unit,maintenance_margin,type
func ParseFutures(data []byte) ([]FutureRow, error) {
	r := csv.NewReader(bytes.NewReader(data))
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read futures CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("futures CSV has no data rows")
	}
	rows := make([]FutureRow, 0, len(records)-1)
	for i, rec := range records[1:] {
		if len(rec) < 5 {
			return nil, fmt.Errorf("futures CSV row %d: expected 5 columns, got %d", i+2, len(rec))
		}
		size, err := strconv.ParseFloat(rec[1], 64)
		if err != nil {
			return nil, fmt.Errorf("futures CSV row %d: bad contract_size %q: %w", i+2, rec[1], err)
		}
		margin, err := strconv.ParseFloat(rec[3], 64)
		if err != nil {
			return nil, fmt.Errorf("futures CSV row %d: bad maintenance_margin %q: %w", i+2, rec[3], err)
		}
		rows = append(rows, FutureRow{
			ContractName:      rec[0],
			ContractSize:      size,
			ContractUnit:      rec[2],
			MaintenanceMargin: margin,
			FutureType:        rec[4],
		})
	}
	return rows, nil
}
