package handler

import (
	"net/http"

	"golang-todo/domain"
	"golang-todo/internal/apierror"
	"golang-todo/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Health godoc
// @Summary Liveness probe
// @Tags system
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready godoc
// @Summary Readiness probe
// @Tags system
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /ready [get]
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if _, err := h.db.ExecContext(r.Context(), "SELECT 1"); err != nil {
		respond(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListTodos godoc
// @Summary List all todos
// @Tags todos
// @Produce json
// @Param status query string false "Filter by status (open|done)"
// @Success 200 {array} domain.Todo
// @Failure 401 {object} map[string]string
// @Security BearerAuth
// @Router /v1/todos [get]
func (h *Handler) ListTodos(w http.ResponseWriter, r *http.Request) {
	var filter domain.TodoFilter

	if s := r.URL.Query().Get("status"); s != "" {
		switch domain.TodoStatus(s) {
		case domain.TodoStatusOpen, domain.TodoStatusDone:
			status := domain.TodoStatus(s)
			filter.Status = &status
		default:
			mapError(w, apierror.ErrBadRequest("status must be 'open' or 'done'"))
			return
		}
	}

	todos, err := h.todos.List(r.Context(), filter)
	if err != nil {
		mapError(w, err)
		return
	}
	respond(w, http.StatusOK, todos)
}

// CreateTodo godoc
// @Summary Create a todo
// @Tags todos
// @Accept json
// @Produce json
// @Param body body createTodoBody true "Todo to create"
// @Success 201 {object} domain.Todo
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Security BearerAuth
// @Router /v1/todos [post]
func (h *Handler) CreateTodo(w http.ResponseWriter, r *http.Request) {
	var body createTodoBody
	if err := decode(w, r, &body); err != nil {
		mapError(w, err)
		return
	}

	todo, err := h.todos.Create(r.Context(), service.CreateTodoRequest{
		Title:       body.Title,
		Description: body.Description,
	})
	if err != nil {
		mapError(w, err)
		return
	}
	respond(w, http.StatusCreated, todo)
}

// GetTodo godoc
// @Summary Get a single todo
// @Tags todos
// @Produce json
// @Param id path string true "Todo ID"
// @Success 200 {object} domain.Todo
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /v1/todos/{id} [get]
func (h *Handler) GetTodo(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r)
	if err != nil {
		mapError(w, err)
		return
	}

	todo, err := h.todos.GetByID(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	respond(w, http.StatusOK, todo)
}

// UpdateTodo godoc
// @Summary Update a todo
// @Tags todos
// @Accept json
// @Produce json
// @Param id path string true "Todo ID"
// @Param body body updateTodoBody true "Todo updates"
// @Success 200 {object} domain.Todo
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /v1/todos/{id} [put]
func (h *Handler) UpdateTodo(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r)
	if err != nil {
		mapError(w, err)
		return
	}

	var body updateTodoBody
	if err := decode(w, r, &body); err != nil {
		mapError(w, err)
		return
	}

	todo, err := h.todos.Update(r.Context(), id, service.UpdateTodoRequest{
		Title:       body.Title,
		Description: body.Description,
		Status:      domain.TodoStatus(body.Status),
	})
	if err != nil {
		mapError(w, err)
		return
	}
	respond(w, http.StatusOK, todo)
}

// DeleteTodo godoc
// @Summary Delete a todo
// @Tags todos
// @Produce json
// @Param id path string true "Todo ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /v1/todos/{id} [delete]
func (h *Handler) DeleteTodo(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r)
	if err != nil {
		mapError(w, err)
		return
	}

	if err := h.todos.Delete(r.Context(), id); err != nil {
		mapError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Request body types.

type createTodoBody struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type updateTodoBody struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

func parseUUID(r *http.Request) (uuid.UUID, error) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, apierror.ErrBadRequest("invalid id: must be a UUID")
	}
	return id, nil
}
