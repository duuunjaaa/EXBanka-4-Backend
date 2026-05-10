package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/RAF-SI-2025/EXBanka-4-Backend/services/api-gateway/middleware"
	pb "github.com/RAF-SI-2025/EXBanka-4-Backend/shared/pb/securities"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type exchangeJSON struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Acronym  string `json:"acronym"`
	MICCode  string `json:"micCode"`
	Polity   string `json:"polity"`
	Currency string `json:"currency"`
	Timezone string `json:"timezone"`
}

func toExchangeJSON(e *pb.StockExchange) exchangeJSON {
	return exchangeJSON{
		ID:       e.Id,
		Name:     e.Name,
		Acronym:  e.Acronym,
		MICCode:  e.MicCode,
		Polity:   e.Polity,
		Currency: e.Currency,
		Timezone: e.Timezone,
	}
}

// resolveExchangeMIC looks up the MIC code for a path param that may be either
// a numeric ID or a MIC string. Returns the MIC on success; writes an HTTP
// error response and returns "" on failure.
func resolveExchangeMIC(c *gin.Context, client pb.SecuritiesServiceClient, id string) string {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if numID, err := strconv.ParseInt(id, 10, 64); err == nil {
		resp, err := client.GetStockExchangeById(ctx, &pb.GetStockExchangeByIdRequest{Id: numID})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": status.Convert(err).Message()})
				return ""
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch exchange"})
			return ""
		}
		return resp.Exchange.MicCode
	}

	resp, err := client.GetStockExchangeByMIC(ctx, &pb.GetStockExchangeByMICRequest{MicCode: id})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": status.Convert(err).Message()})
			return ""
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch exchange"})
		return ""
	}
	return resp.Exchange.MicCode
}

// GetStockExchanges godoc
// @Summary      List all stock exchanges (paginated)
// @Tags         securities
// @Produce      json
// @Param        page      query  int  false  "Page number (default 1)"
// @Param        pageSize  query  int  false  "Page size (default 10)"
// @Success      200  {object}  map[string]interface{}
// @Router       /stock-exchanges [get]
func GetStockExchanges(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		resp, err := client.GetStockExchanges(ctx, &pb.GetStockExchangesRequest{
			Page:     int32(page),
			PageSize: int32(pageSize),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch stock exchanges"})
			return
		}

		result := make([]exchangeJSON, 0, len(resp.Exchanges))
		for _, e := range resp.Exchanges {
			result = append(result, toExchangeJSON(e))
		}
		c.JSON(http.StatusOK, gin.H{"exchanges": result, "totalCount": resp.TotalCount})
	}
}

// GetStockExchange godoc
// @Summary      Get stock exchange by numeric ID or MIC code
// @Tags         securities
// @Produce      json
// @Param        id  path  string  true  "Numeric ID or MIC code"
// @Success      200  {object}  exchangeJSON
// @Router       /stock-exchanges/{id} [get]
func GetStockExchange(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		if numID, err := strconv.ParseInt(id, 10, 64); err == nil {
			resp, err := client.GetStockExchangeById(ctx, &pb.GetStockExchangeByIdRequest{Id: numID})
			if err != nil {
				if status.Code(err) == codes.NotFound {
					c.JSON(http.StatusNotFound, gin.H{"error": status.Convert(err).Message()})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch exchange"})
				return
			}
			c.JSON(http.StatusOK, toExchangeJSON(resp.Exchange))
			return
		}

		resp, err := client.GetStockExchangeByMIC(ctx, &pb.GetStockExchangeByMICRequest{MicCode: id})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": status.Convert(err).Message()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch exchange"})
			return
		}
		c.JSON(http.StatusOK, toExchangeJSON(resp.Exchange))
	}
}

