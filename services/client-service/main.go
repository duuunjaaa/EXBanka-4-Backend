package main

import (
	"log"
	"net"
	"os"

	clientdb "github.com/RAF-SI-2025/EXBanka-4-Backend/services/client-service/db"
	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/client-service/handlers"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/client"
	"google.golang.org/grpc"
)

func main() {
	database, err := clientdb.Connect(os.Getenv("DB_URL"))
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("client_db close: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", ":50056")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterClientServiceServer(s, &handlers.ClientServer{DB: database})
	log.Println("client-service listening on :50056")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
