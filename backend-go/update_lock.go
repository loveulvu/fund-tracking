package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const updateLockKey = "lock:fundtracking:update"
const updateLockTTL = 5 * time.Minute

var releaseUpdateLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
else
	return 0
end
`)

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
