package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/portfolio-service/repository"
	taxcalc "github.com/RAF-SI-2025/EXBanka-4-Backend/services/portfolio-service/tax"
	pb_ex "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/exchange"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/portfolio"
	pb_sec "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/securities"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// SecurityPriceFetcher is the subset of SecuritiesServiceClient we need.
type SecurityPriceFetcher interface {
	GetListingById(ctx context.Context, in *pb_sec.GetListingByIdRequest, opts ...grpc.CallOption) (*pb_sec.GetListingByIdResponse, error)
}

type PortfolioServer struct {
	pb.UnimplementedPortfolioServiceServer
	DB               *sql.DB
	AccountDB        *sql.DB
	ExchangeDB       *sql.DB
	SecuritiesDB     *sql.DB
	SecuritiesClient SecurityPriceFetcher
	ExchangeClient   pb_ex.ExchangeServiceClient
}

func (s *PortfolioServer) UpdateHolding(ctx context.Context, req *pb.UpdateHoldingRequest) (*pb.UpdateHoldingResponse, error) {
	if req.Quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "quantity must be positive")
	}
	if req.Direction != "BUY" && req.Direction != "SELL" {
		return nil, status.Error(codes.InvalidArgument, "direction must be BUY or SELL")
	}

	buyPrice, err := repository.UpsertHolding(ctx, s.DB, req.UserId, req.UserType, req.ListingId, req.AccountId, req.Quantity, req.Price, req.Direction)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "upsert holding: %v", err)
	}

	if req.Direction == "SELL" && req.AssetType == "STOCK" && buyPrice > 0 {
		profit := (req.Price - buyPrice) * float64(req.Quantity)
		profitRSD, err := s.convertToRSD(ctx, profit, req.ListingId)
		if err != nil {
			log.Printf("tax: currency conversion for listing %d: %v — using raw value", req.ListingId, err)
			profitRSD = profit
		}
		if taxOwed := taxcalc.CalculateTax(profitRSD); taxOwed > 0 {
			now := time.Now()
			_ = repository.InsertTaxRecord(ctx, s.DB, req.UserId, req.UserType, taxOwed, int(now.Month()), now.Year())
		}
	}

	return &pb.UpdateHoldingResponse{}, nil
}

func userTypeFromCtx(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("user-type"); len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

func (s *PortfolioServer) GetPortfolio(ctx context.Context, req *pb.GetPortfolioRequest) (*pb.GetPortfolioResponse, error) {
	userType := req.UserType
	if userType == "" {
		userType = userTypeFromCtx(ctx)
	}
	entries, err := repository.GetHoldings(ctx, s.DB, req.UserId, userType)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get holdings: %v", err)
	}

	pbEntries := make([]*pb.PortfolioEntry, 0, len(entries))
	for _, e := range entries {
		entry := &pb.PortfolioEntry{
			Id:           e.ID,
			ListingId:    e.ListingID,
			Amount:       e.Amount,
			BuyPrice:     e.BuyPrice,
			LastModified: e.LastModified.Format("2006-01-02T15:04:05"),
			IsPublic:     e.IsPublic,
			PublicAmount: e.PublicAmount,
			AccountId:    e.AccountID,
		}

		if s.SecuritiesClient != nil {
			resp, secErr := s.SecuritiesClient.GetListingById(ctx, &pb_sec.GetListingByIdRequest{Id: e.ListingID})
			if secErr == nil && resp.Summary != nil {
				entry.Ticker = resp.Summary.Ticker
				entry.AssetType = resp.Summary.Type
				entry.Price = resp.Summary.Price
				entry.Profit = (resp.Summary.Price - e.BuyPrice) * float64(e.Amount)
			}
		}

		pbEntries = append(pbEntries, entry)
	}
	return &pb.GetPortfolioResponse{Entries: pbEntries}, nil
}

func (s *PortfolioServer) GetProfit(ctx context.Context, req *pb.GetProfitRequest) (*pb.GetProfitResponse, error) {
	userType := req.UserType
	if userType == "" {
		userType = userTypeFromCtx(ctx)
	}
	entries, err := repository.GetHoldings(ctx, s.DB, req.UserId, userType)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get holdings: %v", err)
	}

	var totalProfit float64
	if s.SecuritiesClient != nil {
		for _, e := range entries {
			resp, secErr := s.SecuritiesClient.GetListingById(ctx, &pb_sec.GetListingByIdRequest{Id: e.ListingID})
			if secErr == nil && resp.Summary != nil {
				totalProfit += (resp.Summary.Price - e.BuyPrice) * float64(e.Amount)
			}
		}
	}

	return &pb.GetProfitResponse{TotalProfit: totalProfit}, nil
}

func (s *PortfolioServer) SetPublicAmount(_ context.Context, _ *pb.SetPublicAmountRequest) (*pb.SetPublicAmountResponse, error) {
	return nil, status.Error(codes.Unimplemented, "implemented in issue #147")
}

