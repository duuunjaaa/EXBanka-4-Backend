package main

import (
	"database/sql"
	"log"
	"net"
	"os"

	exdb "github.com/RAF-SI-2025/EXBanka-4-Backend/services/exchange-service/db"
	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/exchange-service/handlers"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/exchange"
	"google.golang.org/grpc"
)

const grpcPort = ":50057"

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS daily_exchange_rates (
			id            BIGSERIAL PRIMARY KEY,
			currency_code VARCHAR NOT NULL REFERENCES currencies(code),
			buying_rate   NUMERIC(20, 6) NOT NULL,
			selling_rate  NUMERIC(20, 6) NOT NULL,
			middle_rate   NUMERIC(20, 6) NOT NULL,
			date          DATE NOT NULL DEFAULT CURRENT_DATE,
			UNIQUE (currency_code, date)
		);
		CREATE TABLE IF NOT EXISTS exchange_transactions (
			id            BIGSERIAL PRIMARY KEY,
			client_id     BIGINT NOT NULL,
			from_account  VARCHAR NOT NULL,
			to_account    VARCHAR NOT NULL,
			from_currency VARCHAR NOT NULL,
			to_currency   VARCHAR NOT NULL,
			from_amount   NUMERIC(20, 2) NOT NULL,
			to_amount     NUMERIC(20, 2) NOT NULL,
			rate          NUMERIC(20, 6) NOT NULL,
			commission    NUMERIC(20, 2) NOT NULL,
			timestamp     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			status        VARCHAR NOT NULL DEFAULT 'COMPLETED'
		);
	`)
	return err
}

func main() {
	exchangeDB, err := exdb.Connect(os.Getenv("EXCHANGE_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to exchange_db: %v", err)
	}
	defer func() {
		if err := exchangeDB.Close(); err != nil {
			log.Printf("exchange_db close: %v", err)
		}
	}()

	accountDB, err := exdb.Connect(os.Getenv("ACCOUNT_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to account_db: %v", err)
	}
	defer func() {
		if err := accountDB.Close(); err != nil {
			log.Printf("account_db close: %v", err)
		}
	}()

	if err := migrate(exchangeDB); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", grpcPort, err)
	}

	srv := grpc.NewServer()
	pb.RegisterExchangeServiceServer(srv, &handlers.ExchangeServer{
		DB:        exchangeDB,
		AccountDB: accountDB,
	})

	log.Printf("exchange-service gRPC server listening on %s", grpcPort)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("gRPC serve error: %v", err)
	}
}
