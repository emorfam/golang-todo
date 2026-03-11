package domain

import (
	"time"

	"github.com/google/uuid"
)

type TodoStatus string

const (
	TodoStatusOpen TodoStatus = "open"
	TodoStatusDone TodoStatus = "done"
)

type Todo struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      TodoStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type TodoFilter struct {
	Status *TodoStatus
}
