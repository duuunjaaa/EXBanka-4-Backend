package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/api-gateway/middleware"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/portfolio"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"
)

// GetPortfolio handles GET /portfolio and GET /client/portfolio
func GetPortfolio(portfolioClient pb.PortfolioServiceClient, userType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := middleware.GetUserIDFromToken(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "could not extract identity from token"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("user-type", userType))

		resp, err := portfolioClient.GetPortfolio(ctx, &pb.GetPortfolioRequest{UserId: userID})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"portfolio": resp.Entries})
	}
}

// GetProfit handles GET /portfolio/profit and GET /client/portfolio/profit
func GetProfit(portfolioClient pb.PortfolioServiceClient, userType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := middleware.GetUserIDFromToken(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "could not extract identity from token"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("user-type", userType))

		resp, err := portfolioClient.GetProfit(ctx, &pb.GetProfitRequest{UserId: userID})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"totalProfit": resp.TotalProfit})
	}
}