// CreateStockExchange godoc
// @Summary      Create a new stock exchange
// @Tags         securities
// @Accept       json
// @Produce      json
// @Success      201  {object}  exchangeJSON
// @Router       /stock-exchanges [post]
func CreateStockExchange(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Name     string `json:"name"     binding:"required"`
			Acronym  string `json:"acronym"  binding:"required"`
			MICCode  string `json:"micCode"  binding:"required"`
			Polity   string `json:"polity"   binding:"required"`
			Currency string `json:"currency" binding:"required"`
			Timezone string `json:"timezone" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		resp, err := client.CreateStockExchange(ctx, &pb.CreateStockExchangeRequest{
			Name:     body.Name,
			Acronym:  body.Acronym,
			MicCode:  body.MICCode,
			Polity:   body.Polity,
			Currency: body.Currency,
			Timezone: body.Timezone,
		})
		if err != nil {
			if status.Code(err) == codes.AlreadyExists {
				c.JSON(http.StatusConflict, gin.H{"error": status.Convert(err).Message()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create exchange"})
			return
		}
		c.JSON(http.StatusCreated, toExchangeJSON(resp.Exchange))
	}
}

// UpdateStockExchange godoc
// @Summary      Update a stock exchange by numeric ID or MIC code
// @Tags         securities
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "Numeric ID or MIC code"
// @Success      200  {object}  exchangeJSON
// @Router       /stock-exchanges/{id} [put]
func UpdateStockExchange(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		mic := resolveExchangeMIC(c, client, c.Param("id"))
		if mic == "" {
			return
		}
		var body struct {
			Name     string `json:"name"     binding:"required"`
			Acronym  string `json:"acronym"  binding:"required"`
			Polity   string `json:"polity"   binding:"required"`
			Currency string `json:"currency" binding:"required"`
			Timezone string `json:"timezone" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		resp, err := client.UpdateStockExchange(ctx, &pb.UpdateStockExchangeRequest{
			MicCode:  mic,
			Name:     body.Name,
			Acronym:  body.Acronym,
			Polity:   body.Polity,
			Currency: body.Currency,
			Timezone: body.Timezone,
		})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": status.Convert(err).Message()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update exchange"})
			return
		}
		c.JSON(http.StatusOK, toExchangeJSON(resp.Exchange))
	}
}

// DeleteStockExchange godoc
// @Summary      Delete a stock exchange by numeric ID or MIC code
// @Tags         securities
// @Param        id  path  string  true  "Numeric ID or MIC code"
// @Success      204
// @Router       /stock-exchanges/{id} [delete]
func DeleteStockExchange(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		mic := resolveExchangeMIC(c, client, c.Param("id"))
		if mic == "" {
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		_, err := client.DeleteStockExchange(ctx, &pb.DeleteStockExchangeRequest{MicCode: mic})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": status.Convert(err).Message()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete exchange"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ── Working Hours ─────────────────────────────────────────────────────────────

type workingHoursJSON struct {
	ID        int64  `json:"id"`
	Polity    string `json:"polity"`
	Segment   string `json:"segment"`
	OpenTime  string `json:"openTime"`
	CloseTime string `json:"closeTime"`
}

func toWorkingHoursJSON(h *pb.ExchangeWorkingHours) workingHoursJSON {
	return workingHoursJSON{
		ID: h.Id, Polity: h.Polity, Segment: h.Segment,
		OpenTime: h.OpenTime, CloseTime: h.CloseTime,
	}
}

// GetWorkingHours godoc
// @Summary      Get working hours for an exchange by numeric ID or MIC code
// @Tags         securities
// @Param        id  path  string  true  "Numeric ID or MIC code"
// @Success      200  {array}  workingHoursJSON
// @Router       /stock-exchanges/{id}/hours [get]
func GetWorkingHours(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		mic := resolveExchangeMIC(c, client, c.Param("id"))
		if mic == "" {
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		resp, err := client.GetWorkingHours(ctx, &pb.GetWorkingHoursRequest{MicCode: mic})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": status.Convert(err).Message()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch working hours"})
			return
		}
		result := make([]workingHoursJSON, 0, len(resp.Hours))
		for _, h := range resp.Hours {
			result = append(result, toWorkingHoursJSON(h))
		}
		c.JSON(http.StatusOK, result)
	}
}

