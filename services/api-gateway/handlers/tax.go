package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/api-gateway/middleware"
	pb_client "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/client"
	pb_emp "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/employee"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/portfolio"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"
)

type taxListEntry struct {
	UserID   int64   `json:"userId"`
	FullName string  `json:"fullName"`
	Type     string  `json:"type"`
	DebtRSD  float64 `json:"debtRsd"`
}

// GetTaxList handles GET /tax — supervisor only.
// Returns all users with unpaid tax, enriched with full names.
func GetTaxList(portfolioClient pb.PortfolioServiceClient, employeeClient pb_emp.EmployeeServiceClient, clientClient pb_client.ClientServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		resp, err := portfolioClient.GetTaxList(ctx, &pb.GetTaxListRequest{
			UserTypeFilter: c.Query("type"),
			NameFilter:     c.Query("name"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		entries := make([]taxListEntry, 0, len(resp.Entries))
		for _, e := range resp.Entries {
			fullName := resolveName(c.Request.Context(), e.UserId, e.Type, employeeClient, clientClient)
			entries = append(entries, taxListEntry{
				UserID:   e.UserId,
				FullName: fullName,
				Type:     e.Type,
				DebtRSD:  e.DebtRsd,
			})
		}

		if nameFilter := c.Query("name"); nameFilter != "" {
			filtered := entries[:0]
			for _, e := range entries {
				if strings.Contains(strings.ToLower(e.FullName), strings.ToLower(nameFilter)) {
					filtered = append(filtered, e)
				}
			}
			entries = filtered
		}

		c.JSON(http.StatusOK, entries)
	}
}

// GetMyTax handles GET /tax/my (employee) and GET /client/tax/my (client).
func GetMyTax(portfolioClient pb.PortfolioServiceClient, userType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := middleware.GetUserIDFromToken(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "could not extract identity from token"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("user-type", userType))

		resp, err := portfolioClient.GetMyTax(ctx, &pb.GetMyTaxRequest{UserId: userID, UserType: userType})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"paidThisYear":    resp.PaidThisYear,
			"unpaidThisMonth": resp.UnpaidThisMonth,
		})
	}
}

// CollectTax handles POST /tax/collect — supervisor only.
// Triggers tax collection for all users with unpaid tax.
func CollectTax(portfolioClient pb.PortfolioServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		if _, err := portfolioClient.CollectTax(ctx, &pb.CollectTaxRequest{}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "tax collection completed"})
	}
}

// CollectTaxForUser handles POST /tax/collect/:userId — supervisor only.
func CollectTaxForUser(portfolioClient pb.PortfolioServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid userId"})
			return
		}

		userType := c.DefaultQuery("userType", "CLIENT")

		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		if _, err := portfolioClient.CollectTaxForUser(ctx, &pb.CollectTaxForUserRequest{
			UserId:   userID,
			UserType: userType,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("tax collection completed for user %d", userID)})
	}
}

func resolveName(ctx context.Context, userID int64, userType string, empClient pb_emp.EmployeeServiceClient, cliClient pb_client.ClientServiceClient) string {
	rCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if userType == "EMPLOYEE" {
		resp, err := empClient.GetEmployeeById(rCtx, &pb_emp.GetEmployeeByIdRequest{Id: userID})
		if err == nil && resp.Employee != nil {
			return fmt.Sprintf("%s %s", resp.Employee.FirstName, resp.Employee.LastName)
		}
	} else {
		resp, err := cliClient.GetClientById(rCtx, &pb_client.GetClientByIdRequest{Id: userID})
		if err == nil && resp.Client != nil {
			return fmt.Sprintf("%s %s", resp.Client.FirstName, resp.Client.LastName)
		}
	}
	return ""
}
