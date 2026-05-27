package main

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client

func initRedis() error {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
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
