package update

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Publisher func(context.Context, UpdateTaskMessage) error

type Handler struct {
	service   *Service
	publish   Publisher
	queueName func() string
	logger    *slog.Logger
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func NewHandler(service *Service, publish Publisher, queueName func() string, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, publish: publish, queueName: queueName, logger: logger}
}

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.POST("/update", h.updateFundsGinHandler)
	api.POST("/update/async", h.updateFundsAsyncGinHandler)
	api.GET("/update/tasks/:id", h.updateTaskStatusGinHandler)
}

func (h *Handler) updateFundsGinHandler(c *gin.Context) {
	if !requireUpdateAPIKey(c.Request) {
		errorFail(c, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	locked, lockToken, err := h.service.acquireUpdateLock(ctx)
	if err != nil {
		h.logger.Error("fund_update_lock_failed", "mode", "sync", "error", err)
		errorFail(c, http.StatusInternalServerError, "redis_lock_error", "failed to acquire update lock")
		return
	}
	if !locked {
		errorFail(c, http.StatusConflict, "update_locked", "fund update is already running")
		return
	}

	defer func() {
		released, err := h.service.releaseUpdateLock(context.Background(), lockToken)
		if err != nil {
			h.logger.Error("fund_update_unlock_failed", "mode", "sync", "error", err)
			return
		}
		if !released {
			h.logger.Warn("fund_update_unlock_skipped", "mode", "sync")
		}
	}()

	h.logger.Info("fund_update_start", "mode", "sync")
	response := h.service.executeFundUpdate(ctx)
	h.service.invalidateFundDetailCache(ctx, response.UpdatedCodes)
	h.logger.Info("fund_update_end",
		"mode", "sync",
		"status", response.Status,
		"updated", response.Updated,
		"failed", len(response.FailedCodes),
		"duration_ms", response.DurationMs,
	)
	c.JSON(http.StatusOK, response)
}

func (h *Handler) updateFundsAsyncGinHandler(c *gin.Context) {
	if !requireUpdateAPIKey(c.Request) {
		errorFail(c, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
		return
	}

	now := time.Now()
	taskID := fmt.Sprintf("update_%d", now.UnixNano())
	task := &updateTask{ID: taskID, Status: updateTaskPending, StartedAt: now}
	if err := h.service.saveUpdateTask(c.Request.Context(), task); err != nil {
		h.logger.Error("fund_update_task_save_failed", "task_id", taskID, "error", err)
		errorFail(c, http.StatusInternalServerError, "redis_task_error", "failed to save update task")
		return
	}

	publishCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := h.publish(publishCtx, UpdateTaskMessage{TaskID: taskID, Trigger: "manual", CreatedAt: now}); err != nil {
		task.Status = updateTaskFailed
		task.Error = "failed to publish update task: " + err.Error()
		finishedAt := time.Now()
		task.FinishedAt = &finishedAt
		if saveErr := h.service.saveUpdateTask(context.Background(), task); saveErr != nil {
			h.logger.Error("fund_update_task_save_failed", "task_id", taskID, "status", task.Status, "error", saveErr)
		}
		errorFail(c, http.StatusInternalServerError, "rabbitmq_publish_error", "failed to publish update task")
		return
	}

	h.logger.Info("fund_update_task_published", "task_id", taskID, "queue", h.queueName())
	c.JSON(http.StatusAccepted, updateTaskCreateResponse{Status: "accepted", TaskID: taskID})
}

func (h *Handler) updateTaskStatusGinHandler(c *gin.Context) {
	if !requireUpdateAPIKey(c.Request) {
		errorFail(c, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
		return
	}

	taskID := strings.TrimSpace(c.Param("id"))
	if taskID == "" {
		errorFail(c, http.StatusBadRequest, "invalid_request", "task_id is required")
		return
	}

	task, ok, err := h.service.loadUpdateTask(c.Request.Context(), taskID)
	if err != nil {
		h.logger.Error("fund_update_task_load_failed", "task_id", taskID, "error", err)
		errorFail(c, http.StatusInternalServerError, "redis_task_error", "failed to load update task")
		return
	}
	if !ok {
		errorFail(c, http.StatusNotFound, "not_found", "task not found")
		return
	}
	c.JSON(http.StatusOK, task)
}

func requireUpdateAPIKey(r *http.Request) bool {
	expectedKey := strings.TrimSpace(os.Getenv("UPDATE_API_KEY"))
	if expectedKey == "" {
		return false
	}
	providedKey := strings.TrimSpace(r.Header.Get("X-Update-Key"))
	return providedKey != "" && providedKey == expectedKey
}

func errorFail(c *gin.Context, status int, code string, message string) {
	c.JSON(status, errorResponse{Error: code, Message: message})
}