// SetWorkingHours godoc
// @Summary      Set (upsert) working hours for a polity
// @Tags         securities
// @Accept       json
// @Produce      json
// @Success      200  {object}  workingHoursJSON
// @Router       /stock-exchanges/hours [post]
func SetWorkingHours(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Polity    string `json:"polity"     binding:"required"`
			Segment   string `json:"segment"    binding:"required"`
			OpenTime  string `json:"openTime"   binding:"required"`
			CloseTime string `json:"closeTime"  binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		resp, err := client.SetWorkingHours(ctx, &pb.SetWorkingHoursRequest{
			Polity:    body.Polity,
			Segment:   body.Segment,
			OpenTime:  body.OpenTime,
			CloseTime: body.CloseTime,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set working hours"})
			return
		}
		c.JSON(http.StatusOK, toWorkingHoursJSON(resp.Hours))
	}
}

// ── Holidays ──────────────────────────────────────────────────────────────────

type holidayJSON struct {
	ID          int64  `json:"id"`
	Polity      string `json:"polity"`
	HolidayDate string `json:"holidayDate"`
	Description string `json:"description"`
}

func toHolidayJSON(h *pb.ExchangeHoliday) holidayJSON {
	return holidayJSON{
		ID: h.Id, Polity: h.Polity, HolidayDate: h.HolidayDate, Description: h.Description,
	}
}

// GetHolidays godoc
// @Summary      Get holidays for an exchange's polity by numeric ID or MIC code
// @Tags         securities
// @Param        id  path  string  true  "Numeric ID or MIC code"
// @Success      200  {array}  holidayJSON
// @Router       /stock-exchanges/{id}/holidays [get]
func GetHolidays(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		mic := resolveExchangeMIC(c, client, c.Param("id"))
		if mic == "" {
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		// Resolve polity via GetStockExchangeByMIC, then GetHolidays by polity
		exchResp, err := client.GetStockExchangeByMIC(ctx, &pb.GetStockExchangeByMICRequest{MicCode: mic})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": status.Convert(err).Message()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch exchange"})
			return
		}

		resp, err := client.GetHolidays(ctx, &pb.GetHolidaysRequest{Polity: exchResp.Exchange.Polity})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch holidays"})
			return
		}
		result := make([]holidayJSON, 0, len(resp.Holidays))
		for _, h := range resp.Holidays {
			result = append(result, toHolidayJSON(h))
		}
		c.JSON(http.StatusOK, result)
	}
}

// AddHoliday godoc
// @Summary      Add a holiday for a polity
// @Tags         securities
// @Accept       json
// @Produce      json
// @Success      201  {object}  holidayJSON
// @Router       /stock-exchanges/holidays [post]
func AddHoliday(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Polity      string `json:"polity"       binding:"required"`
			HolidayDate string `json:"holidayDate"  binding:"required"`
			Description string `json:"description"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		resp, err := client.AddHoliday(ctx, &pb.AddHolidayRequest{
			Polity:      body.Polity,
			HolidayDate: body.HolidayDate,
			Description: body.Description,
		})
		if err != nil {
			if status.Code(err) == codes.AlreadyExists {
				c.JSON(http.StatusConflict, gin.H{"error": status.Convert(err).Message()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add holiday"})
			return
		}
		c.JSON(http.StatusCreated, toHolidayJSON(resp.Holiday))
	}
}

// DeleteHoliday godoc
// @Summary      Delete a holiday for a polity
// @Tags         securities
// @Param        polity  path  string  true  "Polity (country)"
// @Param        date    path  string  true  "Holiday date (YYYY-MM-DD)"
// @Success      204
// @Router       /stock-exchanges/holidays/{polity}/{date} [delete]
func DeleteHoliday(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		polity := c.Param("polity")
		date := c.Param("date")
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		_, err := client.DeleteHoliday(ctx, &pb.DeleteHolidayRequest{Polity: polity, HolidayDate: date})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": status.Convert(err).Message()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete holiday"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ── Exchange Status ───────────────────────────────────────────────────────────

// GetTestMode godoc
// @Summary      Get test mode status
// @Tags         securities
// @Produce      json
// @Success      200  {object}  map[string]bool
// @Router       /stock-exchanges/test-mode [get]
func GetTestMode(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		resp, err := client.GetTestMode(ctx, &pb.GetTestModeRequest{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get test mode"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"enabled": resp.Enabled})
	}
}

// SetTestMode godoc
// @Summary      Enable or disable test mode
// @Tags         securities
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]bool
// @Router       /stock-exchanges/test-mode [post]
func SetTestMode(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		resp, err := client.SetTestMode(ctx, &pb.SetTestModeRequest{Enabled: body.Enabled})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set test mode"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"enabled": resp.Enabled})
	}
}

// IsExchangeOpen godoc
// @Summary      Check if an exchange is currently open
// @Tags         securities
// @Param        id  path  string  true  "Exchange ID or MIC code"
// @Success      200  {object}  map[string]interface{}
// @Router       /stock-exchanges/{id}/is-open [get]
func IsExchangeOpen(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		mic := resolveExchangeMIC(c, client, c.Param("id"))
		if mic == "" {
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		resp, err := client.IsExchangeOpen(ctx, &pb.IsExchangeOpenRequest{MicCode: mic})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": status.Convert(err).Message()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check exchange status"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"micCode":          resp.MicCode,
			"isOpen":           resp.IsOpen,
			"segment":          resp.Segment,
			"currentTimeLocal": resp.CurrentTimeLocal,
		})
	}
}

