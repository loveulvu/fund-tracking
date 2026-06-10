package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const defaultUpdateQueueName = "fund.update.tasks"

var rabbitConn *amqp.Connection
var rabbitPublishChannel *amqp.Channel
var rabbitConsumerChannel *amqp.Channel

type UpdateTaskMessage struct {
	TaskID    string    `json:"task_id"`
	Trigger   string    `json:"trigger"`
	CreatedAt time.Time `json:"created_at"`
}

func updateQueueName() string {
	name := os.Getenv("RABBITMQ_UPDATE_QUEUE")
	if name == "" {
		return defaultUpdateQueueName
	}
	return name
}

func initRabbitMQ() error {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@127.0.0.1:5672/"
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return err
	}

	publishCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}

	_, err = publishCh.QueueDeclare(
		updateQueueName(),
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = publishCh.Close()
		_ = conn.Close()
		return err
	}

	consumerCh, err := conn.Channel()
	if err != nil {
		_ = publishCh.Close()
		_ = conn.Close()
		return err
	}

	rabbitConn = conn
	rabbitPublishChannel = publishCh
	rabbitConsumerChannel = consumerCh
	return nil
}

func closeRabbitMQ() {
	if rabbitConsumerChannel != nil {
		_ = rabbitConsumerChannel.Close()
	}
	if rabbitPublishChannel != nil {
		_ = rabbitPublishChannel.Close()
	}
	if rabbitConn != nil {
		_ = rabbitConn.Close()
	}
}

func publishUpdateTask(ctx context.Context, msg UpdateTaskMessage) error {
	if rabbitPublishChannel == nil {
		return errors.New("rabbitmq channel is not initialized")
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return rabbitPublishChannel.PublishWithContext(
		ctx,
		"",
		updateQueueName(),
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

func startUpdateConsumer(ctx context.Context) error {
	if rabbitConsumerChannel == nil {
		return errors.New("rabbitmq channel is not initialized")
	}

	msgs, err := rabbitConsumerChannel.Consume(
		updateQueueName(),
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-msgs:
				if !ok {
					return
				}

				if err := handleUpdateTaskMessage(ctx, d.Body); err != nil {
					appLogger.Error("rabbitmq_update_task_failed", "error", err)
					_ = d.Nack(false, false)
					continue
				}

				_ = d.Ack(false)
			}
		}
	}()

	return nil
}

func handleUpdateTaskMessage(parentCtx context.Context, body []byte) error {
	var msg UpdateTaskMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return err
	}
	msg.TaskID = strings.TrimSpace(msg.TaskID)
	if msg.TaskID == "" {
		return errors.New("update task message task_id is empty")
	}

	task, ok, err := loadUpdateTask(parentCtx, msg.TaskID)
	if err != nil {
		return err
	}
	if !ok {
		appLogger.Warn("rabbitmq_update_task_missing", "task_id", msg.TaskID)
		return fmt.Errorf("update task not found: %s", msg.TaskID)
	}

	if task.Status == updateTaskSuccess {
		appLogger.Info("rabbitmq_update_task_already_success", "task_id", msg.TaskID)
		return nil
	}

	task.Status = updateTaskRunning
	if err := saveUpdateTask(parentCtx, task); err != nil {
		return err
	}

	locked, lockToken, err := acquireUpdateLock(parentCtx)
	if err != nil {
		return err
	}
	if !locked {
		return finishUpdateTaskAsFailed(parentCtx, task, "fund update is already running")
	}

	runCtx, cancel := context.WithTimeout(parentCtx, 2*time.Minute)
	defer cancel()

	appLogger.Info("fund_update_task_started", "task_id", msg.TaskID, "trigger", msg.Trigger)
	response := executeFundUpdate(runCtx)
	invalidateFundDetailCache(runCtx, response.UpdatedCodes)

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

	saveErr := saveUpdateTask(parentCtx, task)

	released, releaseErr := releaseUpdateLock(context.Background(), lockToken)
	if releaseErr != nil {
		appLogger.Error("fund_update_unlock_failed", "task_id", msg.TaskID, "error", releaseErr)
	} else if !released {
		appLogger.Warn("fund_update_unlock_skipped", "task_id", msg.TaskID)
	}
	if saveErr != nil {
		return saveErr
	}

	appLogger.Info("fund_update_task_finished",
		"task_id", msg.TaskID,
		"status", task.Status,
		"updated", response.Updated,
		"failed", len(response.FailedCodes),
		"duration_ms", response.DurationMs,
	)
	return nil
}

func finishUpdateTaskAsFailed(ctx context.Context, task *updateTask, message string) error {
	task.Status = updateTaskFailed
	task.Error = message
	finishedAt := time.Now()
	task.FinishedAt = &finishedAt
	if err := saveUpdateTask(ctx, task); err != nil {
		appLogger.Error("fund_update_task_save_failed", "task_id", task.ID, "status", task.Status, "error", err)
		return err
	}
	return nil
}
