package scheduler

import (
	"context"
	"database/sql"
	"log"
	"time"

	pb_exchange "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/exchange"
	pb_securities "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/securities"
)

type PerformanceScheduler struct {
	DB               *sql.DB
	SecuritiesClient pb_securities.SecuritiesServiceClient
	ExchangeClient   pb_exchange.ExchangeServiceClient
}

func (s *PerformanceScheduler) convertToRSD(ctx context.Context, fromCurrency string, amount float64) float64 {
	if fromCurrency == "" || fromCurrency == "RSD" || s.ExchangeClient == nil {
		return amount
	}
	resp, err := s.ExchangeClient.PreviewConversion(ctx, &pb_exchange.PreviewConversionRequest{
		FromCurrency: fromCurrency,
		ToCurrency:   "RSD",
		Amount:       amount,
	})
	if err != nil {
		return amount
	}
	return resp.ToAmount
}

func (s *PerformanceScheduler) Start() {
	go s.loop()
}

func (s *PerformanceScheduler) loop() {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location())
		if !now.Before(next) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(time.Until(next))
		s.snapshotAll(context.Background())
	}
}

func (s *PerformanceScheduler) snapshotAll(ctx context.Context) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM investment_funds WHERE active = true`)
	if err != nil {
		log.Printf("performance-scheduler: list funds error: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var fundID int64
		if err := rows.Scan(&fundID); err != nil {
			continue
		}

		var liquidAssets float64
		if err := s.DB.QueryRowContext(ctx,
			`SELECT liquid_assets FROM investment_funds WHERE id = $1`, fundID,
		).Scan(&liquidAssets); err != nil {
			log.Printf("performance-scheduler: fetch liquid_assets for fund %d: %v", fundID, err)
			continue
		}

		var totalInvested float64
		_ = s.DB.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(total_invested_amount), 0) FROM client_fund_positions WHERE fund_id = $1`, fundID,
		).Scan(&totalInvested)

		var portfolioValue float64
		pRows, pErr := s.DB.QueryContext(ctx,
			`SELECT listing_id, quantity FROM fund_portfolio_positions WHERE fund_id = $1 AND quantity > 0`, fundID)
		if pErr == nil {
			for pRows.Next() {
				var listingID int64
				var qty float64
				if err := pRows.Scan(&listingID, &qty); err != nil {
					continue
				}
				resp, err := s.SecuritiesClient.GetListingById(ctx, &pb_securities.GetListingByIdRequest{Id: listingID})
				if err != nil || resp.Summary == nil {
					continue
				}
				priceRSD := s.convertToRSD(ctx, resp.Summary.Currency, resp.Summary.Price)
				portfolioValue += qty * priceRSD
			}
			_ = pRows.Close()
		}
		fundValue := liquidAssets + portfolioValue
		profit := fundValue - totalInvested

		if _, err := s.DB.ExecContext(ctx,
			`INSERT INTO fund_performance_history (fund_id, date, fund_value, profit)
			 VALUES ($1, CURRENT_DATE, $2, $3)
			 ON CONFLICT (fund_id, date) DO UPDATE SET fund_value = $2, profit = $3`,
			fundID, fundValue, profit,
		); err != nil {
			log.Printf("performance-scheduler: upsert fund %d: %v", fundID, err)
		}
	}
}
