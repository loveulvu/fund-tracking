package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type WatchlistItem struct {
	UserID           string    `bson:"userId" json:"userId"`
	FundCode         string    `bson:"fundCode" json:"fundCode"`
	FundName         string    `bson:"fundName" json:"fundName"`
	AlertThreshold   float64   `bson:"alertThreshold" json:"alertThreshold"`
	PurchaseDate     string    `bson:"purchase_date" json:"purchase_date"`
	PurchaseNetValue float64   `bson:"purchase_net_value" json:"purchase_net_value"`
	AddedAt          time.Time `bson:"addedAt" json:"addedAt"`
}
type AddWatchlistRequest struct {
	FundCode       string   `json:"fundCode"`
	FundName       string   `json:"fundName"`
	AlertThreshold *float64 `json:"alertThreshold"`
}
type UpdateWatchlistThresholdRequest struct {
	AlertThreshold    *float64 `json:"alertThreshold"`
	PurchaseDate      *string  `json:"purchase_date"`
	PurchaseDateCamel *string  `json:"purchaseDate"`
}

func getWatchlistGinHandler(c *gin.Context) {
	claims, ok := RequireGinAuthClaims(c)
	if !ok {
		return
	}

	result, err := getWatchlistService(c.Request.Context(), claims.UserID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "internal_error", "failed to fetch watchlist")
		return
	}

	c.Header("X-Cache", result.CacheStatus)
	c.Data(http.StatusOK, "application/json;charset=utf-8", result.Data)
}
func addWatchlistGinHandler(c *gin.Context) {
	claims, ok := RequireGinAuthClaims(c)
	if !ok {
		return
	}

	var req AddWatchlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	createdItem, err := addWatchlistService(c.Request.Context(), claims.UserID, req)
	if err != nil {
		handleWatchlistError(c, err, watchlistErrorMessages{
			Invalid:  "fundCode and fundName are required",
			Exists:   "fund already in watchlist",
			Internal: "failed to add watchlist item",
		})
		return
	}

	Success(c, http.StatusCreated, createdItem)
}

func deleteWatchlistGinHandler(c *gin.Context) {
	claims, ok := RequireGinAuthClaims(c)
	if !ok {
		return
	}

	err := deleteWatchlistService(c.Request.Context(), claims.UserID, c.Param("fundCode"))
	if err != nil {
		handleWatchlistError(c, err, watchlistErrorMessages{
			Invalid:  "fundCode is required",
			NotFound: "watchlist item not found",
			Internal: "failed to delete watchlist item",
		})
		return
	}

	Success(c, http.StatusOK, gin.H{
		"message": "successfully removed from watchlist",
	})
}

func updateWatchlistThresholdGinHandler(c *gin.Context) {
	claims, ok := RequireGinAuthClaims(c)
	if !ok {
		return
	}

	var req UpdateWatchlistThresholdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	updatedItem, err := updateWatchlistThresholdService(
		c.Request.Context(),
		claims.UserID,
		c.Param("fundCode"),
		req,
	)
	if err != nil {
		handleWatchlistError(c, err, watchlistErrorMessages{
			Invalid:  "invalid watchlist request",
			NotFound: "watchlist item not found",
			Internal: "failed to update watchlist item",
		})
		return
	}

	Success(c, http.StatusOK, updatedItem)
}

type watchlistErrorMessages struct {
	Invalid  string
	Exists   string
	NotFound string
	Internal string
}

func handleWatchlistError(c *gin.Context, err error, messages watchlistErrorMessages) {
	switch {
	case errors.Is(err, ErrInvalidWatchlistInput):
		Fail(c, http.StatusBadRequest, "invalid_request", watchlistInputErrorMessage(err, messages.Invalid))
	case errors.Is(err, ErrWatchlistExists):
		Fail(c, http.StatusConflict, "conflict", messages.Exists)
	case errors.Is(err, ErrWatchlistNotFound):
		Fail(c, http.StatusNotFound, "not_found", messages.NotFound)
	default:
		Fail(c, http.StatusInternalServerError, "internal_error", messages.Internal)
	}
}
