package main

import (
	"log"
	"net"
	"os"

	funddb "github.com/RAF-SI-2025/EXBanka-4-Backend/services/fund-service/db"
	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/fund-service/handlers"
	pb_account "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/account"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/fund"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const grpcPort = ":50064"

func main() {
	fundDB, err := funddb.Connect(os.Getenv("FUND_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to fund_db: %v", err)
	}
	defer func() { _ = fundDB.Close() }()

	accountDB, err := funddb.Connect(os.Getenv("ACCOUNT_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to account_db: %v", err)
	}
	defer func() { _ = accountDB.Close() }()

	employeeDB, err := funddb.Connect(os.Getenv("EMPLOYEE_DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to employee_db: %v", err)
	}
	defer func() { _ = employeeDB.Close() }()

	accountConn, err := grpc.NewClient(os.Getenv("ACCOUNT_SERVICE_ADDR"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to account-service: %v", err)
	}
	defer func() { _ = accountConn.Close() }()
	accountClient := pb_account.NewAccountServiceClient(accountConn)

	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", grpcPort, err)
	}

	srv := grpc.NewServer()
	pb.RegisterFundServiceServer(srv, &handlers.FundServer{
		DB:            fundDB,
		AccountDB:     accountDB,
		EmployeeDB:    employeeDB,
		AccountClient: accountClient,
	})

	log.Printf("fund-service gRPC server listening on %s", grpcPort)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("gRPC serve error: %v", err)
	}
}