func (s *PortfolioServer) SetPublicMode(ctx context.Context, req *pb.SetPublicModeRequest) (*pb.SetPublicModeResponse, error) {
	if req.Ticker == "" {
		return nil, status.Error(codes.InvalidArgument, "ticker is required")
	}

	var listingID int64
	err := s.SecuritiesDB.QueryRowContext(ctx, `SELECT id FROM listing WHERE ticker = $1`, req.Ticker).Scan(&listingID)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "listing not found for ticker %s", req.Ticker)
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resolve ticker: %v", err)
	}

	// EMPLOYEE portfolios are stored under user_id=0 (shared bank portfolio).
	userID := req.UserId
	if req.UserType == "EMPLOYEE" {
		userID = 0
	}

	res, err := s.DB.ExecContext(ctx, `
		UPDATE portfolio_entry
		SET is_public = $1,
		    public_amount = CASE WHEN $1 THEN amount ELSE 0 END,
		    last_modified = NOW()
		WHERE user_id = $2 AND user_type = $3 AND listing_id = $4`,
		req.IsPublic, userID, req.UserType, listingID,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update public mode: %v", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, status.Error(codes.NotFound, "position not found")
	}

	return &pb.SetPublicModeResponse{
		Ticker:   req.Ticker,
		IsPublic: req.IsPublic,
	}, nil
}

func (s *PortfolioServer) GetMyTax(ctx context.Context, req *pb.GetMyTaxRequest) (*pb.GetMyTaxResponse, error) {
	userType := req.UserType
	if userType == "" {
		userType = userTypeFromCtx(ctx)
	}
	now := time.Now()
	paid, unpaid, err := repository.GetMyTax(ctx, s.DB, req.UserId, userType, now.Year(), int(now.Month()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get my tax: %v", err)
	}
	return &pb.GetMyTaxResponse{PaidThisYear: paid, UnpaidThisMonth: unpaid}, nil
}

func (s *PortfolioServer) GetTaxList(ctx context.Context, req *pb.GetTaxListRequest) (*pb.GetTaxListResponse, error) {
	debts, err := repository.GetTaxDebtList(ctx, s.DB, req.UserTypeFilter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get tax list: %v", err)
	}
	entries := make([]*pb.TaxDebtEntry, 0, len(debts))
	for _, d := range debts {
		entries = append(entries, &pb.TaxDebtEntry{
			UserId:  d.UserID,
			Type:    d.UserType,
			DebtRsd: d.DebtRSD,
		})
	}
	return &pb.GetTaxListResponse{Entries: entries}, nil
}

func (s *PortfolioServer) CollectTax(ctx context.Context, _ *pb.CollectTaxRequest) (*pb.CollectTaxResponse, error) {
	if err := taxcalc.CollectUnpaid(ctx, s.DB, s.AccountDB, s.ExchangeDB, s.ExchangeClient, 0, ""); err != nil {
		return nil, status.Errorf(codes.Internal, "collect tax: %v", err)
	}
	return &pb.CollectTaxResponse{}, nil
}

func (s *PortfolioServer) CollectTaxForUser(ctx context.Context, req *pb.CollectTaxForUserRequest) (*pb.CollectTaxForUserResponse, error) {
	userType := req.UserType
	if userType == "" {
		userType = userTypeFromCtx(ctx)
	}
	if err := taxcalc.CollectUnpaid(ctx, s.DB, s.AccountDB, s.ExchangeDB, s.ExchangeClient, req.UserId, userType); err != nil {
		return nil, status.Errorf(codes.Internal, "collect tax for user: %v", err)
	}
	return &pb.CollectTaxForUserResponse{}, nil
}

// convertToRSD converts amount (denominated in the listing's exchange currency) to RSD
// using the middle rate from the exchange service. Returns amount unchanged if currency is RSD.
func (s *PortfolioServer) convertToRSD(ctx context.Context, amount float64, listingID int64) (float64, error) {
	var currency string
	if err := s.SecuritiesDB.QueryRowContext(ctx, `
		SELECT e.currency
		FROM listing l
		JOIN stock_exchanges e ON l.exchange_id = e.id
		WHERE l.id = $1`, listingID,
	).Scan(&currency); err != nil {
		return 0, fmt.Errorf("listing currency lookup: %w", err)
	}

	if currency == "RSD" {
		return amount, nil
	}

	resp, err := s.ExchangeClient.GetExchangeRates(ctx, &pb_ex.GetExchangeRatesRequest{})
	if err != nil {
		return 0, fmt.Errorf("fetch exchange rates: %w", err)
	}

	for _, r := range resp.Rates {
		if r.CurrencyCode == currency {
			return taxcalc.ConvertToRSD(amount, r.MiddleRate), nil
		}
	}

	return 0, fmt.Errorf("no middle rate found for currency %s", currency)
}
