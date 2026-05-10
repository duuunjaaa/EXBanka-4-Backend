package grpc

import (
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/portfolio"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewPortfolioClient(addr string) (pb.PortfolioServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return pb.NewPortfolioServiceClient(conn), conn, nil
}
