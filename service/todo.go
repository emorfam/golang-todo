package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang-todo/domain"
	"golang-todo/internal/apierror"

	"github.com/google/uuid"
)

type todoService struct {
	repo TodoRepository
}

// NewTodoService creates a new todo service backed by the given repository.
// It returns a concrete type; the consuming package (handler/) defines the
// interface it needs.
func NewTodoService(repo TodoRepository) *todoService {
	return &todoService{repo: repo}
}

func (s *todoService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Todo, error) {
	todo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apierror.ErrNotFound
		}
		return nil, fmt.Errorf("getting todo by id: %w", err)
	}
	return todo, nil
}

func (s *todoService) List(ctx context.Context, filter domain.TodoFilter) ([]domain.Todo, error) {
	todos, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("listing todos: %w", err)
	}
	if todos == nil {
		todos = []domain.Todo{}
	}
	return todos, nil
}

func (s *todoService) Create(ctx context.Context, req CreateTodoRequest) (*domain.Todo, error) {
	if req.Title == "" {
		return nil, apierror.ErrBadRequest("title is required")
	}

	now := time.Now().UTC()
	todo := &domain.Todo{
		ID:          uuid.New(),
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.TodoStatusOpen,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, todo); err != nil {
		return nil, fmt.Errorf("creating todo: %w", err)
	}
	return todo, nil
}

func (s *todoService) Update(ctx context.Context, id uuid.UUID, req UpdateTodoRequest) (*domain.Todo, error) {
	todo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apierror.ErrNotFound
		}
		return nil, fmt.Errorf("getting todo for update: %w", err)
	}

	if req.Title == "" {
		return nil, apierror.ErrBadRequest("title is required")
	}
	if req.Status != domain.TodoStatusOpen && req.Status != domain.TodoStatusDone {
		return nil, apierror.ErrBadRequest("status must be 'open' or 'done'")
	}

	todo.Title = req.Title
	todo.Description = req.Description
	todo.Status = req.Status
	todo.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, todo); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apierror.ErrNotFound
		}
		return nil, fmt.Errorf("updating todo: %w", err)
	}
	return todo, nil
}

func (s *todoService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return apierror.ErrNotFound
		}
		return fmt.Errorf("deleting todo: %w", err)
	}
	return nil
}
