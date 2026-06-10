package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func updateFundsGinHandler(c *gin.Context) {
	if !requireUpdateAPIKey(c.Request) {
		ErrorFail(c, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	locked, lockToken, err := acquireUpdateLock(ctx)
	if err != nil {
		appLogger.Error("fund_update_lock_failed", "mode", "sync", "error", err)
		ErrorFail(c, http.StatusInternalServerError, "redis_lock_error", "failed to acquire update lock")
		return
	}

	if !locked {
		ErrorFail(c, http.StatusConflict, "update_locked", "fund update is already running")
		return
	}

	defer func() {
		released, err := releaseUpdateLock(context.Background(), lockToken)
		if err != nil {
			appLogger.Error("fund_update_unlock_failed", "mode", "sync", "error", err)
			return
		}
		if !released {
			appLogger.Warn("fund_update_unlock_skipped", "mode", "sync")
		}
	}()

	appLogger.Info("fund_update_start", "mode", "sync")

	response := executeFundUpdate(ctx)
	invalidateFundDetailCache(ctx, response.UpdatedCodes)

	appLogger.Info("fund_update_end",
		"mode", "sync",
		"status", response.Status,
		"updated", response.Updated,
		"failed", len(response.FailedCodes),
		"duration_ms", response.DurationMs,
	)

	c.JSON(http.StatusOK, response)
}

func updateFundsAsyncGinHandler(c *gin.Context) {
	if !requireUpdateAPIKey(c.Request) {
		ErrorFail(c, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
		return
	}

	now := time.Now()
	taskID := fmt.Sprintf("update_%d", now.UnixNano())
	task := &updateTask{
		ID:        taskID,
		Status:    updateTaskPending,
		StartedAt: now,
	}

	if err := saveUpdateTask(c.Request.Context(), task); err != nil {
		appLogger.Error("fund_update_task_save_failed", "task_id", taskID, "error", err)
		ErrorFail(c, http.StatusInternalServerError, "redis_task_error", "failed to save update task")
		return
	}

	publishCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := publishUpdateTask(publishCtx, UpdateTaskMessage{
		TaskID:    taskID,
		Trigger:   "manual",
		CreatedAt: now,
	}); err != nil {
		task.Status = updateTaskFailed
		task.Error = "failed to publish update task: " + err.Error()
		finishedAt := time.Now()
		task.FinishedAt = &finishedAt
		if saveErr := saveUpdateTask(context.Background(), task); saveErr != nil {
			appLogger.Error("fund_update_task_save_failed", "task_id", taskID, "status", task.Status, "error", saveErr)
		}

		ErrorFail(c, http.StatusInternalServerError, "rabbitmq_publish_error", "failed to publish update task")
		return
	}

	appLogger.Info("fund_update_task_published", "task_id", taskID, "queue", updateQueueName())
	c.JSON(http.StatusAccepted, updateTaskCreateResponse{
		Status: "accepted",
		TaskID: taskID,
	})
}

func updateTaskStatusGinHandler(c *gin.Context) {
	if !requireUpdateAPIKey(c.Request) {
		ErrorFail(c, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
		return
	}

	taskID := strings.TrimSpace(c.Param("id"))
	if taskID == "" {
		ErrorFail(c, http.StatusBadRequest, "invalid_request", "task_id is required")
		return
	}

	task, ok, err := loadUpdateTask(c.Request.Context(), taskID)
	if err != nil {
		appLogger.Error("fund_update_task_load_failed", "task_id", taskID, "error", err)
		ErrorFail(c, http.StatusInternalServerError, "redis_task_error", "failed to load update task")
		return
	}

	if !ok {
		ErrorFail(c, http.StatusNotFound, "not_found", "task not found")
		return
	}

	c.JSON(http.StatusOK, task)
}

func requireUpdateAPIKey(r *http.Request) bool {
	expectedKey := configuredUpdateAPIKey()
	if expectedKey == "" {
		return false
	}

	providedKey := strings.TrimSpace(r.Header.Get("X-Update-Key"))
	return providedKey != "" && providedKey == expectedKey
}

func configuredUpdateAPIKey() string {
	return strings.TrimSpace(os.Getenv("UPDATE_API_KEY"))
}

func writeJSONError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   code,
		Message: message,
	})
}
