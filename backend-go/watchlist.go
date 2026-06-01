package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
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
	claims, ok := getGinAuthClaims(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "unauthorized",
			"message": "unauthorized",
		})
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

	items, err := findWatchlistByUserID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": "failed to fetch watchlist",
		})
		return
	}

	data, err := json.Marshal(items)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": "internal server error",
		})
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
	claims, ok := getGinAuthClaims(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "unauthorized",
			"message": "unauthorized",
		})
		return
	}

	var req AddWatchlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "invalid_request",
			"message": "invalid request body",
		})
		return
	}

	req.FundCode = strings.TrimSpace(req.FundCode)
	req.FundName = strings.TrimSpace(req.FundName)

	if req.FundCode == "" || req.FundName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "invalid_request",
			"message": "fundCode and fundName are required",
		})
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

	createdItem, err := insertWatchlistItem(item)
	if err != nil {
		if errors.Is(err, errWatchlistExists) {
			c.JSON(http.StatusConflict, gin.H{
				"code":    "conflict",
				"message": "fund already in watchlist",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": "failed to add watchlist item",
		})
		return
	}

	invalidateWatchlistCache(c.Request.Context(), claims.UserID)

	c.JSON(http.StatusCreated, createdItem)
}

func insertWatchlistItem(item WatchlistItem) (WatchlistItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, collection, err := getWatchlistCollection(ctx)
	if err != nil {
		return WatchlistItem{}, err
	}
	defer client.Disconnect(ctx)
	filter := bson.M{
		"userId":   item.UserID,
		"fundCode": item.FundCode,
	}
	err = collection.FindOne(ctx, filter).Err()
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
func getWatchlistCollection(ctx context.Context) (*mongo.Client, *mongo.Collection, error) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://127.0.0.1:27017"
	}
	clientOptions := options.Client().
		ApplyURI(uri).
		SetServerSelectionTimeout(30 * time.Second).
		SetConnectTimeout(10 * time.Second)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, nil, err
	}
	collection := client.Database("fund_tracking").Collection("watchlists")
	return client, collection, nil
}
func findWatchlistByUserID(userID string) ([]WatchlistItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, collection, err := getWatchlistCollection(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(ctx)

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
	claims, ok := getGinAuthClaims(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "unauthorized",
			"message": "unauthorized",
		})
		return
	}

	fundCode := strings.TrimSpace(c.Param("fundCode"))
	if fundCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "invalid_request",
			"message": "fundCode is required",
		})
		return
	}

	deleted, err := deleteWatchlistItem(claims.UserID, fundCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": "failed to delete watchlist item",
		})
		return
	}

	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "not_found",
			"message": "watchlist item not found",
		})
		return
	}

	invalidateWatchlistCache(c.Request.Context(), claims.UserID)

	c.JSON(http.StatusOK, gin.H{
		"message": "successfully removed from watchlist",
	})
}

func deleteWatchlistItem(userID string, fundCode string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, collection, err := getWatchlistCollection(ctx)
	if err != nil {
		return false, err
	}
	defer client.Disconnect(ctx)
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
	claims, ok := getGinAuthClaims(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "unauthorized",
			"message": "unauthorized",
		})
		return
	}

	fundCode := strings.TrimSpace(c.Param("fundCode"))
	if fundCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "invalid_request",
			"message": "fundCode is required",
		})
		return
	}

	var req UpdateWatchlistThresholdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "invalid_request",
			"message": "invalid request body",
		})
		return
	}

	if req.AlertThreshold == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "invalid_request",
			"message": "alertThreshold is required",
		})
		return
	}

	updatedItem, found, err := updateWatchlistThreshold(
		claims.UserID,
		fundCode,
		*req.AlertThreshold,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": "failed to update watchlist threshold",
		})
		return
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "not_found",
			"message": "watchlist item not found",
		})
		return
	}

	invalidateWatchlistCache(c.Request.Context(), claims.UserID)

	c.JSON(http.StatusOK, updatedItem)
}

func updateWatchlistThreshold(userID string, fundCode string, alertThreshold float64) (WatchlistItem, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, collection, err := getWatchlistCollection(ctx)
	if err != nil {
		return WatchlistItem{}, false, err
	}
	defer client.Disconnect(ctx)

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
