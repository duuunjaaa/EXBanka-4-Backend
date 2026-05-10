package main

import (
	"log"
	"net"
	"os"

	carddb "github.com/RAF-SI-2025/EXBanka-4-Backend/services/card-service/db"
	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/card-service/handlers"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/card"
	"google.golang.org/grpc"
)

const grpcPort = ":50059"

func main() {
	cardDB, err := carddb.Connect(os.Getenv("CARD_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to card_db: %v", err)
	}
	defer func() {
		if err := cardDB.Close(); err != nil {
			log.Printf("card_db close: %v", err)
		}
	}()

	accountDB, err := carddb.Connect(os.Getenv("ACCOUNT_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to account_db: %v", err)
	}
	defer func() {
		if err := accountDB.Close(); err != nil {
			log.Printf("account_db close: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", grpcPort, err)
	}

	srv := grpc.NewServer()
	pb.RegisterCardServiceServer(srv, &handlers.CardServer{
		DB:        cardDB,
		AccountDB: accountDB,
	})

	log.Printf("card-service gRPC server listening on %s", grpcPort)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("gRPC serve error: %v", err)
	}
}
