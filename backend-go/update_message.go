package main

import "time"

type UpdateTaskMessage struct {
	TaskID    string    `json:"task_id"`
	Trigger   string    `json:"trigger"`
	CreatedAt time.Time `json:"created_at"`
}
