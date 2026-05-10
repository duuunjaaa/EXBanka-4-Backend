package main

import (
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authdb "github.com/RAF-SI-2025/EXBanka-4-Backend/services/auth-service/db"
	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/auth-service/handlers"
	pb_auth "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/auth"
	pb_client "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/client"
	pb_email "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/email"
	pb_emp "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/employee"
)

func main() {
	database, err := authdb.Connect(os.Getenv("DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to auth-db: %v", err)
	}
	defer func() { _ = database.Close() }()

	clientConn, err := grpc.NewClient(os.Getenv("CLIENT_SERVICE_ADDR"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to client-service: %v", err)
	}
	defer func() { _ = clientConn.Close() }()

	clientClient := pb_client.NewClientServiceClient(clientConn)

	empConn, err := grpc.NewClient(os.Getenv("EMPLOYEE_SERVICE_ADDR"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to employee-service: %v", err)
	}
	defer func() { _ = empConn.Close() }()

	employeeClient := pb_emp.NewEmployeeServiceClient(empConn)

	emailConn, err := grpc.NewClient(os.Getenv("EMAIL_SERVICE_ADDR"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to email-service: %v", err)
	}
	defer func() { _ = emailConn.Close() }()

	emailClient := pb_email.NewEmailServiceClient(emailConn)

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb_auth.RegisterAuthServiceServer(s, &handlers.AuthServer{
		DB:             database,
		EmployeeClient: employeeClient,
		EmailClient:    emailClient,
		ClientClient:   clientClient,
	})

	log.Println("auth-service listening on :50052")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
