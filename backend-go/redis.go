package main

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client

func initRedis() error {
	redisURL := os.Getenv("REDIS_URL")

	var options *redis.Options
	var err error

	if redisURL != "" {
		options, err = redis.ParseURL(redisURL)
		if err != nil {
			return err
		}
	} else {
		addr := os.Getenv("REDIS_ADDR")
		if addr == "" {
			addr = "127.0.0.1:6379"
		}
		options = &redis.Options{
			Addr: addr,
		}
	}

	client := redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return err
	}
	redisClient = client
	return nil
}

func getRedisClient() *redis.Client {
	return redisClient
}

func invalidateFundDetailCache(ctx context.Context, codes []string) {
	if redisClient == nil || len(codes) == 0 {
		return
	}
	keys := make([]string, 0, len(codes))
	for _, code := range codes {
		keys = append(keys, "fund:detail:"+code)
	}
	if err := redisClient.Del(ctx, keys...).Err(); err != nil {
		appLogger.Warn("redis_delete_failed", "keys", keys, "error", err)
	}
}

func searchCacheKey(query string) string {
	return "fund:search:" + query
}
func watchlistCacheKey(userID string) string {
	return "user:" + userID + ":watchlist"
}
func invalidateWatchlistCache(ctx context.Context, userID string) {
	if redisClient == nil {
		return
	}
	if err := redisClient.Del(ctx, watchlistCacheKey(userID)).Err(); err != nil {
		appLogger.Warn("redis_delete_failed", "key", watchlistCacheKey(userID), "error", err)
	}
}
