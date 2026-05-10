package main

import (
	"log"
	"net"
	"os"

	acdb "github.com/RAF-SI-2025/EXBanka-4-Backend/services/account-service/db"
	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/account-service/handlers"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/account"
	pb_email "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/email"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	database, err := acdb.Connect(os.Getenv("ACCOUNT_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to account_db: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("account_db close: %v", err)
		}
	}()

	clientDB, err := acdb.Connect(os.Getenv("CLIENT_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to client_db: %v", err)
	}
	defer func() {
		if err := clientDB.Close(); err != nil {
			log.Printf("client_db close: %v", err)
		}
	}()

	exchangeDB, err := acdb.Connect(os.Getenv("EXCHANGE_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to exchange_db: %v", err)
	}
	defer func() {
		if err := exchangeDB.Close(); err != nil {
			log.Printf("exchange_db close: %v", err)
		}
	}()

	emailConn, err := grpc.NewClient(os.Getenv("EMAIL_SERVICE_ADDR"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to email-service: %v", err)
	}
	defer func() {
		if err := emailConn.Close(); err != nil {
			log.Printf("email-service conn close: %v", err)
		}
	}()
	emailClient := pb_email.NewEmailServiceClient(emailConn)

	lis, err := net.Listen("tcp", ":50054")
	if err != nil {
		log.Fatalf("failed to listen on :50054: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAccountServiceServer(grpcServer, &handlers.AccountServer{
		DB:          database,
		ClientDB:    clientDB,
		ExchangeDB:  exchangeDB,
		EmailClient: emailClient,
	})

	log.Println("account-service gRPC server listening on :50054")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
