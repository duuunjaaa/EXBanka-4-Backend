package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"os"
	"time"

	portfoliodb "github.com/RAF-SI-2025/EXBanka-4-Backend/services/portfolio-service/db"
	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/portfolio-service/handlers"
	taxcollector "github.com/RAF-SI-2025/EXBanka-4-Backend/services/portfolio-service/tax"
	pb_ex "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/exchange"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/portfolio"
	pb_sec "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/securities"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const grpcPort = ":50062"

func main() {
	db, err := portfoliodb.Connect(os.Getenv("PORTFOLIO_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to portfolio_db: %v", err)
	}
	defer func() { _ = db.Close() }()

	accountDB, err := portfoliodb.Connect(os.Getenv("ACCOUNT_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to account_db: %v", err)
	}
	defer func() { _ = accountDB.Close() }()

	exchangeDB, err := portfoliodb.Connect(os.Getenv("EXCHANGE_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to exchange_db: %v", err)
	}
	defer func() { _ = exchangeDB.Close() }()

	securitiesDB, err := portfoliodb.Connect(os.Getenv("SECURITIES_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to securities_db: %v", err)
	}
	defer func() { _ = securitiesDB.Close() }()

	secConn, err := grpc.NewClient(os.Getenv("SECURITIES_SERVICE_ADDR"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to securities-service: %v", err)
	}
	defer func() { _ = secConn.Close() }()

	exConn, err := grpc.NewClient(os.Getenv("EXCHANGE_SERVICE_ADDR"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to exchange-service: %v", err)
	}
	defer func() { _ = exConn.Close() }()

	securitiesClient := pb_sec.NewSecuritiesServiceClient(secConn)
	exchangeClient := pb_ex.NewExchangeServiceClient(exConn)

	go runMonthlyTaxJob(db, accountDB, exchangeDB, exchangeClient)

	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", grpcPort, err)
	}

	srv := grpc.NewServer()
	pb.RegisterPortfolioServiceServer(srv, &handlers.PortfolioServer{
		DB:               db,
		AccountDB:        accountDB,
		ExchangeDB:       exchangeDB,
		SecuritiesDB:     securitiesDB,
		SecuritiesClient: securitiesClient,
		ExchangeClient:   exchangeClient,
	})

	log.Printf("portfolio-service gRPC server listening on %s", grpcPort)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("gRPC serve error: %v", err)
	}
}

// runMonthlyTaxJob sleeps until the last day of each month at 23:59 and collects all unpaid tax.
func runMonthlyTaxJob(portfolioDB, accountDB, exchangeDB *sql.DB, exchangeClient pb_ex.ExchangeServiceClient) {
	for {
		now := time.Now()
		// Last day of current month at 23:59
		firstOfNext := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
		lastDay := time.Date(firstOfNext.Year(), firstOfNext.Month(), firstOfNext.Day()-1, 23, 59, 0, 0, now.Location())
		if !now.Before(lastDay) {
			// Already past this month's window; schedule for next month
			firstOfNextNext := time.Date(now.Year(), now.Month()+2, 1, 0, 0, 0, 0, now.Location())
			lastDay = time.Date(firstOfNextNext.Year(), firstOfNextNext.Month(), firstOfNextNext.Day()-1, 23, 59, 0, 0, now.Location())
		}
		time.Sleep(time.Until(lastDay))

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if err := taxcollector.CollectUnpaid(ctx, portfolioDB, accountDB, exchangeDB, exchangeClient, 0, ""); err != nil {
			log.Printf("monthly tax job error: %v", err)
		} else {
			log.Printf("monthly tax collection completed")
		}
		cancel()
	}
}
