package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Worker struct {
	service *Service
	logger  *slog.Logger
}

func NewWorker(service *Service, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{service: service, logger: logger}
}

func (w *Worker) StartUpdateConsumer(ctx context.Context, msgs <-chan amqp.Delivery) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case delivery, ok := <-msgs:
				if !ok {
					return
				}
				if err := w.HandleUpdateTaskMessage(ctx, delivery.Body); err != nil {
					w.logger.Error("rabbitmq_update_task_failed", "error", err)
					_ = delivery.Nack(false, false)
					continue
				}
				_ = delivery.Ack(false)
			}
		}
	}()
}

func (w *Worker) HandleUpdateTaskMessage(parentCtx context.Context, body []byte) error {
	var msg UpdateTaskMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return err
	}
	msg.TaskID = strings.TrimSpace(msg.TaskID)
	if msg.TaskID == "" {
		return errors.New("update task message task_id is empty")
	}

	task, ok, err := w.service.loadUpdateTask(parentCtx, msg.TaskID)
	if err != nil {
		return err
	}
	if !ok {
		w.logger.Warn("rabbitmq_update_task_missing", "task_id", msg.TaskID)
		return fmt.Errorf("update task not found: %s", msg.TaskID)
	}
	if task.Status == updateTaskSuccess {
		w.logger.Info("rabbitmq_update_task_already_success", "task_id", msg.TaskID)
		return nil
	}

	task.Status = updateTaskRunning
	if err := w.service.saveUpdateTask(parentCtx, task); err != nil {
		return err
	}

	locked, lockToken, err := w.service.acquireUpdateLock(parentCtx)
	if err != nil {
		return err
	}
	if !locked {
		return w.finishUpdateTaskAsFailed(parentCtx, task, "fund update is already running")
	}

	runCtx, cancel := context.WithTimeout(parentCtx, 2*time.Minute)
	defer cancel()

	w.logger.Info("fund_update_task_started", "task_id", msg.TaskID, "trigger", msg.Trigger)
	response := w.service.executeFundUpdate(runCtx)
	w.service.invalidateFundDetailCache(runCtx, response.UpdatedCodes)

	finishedAt := time.Now()
	task.Response = &response
	task.FinishedAt = &finishedAt
	if response.Status == "success" || response.Status == "partial_success" {
		task.Status = updateTaskSuccess
		task.Error = ""
	} else {
		task.Status = updateTaskFailed
		task.Error = "fund update failed"
	}

	saveErr := w.service.saveUpdateTask(parentCtx, task)
	released, releaseErr := w.service.releaseUpdateLock(context.Background(), lockToken)
	if releaseErr != nil {
		w.logger.Error("fund_update_unlock_failed", "task_id", msg.TaskID, "error", releaseErr)
	} else if !released {
		w.logger.Warn("fund_update_unlock_skipped", "task_id", msg.TaskID)
	}
	if saveErr != nil {
		return saveErr
	}

	w.logger.Info("fund_update_task_finished",
		"task_id", msg.TaskID,
		"status", task.Status,
		"updated", response.Updated,
		"failed", len(response.FailedCodes),
		"duration_ms", response.DurationMs,
	)
	return nil
}

func (w *Worker) finishUpdateTaskAsFailed(ctx context.Context, task *updateTask, message string) error {
	task.Status = updateTaskFailed
	task.Error = message
	finishedAt := time.Now()
	task.FinishedAt = &finishedAt
	if err := w.service.saveUpdateTask(ctx, task); err != nil {
		w.logger.Error("fund_update_task_save_failed", "task_id", task.ID, "status", task.Status, "error", err)
		return err
	}
	return nil
}