// ── Listings ──────────────────────────────────────────────────────────────────

// clientAllowedTypes is the set of listing types visible to CLIENT-role callers.
var clientAllowedTypes = map[string]bool{"STOCK": true, "FUTURES_CONTRACT": true}

// GetSecurities godoc
// @Summary      List securities listings with optional filtering and pagination
// @Tags         securities
// @Produce      json
// @Param        type       query  string  false  "STOCK | FOREX_PAIR | FUTURES_CONTRACT | OPTION"
// @Param        exchange   query  string  false  "Exchange acronym prefix"
// @Param        ticker     query  string  false  "Ticker prefix"
// @Param        name       query  string  false  "Name substring"
// @Param        page       query  int     false  "Page (0-based, default 0)"
// @Param        pageSize   query  int     false  "Page size (default 20)"
// @Param        sortBy     query  string  false  "price | volume | change_percent | maintenance_margin"
// @Param        sortOrder  query  string  false  "ASC | DESC"
// @Success      200  {object}  map[string]interface{}
// @Router       /securities [get]
func GetSecurities(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := middleware.GetUserIDFromToken(c); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		isClient := middleware.GetCallerRoleFromToken(c) == "CLIENT"
		typeParam := c.Query("type")

		// Clients may only see STOCK and FUTURES_CONTRACT
		if isClient && typeParam != "" && !clientAllowedTypes[typeParam] {
			c.JSON(http.StatusOK, gin.H{"listings": []interface{}{}, "totalPages": 0, "totalElements": int64(0)})
			return
		}

		page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		resp, err := client.GetListings(ctx, &pb.GetListingsRequest{
			Type:                  typeParam,
			ExchangeAcronymPrefix: c.Query("exchange"),
			TickerPrefix:          c.Query("ticker"),
			NameSearch:            c.Query("name"),
			Page:                  int32(page + 1), // HTTP is 0-based; gRPC is 1-based
			PageSize:              int32(pageSize),
			SortBy:                c.Query("sortBy"),
			SortOrder:             c.Query("sortOrder"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch securities"})
			return
		}

		listings := resp.Listings
		if isClient {
			filtered := listings[:0]
			for _, l := range listings {
				if clientAllowedTypes[l.Type] {
					filtered = append(filtered, l)
				}
			}
			listings = filtered
		}

		c.JSON(http.StatusOK, gin.H{
			"listings":      listingSummariesToJSON(listings),
			"totalPages":    resp.TotalPages,
			"totalElements": resp.TotalElements,
		})
	}
}

// GetSecurityById godoc
// @Summary      Get full listing detail by ID
// @Tags         securities
// @Produce      json
// @Param        id  path  int  true  "Listing ID"
// @Success      200  {object}  map[string]interface{}
// @Router       /securities/{id} [get]
func GetSecurityById(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := middleware.GetUserIDFromToken(c); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid listing id"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		resp, err := client.GetListingById(ctx, &pb.GetListingByIdRequest{Id: id})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": status.Convert(err).Message()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch listing"})
			return
		}

		body := gin.H{
			"summary":      listingSummaryToJSON(resp.Summary),
			"priceHistory": dailyPriceHistoryToJSON(resp.PriceHistory),
		}
		switch d := resp.Detail.(type) {
		case *pb.GetListingByIdResponse_Stock:
			body["detail"] = gin.H{
				"type":              "STOCK",
				"outstandingShares": d.Stock.OutstandingShares,
				"dividendYield":     d.Stock.DividendYield,
				"marketCap":         d.Stock.MarketCap,
			}
		case *pb.GetListingByIdResponse_Forex:
			body["detail"] = gin.H{
				"type":          "FOREX_PAIR",
				"baseCurrency":  d.Forex.BaseCurrency,
				"quoteCurrency": d.Forex.QuoteCurrency,
				"liquidity":     d.Forex.Liquidity,
				"nominalValue":  d.Forex.NominalValue,
			}
		case *pb.GetListingByIdResponse_Futures:
			body["detail"] = gin.H{
				"type":           "FUTURES_CONTRACT",
				"contractSize":   d.Futures.ContractSize,
				"contractUnit":   d.Futures.ContractUnit,
				"settlementDate": d.Futures.SettlementDate,
			}
		case *pb.GetListingByIdResponse_Option:
			body["detail"] = gin.H{
				"type":              "OPTION",
				"stockListingId":    d.Option.StockListingId,
				"optionType":        d.Option.OptionType,
				"strikePrice":       d.Option.StrikePrice,
				"impliedVolatility": d.Option.ImpliedVolatility,
				"openInterest":      d.Option.OpenInterest,
				"settlementDate":    d.Option.SettlementDate,
			}
		}
		c.JSON(http.StatusOK, body)
	}
}

