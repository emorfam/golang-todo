package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	todohandler "golang-todo/handler"
	"golang-todo/domain"
	"golang-todo/internal/apierror"
	"golang-todo/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTodoService implements handler.TodoService for testing.
type fakeTodoService struct {
	todos map[uuid.UUID]*domain.Todo
}

func newFakeService() *fakeTodoService {
	return &fakeTodoService{todos: make(map[uuid.UUID]*domain.Todo)}
}

func (f *fakeTodoService) GetByID(_ context.Context, id uuid.UUID) (*domain.Todo, error) {
	t, ok := f.todos[id]
	if !ok {
		return nil, apierror.ErrNotFound
	}
	return t, nil
}

func (f *fakeTodoService) List(_ context.Context, _ domain.TodoFilter) ([]domain.Todo, error) {
	result := make([]domain.Todo, 0, len(f.todos))
	for _, t := range f.todos {
		result = append(result, *t)
	}
	return result, nil
}

func (f *fakeTodoService) Create(_ context.Context, req service.CreateTodoRequest) (*domain.Todo, error) {
	if req.Title == "" {
		return nil, apierror.ErrBadRequest("title is required")
	}
	todo := &domain.Todo{
		ID:          uuid.New(),
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.TodoStatusOpen,
	}
	f.todos[todo.ID] = todo
	return todo, nil
}

func (f *fakeTodoService) Update(_ context.Context, id uuid.UUID, req service.UpdateTodoRequest) (*domain.Todo, error) {
	t, ok := f.todos[id]
	if !ok {
		return nil, apierror.ErrNotFound
	}
	t.Title = req.Title
	t.Status = req.Status
	return t, nil
}

func (f *fakeTodoService) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.todos[id]; !ok {
		return apierror.ErrNotFound
	}
	delete(f.todos, id)
	return nil
}

// handlerTestSetup wires a chi router with the fake service (no auth, no DB).
// The parameter type is handler.TodoService (the interface defined in the consuming package).
func handlerTestSetup(svc todohandler.TodoService) *chi.Mux {
	r := chi.NewRouter()
	h := newTestHandler(svc)
	r.Get("/v1/todos", h.listTodos)
	r.Post("/v1/todos", h.createTodo)
	r.Get("/v1/todos/{id}", h.getTodo)
	r.Put("/v1/todos/{id}", h.updateTodo)
	r.Delete("/v1/todos/{id}", h.deleteTodo)
	return r
}

// testHandler is a minimal inline handler to avoid needing a real *sql.DB in tests.
type testHandler struct{ svc todohandler.TodoService }

func newTestHandler(svc todohandler.TodoService) *testHandler { return &testHandler{svc: svc} }

func respond(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *testHandler) listTodos(w http.ResponseWriter, r *http.Request) {
	todos, err := h.svc.List(r.Context(), domain.TodoFilter{})
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, todos)
}

func (h *testHandler) createTodo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	todo, err := h.svc.Create(r.Context(), service.CreateTodoRequest{Title: body.Title, Description: body.Description})
	if err != nil {
		var apiErr *apierror.APIError
		if ok := isAPIError(err, &apiErr); ok {
			respond(w, apiErr.Code, map[string]string{"error": apiErr.Message})
			return
		}
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusCreated, todo)
}

func (h *testHandler) getTodo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	todo, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		var apiErr *apierror.APIError
		if ok := isAPIError(err, &apiErr); ok {
			respond(w, apiErr.Code, map[string]string{"error": apiErr.Message})
			return
		}
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, todo)
}

func (h *testHandler) updateTodo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var body struct {
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	todo, err := h.svc.Update(r.Context(), id, service.UpdateTodoRequest{
		Title:  body.Title,
		Status: domain.TodoStatus(body.Status),
	})
	if err != nil {
		var apiErr *apierror.APIError
		if ok := isAPIError(err, &apiErr); ok {
			respond(w, apiErr.Code, map[string]string{"error": apiErr.Message})
			return
		}
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, todo)
}

func (h *testHandler) deleteTodo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		var apiErr *apierror.APIError
		if ok := isAPIError(err, &apiErr); ok {
			respond(w, apiErr.Code, map[string]string{"error": apiErr.Message})
			return
		}
		respond(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func isAPIError(err error, target **apierror.APIError) bool {
	if e, ok := err.(*apierror.APIError); ok {
		*target = e
		return true
	}
	return false
}

// ---- Tests ----

func TestListTodos(t *testing.T) {
	svc := newFakeService()
	_, _ = svc.Create(context.Background(), service.CreateTodoRequest{Title: "one"})
	_, _ = svc.Create(context.Background(), service.CreateTodoRequest{Title: "two"})

	r := handlerTestSetup(svc)
	req := httptest.NewRequest(http.MethodGet, "/v1/todos", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var todos []domain.Todo
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &todos))
	assert.Len(t, todos, 2)
}

func TestCreateTodo(t *testing.T) {
	svc := newFakeService()
	r := handlerTestSetup(svc)

	body, _ := json.Marshal(map[string]string{"title": "Buy milk", "description": "2% fat"})
	req := httptest.NewRequest(http.MethodPost, "/v1/todos", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var todo domain.Todo
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &todo))
	assert.Equal(t, "Buy milk", todo.Title)
	assert.Equal(t, domain.TodoStatusOpen, todo.Status)
}

func TestCreateTodo_EmptyTitle(t *testing.T) {
	svc := newFakeService()
	r := handlerTestSetup(svc)

	body, _ := json.Marshal(map[string]string{"title": ""})
	req := httptest.NewRequest(http.MethodPost, "/v1/todos", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetTodo(t *testing.T) {
	svc := newFakeService()
	created, _ := svc.Create(context.Background(), service.CreateTodoRequest{Title: "test"})

	r := handlerTestSetup(svc)
	req := httptest.NewRequest(http.MethodGet, "/v1/todos/"+created.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got domain.Todo
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, created.ID, got.ID)
}

func TestGetTodo_NotFound(t *testing.T) {
	svc := newFakeService()
	r := handlerTestSetup(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/todos/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteTodo(t *testing.T) {
	svc := newFakeService()
	created, _ := svc.Create(context.Background(), service.CreateTodoRequest{Title: "delete me"})

	r := handlerTestSetup(svc)
	req := httptest.NewRequest(http.MethodDelete, "/v1/todos/"+created.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}
