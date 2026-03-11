package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"golang-todo/domain"
	"golang-todo/internal/apierror"
	"golang-todo/service"

	"github.com/google/uuid"
)

// TodoService defines what the handler layer needs from the business logic layer.
// It is intentionally defined here (the consuming package) rather than in
// service/, following the "accept interfaces, return structs" convention.
type TodoService interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Todo, error)
	List(ctx context.Context, filter domain.TodoFilter) ([]domain.Todo, error)
	Create(ctx context.Context, req service.CreateTodoRequest) (*domain.Todo, error)
	Update(ctx context.Context, id uuid.UUID, req service.UpdateTodoRequest) (*domain.Todo, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	todos TodoService
	db    *sql.DB
}

// New creates a Handler with the given dependencies.
func New(todos TodoService, db *sql.DB) *Handler {
	return &Handler{todos: todos, db: db}
}

// respond encodes v as JSON and writes it with the given HTTP status code.
func respond(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decode validates Content-Type and decodes the JSON request body into v.
// w is required so that http.MaxBytesReader can write a 413 response on overflow.
func decode(w http.ResponseWriter, r *http.Request, v interface{}) error {
	ct := r.Header.Get("Content-Type")
	if ct != "application/json" {
		return apierror.ErrBadRequest("Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return apierror.ErrBadRequest("invalid JSON body: " + err.Error())
	}
	return nil
}

// mapError converts an error to the appropriate HTTP status and error message.
func mapError(w http.ResponseWriter, err error) {
	var apiErr *apierror.APIError
	if errors.As(err, &apiErr) {
		respond(w, apiErr.Code, map[string]string{"error": apiErr.Message})
		return
	}
	respond(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