// GetSecurityHistory godoc
// @Summary      Get daily price history for a listing
// @Tags         securities
// @Produce      json
// @Param        id    path   int     true  "Listing ID"
// @Param        from  query  string  true  "Start date (YYYY-MM-DD)"
// @Param        to    query  string  true  "End date (YYYY-MM-DD)"
// @Success      200  {array}  map[string]interface{}
// @Router       /securities/{id}/history [get]
func GetSecurityHistory(client pb.SecuritiesServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := middleware.GetUserIDFromToken(c); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid listing id"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		resp, err := client.GetListingHistory(ctx, &pb.GetListingHistoryRequest{
			Id:       id,
			FromDate: c.Query("from"),
			ToDate:   c.Query("to"),
		})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": status.Convert(err).Message()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch price history"})
			return
		}

		c.JSON(http.StatusOK, dailyPriceHistoryToJSON(resp.History))
	}
}

// ── JSON serialisation helpers ────────────────────────────────────────────────

func listingSummaryToJSON(l *pb.ListingSummary) gin.H {
	if l == nil {
		return nil
	}
	return gin.H{
		"id":                l.Id,
		"ticker":            l.Ticker,
		"name":              l.Name,
		"type":              l.Type,
		"exchangeAcronym":   l.ExchangeAcronym,
		"price":             l.Price,
		"ask":               l.Ask,
		"bid":               l.Bid,
		"volume":            l.Volume,
		"changePercent":     l.ChangePercent,
		"maintenanceMargin": l.MaintenanceMargin,
		"initialMarginCost": l.InitialMarginCost,
		"nominalValue":      l.NominalValue,
		"optionType":        l.OptionType,
		"strikePrice":       l.StrikePrice,
		"settlementDate":    l.SettlementDate,
		"openInterest":      l.OpenInterest,
	}
}

func listingSummariesToJSON(listings []*pb.ListingSummary) []gin.H {
	result := make([]gin.H, 0, len(listings))
	for _, l := range listings {
		result = append(result, listingSummaryToJSON(l))
	}
	return result
}

func dailyPriceHistoryToJSON(history []*pb.DailyPriceInfo) []gin.H {
	result := make([]gin.H, 0, len(history))
	for _, p := range history {
		result = append(result, gin.H{
			"date":   p.Date,
			"price":  p.Price,
			"ask":    p.Ask,
			"bid":    p.Bid,
			"change": p.Change,
			"volume": p.Volume,
		})
	}
	return result
}
