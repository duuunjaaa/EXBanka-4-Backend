package scheduler

import (
	"database/sql"
	"log"
	"time"

	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/securities-service/seeder"
)

// StartPriceRefresh launches a background goroutine that refreshes listing prices
// every interval using AlphaVantage. The first tick fires after one full interval.
func StartPriceRefresh(db *sql.DB, avKey string, interval time.Duration) {
	if avKey == "" {
		log.Println("price_refresh: ALPHAVANTAGE_API_KEY not set, price refresh disabled")
		return
	}
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			log.Println("price_refresh: running")
			refreshPrices(db, avKey)
			log.Println("price_refresh: done")
		}
	}()
	log.Printf("price_refresh: scheduled every %s", interval)
}

func refreshPrices(db *sql.DB, avKey string) {
	refreshStocks(db, avKey)
	refreshForex(db, avKey)
}

func refreshStocks(db *sql.DB, avKey string) {
	rows, err := db.Query(`SELECT id, ticker FROM listing WHERE type = 'STOCK'`)
	if err != nil {
		log.Printf("price_refresh: query stocks: %v", err)
		return
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("price_refresh: rows close: %v", err)
		}
	}()

	type stockRow struct {
		id     int64
		ticker string
	}
	var stocks []stockRow
	for rows.Next() {
		var s stockRow
		if err := rows.Scan(&s.id, &s.ticker); err == nil {
			stocks = append(stocks, s)
		}
	}

	for _, s := range stocks {
		q, err := seeder.FetchGlobalQuote(s.ticker, avKey)
		if err != nil {
			log.Printf("price_refresh: global quote %s: %v", s.ticker, err)
			continue
		}
		if q == nil {
			continue // rate-limit hit, skip
		}
		_, err = db.Exec(`
			UPDATE listing SET price=$2, ask=$3, bid=$4, change=$5, volume=$6, last_refresh=$7
			WHERE id=$1`,
			s.id, q.Price, q.Ask, q.Bid, q.Change, q.Volume, time.Now())
		if err != nil {
			log.Printf("price_refresh: update stock %s: %v", s.ticker, err)
		}
	}
}

func refreshForex(db *sql.DB, avKey string) {
	rows, err := db.Query(`
		SELECT l.id, fp.base_currency, fp.quote_currency
		FROM listing l
		JOIN listing_forex_pair fp ON fp.listing_id = l.id
		WHERE l.type = 'FOREX_PAIR'`)
	if err != nil {
		log.Printf("price_refresh: query forex: %v", err)
		return
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("price_refresh: rows close: %v", err)
		}
	}()

	type fxRow struct {
		id   int64
		from string
		to   string
	}
	var pairs []fxRow
	for rows.Next() {
		var f fxRow
		if err := rows.Scan(&f.id, &f.from, &f.to); err == nil {
			pairs = append(pairs, f)
		}
	}

	for _, p := range pairs {
		rate, err := seeder.FetchFXRate(p.from, p.to, avKey)
		if err != nil {
			log.Printf("price_refresh: fx rate %s/%s: %v", p.from, p.to, err)
			continue
		}
		if rate == nil {
			continue
		}
		_, err = db.Exec(`
			UPDATE listing SET price=$2, ask=$3, bid=$4, last_refresh=$5
			WHERE id=$1`,
			p.id, rate.Price, rate.Ask, rate.Bid, time.Now())
		if err != nil {
			log.Printf("price_refresh: update forex %s/%s: %v", p.from, p.to, err)
		}
	}
}
