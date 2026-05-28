package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const updateLockKey = "lock:fundtracking:update"
const updateLockTTL = 120 * time.Second

var releaseUpdateLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
else
	return 0
end
`)
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
func newLockToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func acquireUpdateLock(ctx context.Context) (bool, string, error) {
	if redisClient == nil {
		return false, "", errors.New("redis client is not initialized")
	}
	token, err := newLockToken()
	if err != nil {
		return false, "", err
	}
	locked, err := redisClient.SetNX(ctx, updateLockKey, token, updateLockTTL).Result()
	if err != nil {
		return false, "", err
	}
	if !locked {
		return false, "", nil
	}
	return true, token, nil
}
func releaseUpdateLock(ctx context.Context, token string) (bool, error) {
	if redisClient == nil {
		return false, errors.New("redis client is not initialized")
	}
	if token == "" {
		return false, errors.New("lock token is empty")
	}
	result, err := releaseUpdateLockScript.Run(ctx, redisClient, []string{updateLockKey}, token).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}
