package update

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const updateTaskTTL = 1 * time.Hour

type updateTaskStatus string

const (
	updateTaskPending updateTaskStatus = "pending"
	updateTaskRunning updateTaskStatus = "running"
	updateTaskSuccess updateTaskStatus = "success"
	updateTaskFailed  updateTaskStatus = "failed"
)

type updateTaskCreateResponse struct {
	Status string `json:"status"`
	TaskID string `json:"task_id"`
}

type updateTask struct {
	ID         string           `json:"id"`
	Status     updateTaskStatus `json:"status"`
	StartedAt  time.Time        `json:"started_at"`
	FinishedAt *time.Time       `json:"finished_at,omitempty"`
	Response   *Response        `json:"response,omitempty"`
	Error      string           `json:"error,omitempty"`
}

func updateTaskKey(taskID string) string {
	return "fund:update:task:" + taskID
}

func (s *Service) saveUpdateTask(ctx context.Context, task *updateTask) error {
	if s.redisClient == nil {
		return errors.New("redis client is not initialized")
	}
	if task == nil {
		return errors.New("task is nil")
	}
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return s.redisClient.Set(ctx, updateTaskKey(task.ID), data, updateTaskTTL).Err()
}

func (s *Service) loadUpdateTask(ctx context.Context, taskID string) (*updateTask, bool, error) {
	if s.redisClient == nil {
		return nil, false, errors.New("redis client is not initialized")
	}
	data, err := s.redisClient.Get(ctx, updateTaskKey(taskID)).Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var task updateTask
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		return nil, false, err
	}
	return &task, true, nil
}
