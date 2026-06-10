package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	updatepkg "fund-tracking-backend-go/internal/update"
	amqp "github.com/rabbitmq/amqp091-go"
)

const defaultUpdateQueueName = "fund.update.tasks"

var rabbitConn *amqp.Connection
var rabbitPublishChannel *amqp.Channel
var rabbitConsumerChannel *amqp.Channel

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

	_, err = publishCh.QueueDeclare(updateQueueName(), true, false, false, false, nil)
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

func publishUpdateTask(ctx context.Context, msg updatepkg.UpdateTaskMessage) error {
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

func startUpdateConsumer(ctx context.Context, worker *updatepkg.Worker) error {
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

	worker.StartUpdateConsumer(ctx, msgs)
	return nil
}
