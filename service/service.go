package service

import (
	"context"

	"golang-todo/domain"

	"github.com/google/uuid"
)

// TodoRepository defines what the service layer needs from the data layer.
// It is intentionally defined here (the consuming package) rather than in
// repository/, following the "accept interfaces, return structs" convention.
type TodoRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Todo, error)
	List(ctx context.Context, filter domain.TodoFilter) ([]domain.Todo, error)
	Create(ctx context.Context, todo *domain.Todo) error
	Update(ctx context.Context, todo *domain.Todo) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// CreateTodoRequest carries the input for creating a todo.
type CreateTodoRequest struct {
	Title       string
	Description string
}

// UpdateTodoRequest carries the input for updating a todo.
type UpdateTodoRequest struct {
	Title       string
	Description string
	Status      domain.TodoStatus
}
