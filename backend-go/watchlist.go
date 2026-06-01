package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type WatchlistItem struct {
	UserID         string    `bson:"userId" json:"userId"`
	FundCode       string    `bson:"fundCode" json:"fundCode"`
	FundName       string    `bson:"fundName" json:"fundName"`
	AlertThreshold float64   `bson:"alertThreshold" json:"alertThreshold"`
	AddedAt        time.Time `bson:"addedAt" json:"addedAt"`
}
type AddWatchlistRequest struct {
	FundCode       string   `json:"fundCode"`
	FundName       string   `json:"fundName"`
	AlertThreshold *float64 `json:"alertThreshold"`
}
type UpdateWatchlistThresholdRequest struct {
	AlertThreshold *float64 `json:"alertThreshold"`
}

var errWatchlistExists = errors.New("watchlist item already exists")

func getWatchlistGinHandler(c *gin.Context) {
	claims, ok := RequireGinAuthClaims(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	userID := claims.UserID
	cacheKey := watchlistCacheKey(userID)

	if redisClient != nil {
		cached, err := redisClient.Get(ctx, cacheKey).Result()
		if err == nil {
			c.Header("X-Cache", "HIT")
			c.Data(http.StatusOK, "application/json;charset=utf-8", []byte(cached))
			return
		}
		if err != redis.Nil {
			appLogger.Warn("redis_get_failed", "key", cacheKey, "error", err)
		}
	}

	items, err := findWatchlistByUserID(ctx, userID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "internal_error", "failed to fetch watchlist")
		return
	}

	data, err := json.Marshal(items)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	if redisClient != nil {
		if err := redisClient.Set(ctx, cacheKey, data, time.Minute).Err(); err != nil {
			appLogger.Warn("redis_set_failed", "key", cacheKey, "error", err)
		}
	}

	c.Header("X-Cache", "MISS")
	c.Data(http.StatusOK, "application/json;charset=utf-8", data)
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

	req.FundCode = strings.TrimSpace(req.FundCode)
	req.FundName = strings.TrimSpace(req.FundName)

	if req.FundCode == "" || req.FundName == "" {
		Fail(c, http.StatusBadRequest, "invalid_request", "fundCode and fundName are required")
		return
	}

	alertThreshold := 5.0
	if req.AlertThreshold != nil {
		alertThreshold = *req.AlertThreshold
	}

	item := WatchlistItem{
		UserID:         claims.UserID,
		FundCode:       req.FundCode,
		FundName:       req.FundName,
		AlertThreshold: alertThreshold,
		AddedAt:        time.Now().UTC(),
	}

	ctx := c.Request.Context()
	createdItem, err := insertWatchlistItem(ctx, item)
	if err != nil {
		if errors.Is(err, errWatchlistExists) {
			Fail(c, http.StatusConflict, "conflict", "fund already in watchlist")
			return
		}

		Fail(c, http.StatusInternalServerError, "internal_error", "failed to add watchlist item")
		return
	}

	invalidateWatchlistCache(ctx, claims.UserID)

	Success(c, http.StatusCreated, createdItem)
}

func insertWatchlistItem(parentCtx context.Context, item WatchlistItem) (WatchlistItem, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()
	collection := getWatchlistCollection()
	filter := bson.M{
		"userId":   item.UserID,
		"fundCode": item.FundCode,
	}
	err := collection.FindOne(ctx, filter).Err()
	if err == nil {
		return WatchlistItem{}, errWatchlistExists
	}
	if err != mongo.ErrNoDocuments {
		return WatchlistItem{}, err
	}
	_, err = collection.InsertOne(ctx, item)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return WatchlistItem{}, errWatchlistExists
		}
		return WatchlistItem{}, err
	}

	return item, nil
}
func getWatchlistCollection() *mongo.Collection {
	return mongoClient.Database("fund_tracking").Collection("watchlists")
}
func findWatchlistByUserID(parentCtx context.Context, userID string) ([]WatchlistItem, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	collection := getWatchlistCollection()

	filter := bson.M{"userId": userID}

	findOptions := options.Find().SetProjection(bson.M{
		"_id": 0,
	})

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]WatchlistItem, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil

}
func deleteWatchlistGinHandler(c *gin.Context) {
	claims, ok := RequireGinAuthClaims(c)
	if !ok {
		return
	}

	fundCode := strings.TrimSpace(c.Param("fundCode"))
	if fundCode == "" {
		Fail(c, http.StatusBadRequest, "invalid_request", "fundCode is required")
		return
	}

	ctx := c.Request.Context()
	deleted, err := deleteWatchlistItem(ctx, claims.UserID, fundCode)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "internal_error", "failed to delete watchlist item")
		return
	}

	if !deleted {
		Fail(c, http.StatusNotFound, "not_found", "watchlist item not found")
		return
	}

	invalidateWatchlistCache(ctx, claims.UserID)

	Success(c, http.StatusOK, gin.H{
		"message": "successfully removed from watchlist",
	})
}

func deleteWatchlistItem(parentCtx context.Context, userID string, fundCode string) (bool, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()
	collection := getWatchlistCollection()
	filter := bson.M{
		"userId":   userID,
		"fundCode": fundCode,
	}
	result, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return false, err
	}
	return result.DeletedCount > 0, nil
}
func updateWatchlistThresholdGinHandler(c *gin.Context) {
	claims, ok := RequireGinAuthClaims(c)
	if !ok {
		return
	}

	fundCode := strings.TrimSpace(c.Param("fundCode"))
	if fundCode == "" {
		Fail(c, http.StatusBadRequest, "invalid_request", "fundCode is required")
		return
	}

	var req UpdateWatchlistThresholdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	if req.AlertThreshold == nil {
		Fail(c, http.StatusBadRequest, "invalid_request", "alertThreshold is required")
		return
	}

	updatedItem, found, err := updateWatchlistThreshold(
		c.Request.Context(),
		claims.UserID,
		fundCode,
		*req.AlertThreshold,
	)

	if err != nil {
		Fail(c, http.StatusInternalServerError, "internal_error", "failed to update watchlist threshold")
		return
	}

	if !found {
		Fail(c, http.StatusNotFound, "not_found", "watchlist item not found")
		return
	}

	invalidateWatchlistCache(c.Request.Context(), claims.UserID)

	Success(c, http.StatusOK, updatedItem)
}

func updateWatchlistThreshold(parentCtx context.Context, userID string, fundCode string, alertThreshold float64) (WatchlistItem, bool, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	collection := getWatchlistCollection()

	filter := bson.M{
		"userId":   userID,
		"fundCode": fundCode,
	}

	update := bson.M{
		"$set": bson.M{
			"alertThreshold": alertThreshold,
		},
	}

	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return WatchlistItem{}, false, err
	}

	if result.MatchedCount == 0 {
		return WatchlistItem{}, false, nil
	}

	findOptions := options.FindOne().SetProjection(bson.M{
		"_id": 0,
	})

	var updatedItem WatchlistItem
	if err := collection.FindOne(ctx, filter, findOptions).Decode(&updatedItem); err != nil {
		return WatchlistItem{}, false, err
	}

	return updatedItem, true, nil
}
