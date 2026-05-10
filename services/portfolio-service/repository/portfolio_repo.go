package repository

import (
	"context"
	"database/sql"

	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/portfolio-service/models"
)

// sharedUserID returns 0 for EMPLOYEE-type users so all actuaries share one portfolio.
func sharedUserID(userID int64, userType string) int64 {
	if userType == "EMPLOYEE" {
		return 0
	}
	return userID
}

// UpsertHolding updates portfolio holdings on each order fill.
// BUY: creates or updates entry with weighted average buy price.
// SELL: decrements amount; deletes entry if amount reaches zero.
// Returns the buy_price at the time of the operation (non-zero only on SELL, used by the caller to calculate tax).
func UpsertHolding(ctx context.Context, db *sql.DB, userID int64, userType string, listingID, accountID int64, qty int32, price float64, direction string) (buyPrice float64, err error) {
	uid := sharedUserID(userID, userType)
	if direction == "BUY" {
		_, err = db.ExecContext(ctx, `
			INSERT INTO portfolio_entry (user_id, user_type, listing_id, amount, buy_price, account_id, last_modified)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())
			ON CONFLICT (user_id, user_type, listing_id) DO UPDATE SET
				buy_price     = (portfolio_entry.amount * portfolio_entry.buy_price + $4 * $5) / (portfolio_entry.amount + $4),
				amount        = portfolio_entry.amount + $4,
				last_modified = NOW()`,
			uid, userType, listingID, qty, price, accountID,
		)
		return 0, err
	}

	// SELL: read current buy_price before decrementing
	row := db.QueryRowContext(ctx,
		`SELECT buy_price FROM portfolio_entry WHERE user_id = $1 AND user_type = $2 AND listing_id = $3`,
		uid, userType, listingID,
	)
	if scanErr := row.Scan(&buyPrice); scanErr != nil {
		buyPrice = 0
	}

	_, err = db.ExecContext(ctx, `
		UPDATE portfolio_entry
		SET amount = amount - $1, last_modified = NOW()
		WHERE user_id = $2 AND user_type = $3 AND listing_id = $4`,
		qty, uid, userType, listingID,
	)
	if err != nil {
		return buyPrice, err
	}

	_, err = db.ExecContext(ctx, `
		DELETE FROM portfolio_entry
		WHERE user_id = $1 AND user_type = $2 AND listing_id = $3 AND amount <= 0`,
		uid, userType, listingID,
	)
	return buyPrice, err
}

// GetHoldings returns all portfolio entries for a user filtered by user type.
func GetHoldings(ctx context.Context, db *sql.DB, userID int64, userType string) ([]models.PortfolioEntry, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, user_type, listing_id, amount, buy_price, last_modified, is_public, public_amount, account_id
		FROM portfolio_entry
		WHERE user_id = $1 AND user_type = $2 AND amount > 0
		ORDER BY last_modified DESC`,
		sharedUserID(userID, userType), userType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.PortfolioEntry
	for rows.Next() {
		var e models.PortfolioEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.UserType, &e.ListingID, &e.Amount, &e.BuyPrice, &e.LastModified, &e.IsPublic, &e.PublicAmount, &e.AccountID); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
