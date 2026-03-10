package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/exbanka/backend/services/auth-service/handlers"
	pb_auth "github.com/exbanka/backend/shared/pb/auth"
	pb_emp "github.com/exbanka/backend/shared/pb/employee"
)

func main() {
	empConn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to employee-service: %v", err)
	}
	defer empConn.Close()

	employeeClient := pb_emp.NewEmployeeServiceClient(empConn)

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb_auth.RegisterAuthServiceServer(s, &handlers.AuthServer{EmployeeClient: employeeClient})

	log.Println("auth-service listening on :50052")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
