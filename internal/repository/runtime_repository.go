package repository

import (
	"os"
	"time"
)

type RuntimeRepository struct{}

func NewRuntimeRepository() *RuntimeRepository {
	return &RuntimeRepository{}
}
func (r *RuntimeRepository) Version() string {
	v := os.Getenv("APP_VERSION")
	if v == "" {
		return "dev"
	}
	return v
}
func (r *RuntimeRepository) BuiltAt() *string {
	v := os.Getenv("APP_BUILT_AT")
	if v == "" {
		return nil
	}
	return &v
}
func (r *RuntimeRepository) CommitFull() *string {
	for _, key := range []string{"GIT_COMMIT", "RAILWAY_GIT_COMMIT_SHA", "SOURCE_VERSION"} {
		if v := os.Getenv(key); v != "" {
			return &v
		}
	}
	return nil
}
func (r *RuntimeRepository) ServerTime() int64 {
	return time.Now().Unix()
}
