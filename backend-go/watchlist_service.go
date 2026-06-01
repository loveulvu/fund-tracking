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

	item := WatchlistItem{
		UserID:         userID,
		FundCode:       fundCode,
		FundName:       fundName,
		AlertThreshold: alertThreshold,
		AddedAt:        time.Now().UTC(),
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

	if req.AlertThreshold == nil {
		return WatchlistItem{}, watchlistInputError{message: "alertThreshold is required"}
	}

	updatedItem, found, err := updateWatchlistThreshold(ctx, userID, fundCode, *req.AlertThreshold)
	if err != nil {
		return WatchlistItem{}, err
	}

	if !found {
		return WatchlistItem{}, ErrWatchlistNotFound
	}

	invalidateWatchlistCache(ctx, userID)
	return updatedItem, nil
}

func watchlistInputErrorMessage(err error, fallback string) string {
	var inputErr watchlistInputError
	if errors.As(err, &inputErr) && inputErr.message != "" {
		return inputErr.message
	}
	return fallback
}
