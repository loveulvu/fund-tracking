package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrWatchlistExists       = errors.New("watchlist item already exists")
	ErrWatchlistNotFound     = errors.New("watchlist item not found")
	ErrInvalidWatchlistInput = errors.New("invalid watchlist input")
)

type watchlistInputError struct {
	message string
}

func (e watchlistInputError) Error() string {
	return e.message
}

func (e watchlistInputError) Is(target error) bool {
	return target == ErrInvalidWatchlistInput
}

type watchlistListResult struct {
	Data        []byte
	CacheStatus string
}

func getWatchlistService(ctx context.Context, userID string) (watchlistListResult, error) {
	cacheKey := watchlistCacheKey(userID)

	if redisClient != nil {
		cached, err := redisClient.Get(ctx, cacheKey).Result()
		if err == nil {
			return watchlistListResult{
				Data:        []byte(cached),
				CacheStatus: "HIT",
			}, nil
		}
		if err != redis.Nil {
			appLogger.Warn("redis_get_failed", "key", cacheKey, "error", err)
		}
	}

	items, err := findWatchlistByUserID(ctx, userID)
	if err != nil {
		return watchlistListResult{}, err
	}

	data, err := json.Marshal(items)
	if err != nil {
		return watchlistListResult{}, err
	}

	if redisClient != nil {
		if err := redisClient.Set(ctx, cacheKey, data, time.Minute).Err(); err != nil {
			appLogger.Warn("redis_set_failed", "key", cacheKey, "error", err)
		}
	}

	return watchlistListResult{
		Data:        data,
		CacheStatus: "MISS",
	}, nil
}

func addWatchlistService(ctx context.Context, userID string, req AddWatchlistRequest) (WatchlistItem, error) {
	fundCode := strings.TrimSpace(req.FundCode)
	fundName := strings.TrimSpace(req.FundName)

	if fundCode == "" || fundName == "" {
		return WatchlistItem{}, watchlistInputError{message: "fundCode and fundName are required"}
	}

	alertThreshold := 5.0
	if req.AlertThreshold != nil {
		alertThreshold = *req.AlertThreshold
	}

	fund, found, err := findFundByCodeInMongoDB(ctx, fundCode)
	if err != nil {
		return WatchlistItem{}, err
	}
	if !found || fund.NetValue <= 0 {
		return WatchlistItem{}, watchlistInputError{message: "current fund net_value is unavailable"}
	}
	if strings.TrimSpace(fund.FundName) != "" {
		fundName = strings.TrimSpace(fund.FundName)
	}

	item := WatchlistItem{
		UserID:           userID,
		FundCode:         fundCode,
		FundName:         fundName,
		AlertThreshold:   alertThreshold,
		PurchaseDate:     time.Now().UTC().Format("2006-01-02"),
		PurchaseNetValue: fund.NetValue,
		AddedAt:          time.Now().UTC(),
	}

	createdItem, err := insertWatchlistItem(ctx, item)
	if err != nil {
		return WatchlistItem{}, err
	}

	invalidateWatchlistCache(ctx, userID)
	return createdItem, nil
}

func deleteWatchlistService(ctx context.Context, userID string, fundCode string) error {
	fundCode = strings.TrimSpace(fundCode)
	if fundCode == "" {
		return watchlistInputError{message: "fundCode is required"}
	}

	deleted, err := deleteWatchlistItem(ctx, userID, fundCode)
	if err != nil {
		return err
	}

	if !deleted {
		return ErrWatchlistNotFound
	}

	invalidateWatchlistCache(ctx, userID)
	return nil
}

func updateWatchlistThresholdService(ctx context.Context, userID string, fundCode string, req UpdateWatchlistThresholdRequest) (WatchlistItem, error) {
	fundCode = strings.TrimSpace(fundCode)
	if fundCode == "" {
		return WatchlistItem{}, watchlistInputError{message: "fundCode is required"}
	}

	purchaseDate := requestedPurchaseDate(req)
	if req.AlertThreshold == nil && purchaseDate == nil {
		return WatchlistItem{}, watchlistInputError{message: "alertThreshold or purchase_date is required"}
	}

	updateFields := map[string]any{}
	if req.AlertThreshold != nil {
		updateFields["alertThreshold"] = *req.AlertThreshold
	}
	if purchaseDate != nil {
		parsedDate, err := parsePurchaseDate(*purchaseDate)
		if err != nil {
			return WatchlistItem{}, err
		}
		snapshot, found, err := findPurchaseSnapshot(ctx, fundCode, parsedDate)
		if err != nil {
			return WatchlistItem{}, err
		}
		if !found || snapshot.NetValue <= 0 {
			return WatchlistItem{}, watchlistInputError{message: "purchase net value is unavailable for the requested date"}
		}
		updateFields["purchase_date"] = snapshot.NetValueDate
		updateFields["purchase_net_value"] = snapshot.NetValue
	}

	updatedItem, found, err := updateWatchlistItem(ctx, userID, fundCode, updateFields)
	if err != nil {
		return WatchlistItem{}, err
	}

	if !found {
		return WatchlistItem{}, ErrWatchlistNotFound
	}

	invalidateWatchlistCache(ctx, userID)
	return updatedItem, nil
}

func requestedPurchaseDate(req UpdateWatchlistThresholdRequest) *string {
	if req.PurchaseDate != nil {
		return req.PurchaseDate
	}
	return req.PurchaseDateCamel
}

func parsePurchaseDate(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", watchlistInputError{message: "purchase_date is required"}
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return "", watchlistInputError{message: "purchase_date must use YYYY-MM-DD"}
	}
	return parsed.Format("2006-01-02"), nil
}

func watchlistInputErrorMessage(err error, fallback string) string {
	var inputErr watchlistInputError
	if errors.As(err, &inputErr) && inputErr.message != "" {
		return inputErr.message
	}
	return fallback
}
