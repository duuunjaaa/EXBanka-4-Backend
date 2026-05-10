package main

import (
	_ "embed"
	"log"
	"net"
	"os"
	"time"
	_ "time/tzdata"

	secdb "github.com/RAF-SI-2025/EXBanka-4-Backend/services/securities-service/db"
	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/securities-service/handlers"
	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/securities-service/scheduler"
	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/securities-service/seeder"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/securities"
	"google.golang.org/grpc"
)

//go:embed assets/exchange_1.csv
var exchangeCSV []byte

//go:embed assets/future_data.csv
var futureDataCSV []byte

const grpcPort = ":50060"

func main() {
	securitiesDB, err := secdb.Connect(os.Getenv("SECURITIES_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to securities_db: %v", err)
	}
	defer func() {
		if err := securitiesDB.Close(); err != nil {
			log.Printf("securities_db close: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", grpcPort, err)
	}

	srv := grpc.NewServer()
	pb.RegisterSecuritiesServiceServer(srv, &handlers.SecuritiesServer{
		DB: securitiesDB,
	})

	// Seed data on startup (runs in background, does not block gRPC server).
	go seeder.Seed(securitiesDB, os.Getenv("ALPACA_API_KEY"), os.Getenv("ALPACA_API_SECRET_KEY"), os.Getenv("ALPHAVANTAGE_API_KEY"), exchangeCSV, futureDataCSV)

	// Refresh prices every 15 minutes.
	scheduler.StartPriceRefresh(securitiesDB, os.Getenv("ALPHAVANTAGE_API_KEY"), 15*time.Minute)

	// Snapshot daily prices + reset actuary limits at 23:59 every day.
	scheduler.ScheduleEOD(securitiesDB, os.Getenv("EMPLOYEE_SERVICE_ADDR"))

	// Simulate market price fluctuations every minute when test mode is enabled.
	scheduler.StartPriceSimulation(securitiesDB)

	log.Printf("securities-service gRPC server listening on %s", grpcPort)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("gRPC serve error: %v", err)
	}
}
