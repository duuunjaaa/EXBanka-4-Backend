package handlers

import (
	"encoding/json"
	"net/http"
	"os"

	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/payment"
	"github.com/gin-gonic/gin"
)

type interbankTransactionId struct {
	RoutingNumber int32  `json:"routingNumber"`
	ID            string `json:"id"`
}

type interbankIdempotenceKey struct {
	RoutingNumber       int32  `json:"routingNumber"`
	LocallyGeneratedKey string `json:"locallyGeneratedKey"`
}

type interbankEnvelope struct {
	IdempotenceKey interbankIdempotenceKey `json:"idempotenceKey"`
	MessageType    string                  `json:"messageType"`
	Message        json.RawMessage         `json:"message"`
}

type newTxMessage struct {
	TransactionId  interbankTransactionId `json:"transactionId"`
	Postings       []interbankPosting     `json:"postings"`
	PaymentCode    string                 `json:"paymentCode"`
	PaymentPurpose string                 `json:"paymentPurpose"`
}

type interbankPosting struct {
	AccountType string  `json:"accountType"`
	AccountNum  string  `json:"accountNum"`
	Amount      float64 `json:"amount"`
	AssetType   string  `json:"assetType"`
	Currency    string  `json:"currency"`
}

type commitRollbackMessage struct {
	TransactionId interbankTransactionId `json:"transactionId"`
}

// InterbankHandler handles POST /interbank for all 2PC message types.
// Authenticated via X-Api-Key header matched against OWN_INTERBANK_API_KEY env var.
func InterbankHandler(paymentClient pb.PaymentServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := os.Getenv("OWN_INTERBANK_API_KEY")
		if apiKey == "" || c.GetHeader("X-Api-Key") != apiKey {
			c.Status(http.StatusUnauthorized)
			return
		}

		var env interbankEnvelope
		if err := c.ShouldBindJSON(&env); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		switch env.MessageType {
		case "NEW_TX":
			handleNewTx(c, paymentClient, env)
		case "COMMIT_TX":
			handleCommitRollbackTx(c, paymentClient, env, false)
		case "ROLLBACK_TX":
			handleCommitRollbackTx(c, paymentClient, env, true)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown messageType: " + env.MessageType})
		}
	}
}

func handleNewTx(c *gin.Context, client pb.PaymentServiceClient, env interbankEnvelope) {
	var msg newTxMessage
	if err := json.Unmarshal(env.Message, &msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid NEW_TX message"})
		return
	}

	pbPostings := make([]*pb.InterbankPosting, len(msg.Postings))
	for i, p := range msg.Postings {
		pbPostings[i] = &pb.InterbankPosting{
			AccountType: p.AccountType,
			AccountNum:  p.AccountNum,
			Amount:      p.Amount,
			AssetType:   p.AssetType,
			Currency:    p.Currency,
		}
	}

	resp, err := client.PrepareInterbankPayment(c.Request.Context(), &pb.PrepareInterbankPaymentRequest{
		IdempotenceKey: &pb.InterbankIdempotenceKey{
			RoutingNumber:       env.IdempotenceKey.RoutingNumber,
			LocallyGeneratedKey: env.IdempotenceKey.LocallyGeneratedKey,
		},
		TransactionId: &pb.InterbankTransactionId{
			RoutingNumber: msg.TransactionId.RoutingNumber,
			Id:            msg.TransactionId.ID,
		},
		Postings:       pbPostings,
		PaymentCode:    msg.PaymentCode,
		PaymentPurpose: msg.PaymentPurpose,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type reason struct {
		Reason string `json:"reason"`
	}
	reasons := make([]reason, len(resp.Reasons))
	for i, r := range resp.Reasons {
		reasons[i] = reason{Reason: r.Reason}
	}
	c.JSON(http.StatusOK, gin.H{
		"vote":    resp.Vote,
		"reasons": reasons,
	})
}

func handleCommitRollbackTx(c *gin.Context, client pb.PaymentServiceClient, env interbankEnvelope, isRollback bool) {
	var msg commitRollbackMessage
	if err := json.Unmarshal(env.Message, &msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid COMMIT_TX/ROLLBACK_TX message"})
		return
	}

	req := &pb.CommitRollbackInterbankRequest{
		TransactionId: &pb.InterbankTransactionId{
			RoutingNumber: msg.TransactionId.RoutingNumber,
			Id:            msg.TransactionId.ID,
		},
	}

	if isRollback {
		_, err := client.RollbackInterbankPayment(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		_, err := client.CommitInterbankPayment(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.Status(http.StatusNoContent)
}
