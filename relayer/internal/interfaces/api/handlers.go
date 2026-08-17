// Package api provides HTTP API handlers.
package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/popolo229099-svg/veilo-relayer/internal/usecase"
)

// Handler holds the use cases and provides HTTP handlers.
type Handler struct {
	relayUC *usecase.RelayUseCase
}

// NewHandler creates a new API handler.
func NewHandler(relayUC *usecase.RelayUseCase) *Handler {
	return &Handler{relayUC: relayUC}
}

// RegisterRoutes registers all API routes.
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1")
	{
		// Health
		api.GET("/health", h.Health)

		// Transactions
		api.POST("/relay", h.Relay)
		api.GET("/transactions", h.GetRecentTransactions)
		api.GET("/transactions/:id", h.GetTransaction)
		api.GET("/transactions/signature/:signature", h.GetTransactionBySignature)

		// Stats
		api.GET("/stats", h.GetStats)

		// Pools (read-only)
		api.GET("/pools", h.GetPools)
	}
}

// Health returns the system health status.
func (h *Handler) Health(c *gin.Context) {
	health, err := h.relayUC.GetHealth(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, health)
}

// Relay processes a relay request.
func (h *Handler) Relay(c *gin.Context) {
	var req usecase.RelayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	resp, err := h.relayUC.Relay(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetRecentTransactions returns recent transactions.
func (h *Handler) GetRecentTransactions(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	}

	txs, err := h.relayUC.GetRecentTransactions(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": txs,
		"count":        len(txs),
	})
}

// GetTransaction returns a transaction by ID.
func (h *Handler) GetTransaction(c *gin.Context) {
	id := c.Param("id")

	tx, err := h.relayUC.GetTransaction(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if tx == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "transaction not found",
		})
		return
	}

	c.JSON(http.StatusOK, tx)
}

// GetTransactionBySignature returns a transaction by signature.
func (h *Handler) GetTransactionBySignature(c *gin.Context) {
	signature := c.Param("signature")

	tx, err := h.relayUC.GetTransactionBySignature(c.Request.Context(), signature)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if tx == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "transaction not found",
		})
		return
	}

	c.JSON(http.StatusOK, tx)
}

// GetStats returns relayer statistics.
func (h *Handler) GetStats(c *gin.Context) {
	stats, err := h.relayUC.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetPools returns all pools.
func (h *Handler) GetPools(c *gin.Context) {
	// Simplified - return empty for now
	c.JSON(http.StatusOK, gin.H{
		"pools": []interface{}{},
	})
}

// CORS middleware
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Logger middleware
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		gin.DefaultWriter.Write([]byte(
			"[API] " + c.Request.Method + " " + path + " " +
				strconv.Itoa(status) + " " + latency.String() + "\n",
		))
	}
}

// Recovery middleware
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
