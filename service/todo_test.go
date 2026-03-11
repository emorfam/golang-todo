package service_test

import (
	"context"
	"testing"

	"golang-todo/domain"
	"golang-todo/internal/apierror"
	"golang-todo/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTodoRepository is a simple in-memory fake.
type fakeTodoRepository struct {
	todos map[uuid.UUID]*domain.Todo
}

func newFakeRepo() *fakeTodoRepository {
	return &fakeTodoRepository{todos: make(map[uuid.UUID]*domain.Todo)}
}

func (f *fakeTodoRepository) GetByID(_ context.Context, id uuid.UUID) (*domain.Todo, error) {
	t, ok := f.todos[id]
	if !ok {
		return nil, apierror.ErrNotFound
	}
	return t, nil
}

func (f *fakeTodoRepository) List(_ context.Context, filter domain.TodoFilter) ([]domain.Todo, error) {
	var result []domain.Todo
	for _, t := range f.todos {
		if filter.Status == nil || t.Status == *filter.Status {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (f *fakeTodoRepository) Create(_ context.Context, todo *domain.Todo) error {
	f.todos[todo.ID] = todo
	return nil
}

func (f *fakeTodoRepository) Update(_ context.Context, todo *domain.Todo) error {
	if _, ok := f.todos[todo.ID]; !ok {
		return apierror.ErrNotFound
	}
	f.todos[todo.ID] = todo
	return nil
}

func (f *fakeTodoRepository) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.todos[id]; !ok {
		return apierror.ErrNotFound
	}
	delete(f.todos, id)
	return nil
}

func TestTodoService_Create(t *testing.T) {
	svc := service.NewTodoService(newFakeRepo())

	todo, err := svc.Create(context.Background(), service.CreateTodoRequest{Title: "Buy milk"})
	require.NoError(t, err)
	assert.Equal(t, "Buy milk", todo.Title)
	assert.Equal(t, domain.TodoStatusOpen, todo.Status)
	assert.NotEqual(t, uuid.Nil, todo.ID)
}

func TestTodoService_Create_EmptyTitle(t *testing.T) {
	svc := service.NewTodoService(newFakeRepo())

	_, err := svc.Create(context.Background(), service.CreateTodoRequest{Title: ""})
	require.Error(t, err)
	var apiErr *apierror.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 400, apiErr.Code)
}

func TestTodoService_GetByID(t *testing.T) {
	repo := newFakeRepo()
	svc := service.NewTodoService(repo)

	created, err := svc.Create(context.Background(), service.CreateTodoRequest{Title: "Test"})
	require.NoError(t, err)

	got, err := svc.GetByID(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestTodoService_GetByID_NotFound(t *testing.T) {
	svc := service.NewTodoService(newFakeRepo())

	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, apierror.ErrNotFound)
}

func TestTodoService_Update(t *testing.T) {
	svc := service.NewTodoService(newFakeRepo())

	created, err := svc.Create(context.Background(), service.CreateTodoRequest{Title: "original"})
	require.NoError(t, err)

	updated, err := svc.Update(context.Background(), created.ID, service.UpdateTodoRequest{
		Title:  "updated",
		Status: domain.TodoStatusDone,
	})
	require.NoError(t, err)
	assert.Equal(t, "updated", updated.Title)
	assert.Equal(t, domain.TodoStatusDone, updated.Status)
}

func TestTodoService_Update_InvalidStatus(t *testing.T) {
	svc := service.NewTodoService(newFakeRepo())

	created, err := svc.Create(context.Background(), service.CreateTodoRequest{Title: "test"})
	require.NoError(t, err)

	_, err = svc.Update(context.Background(), created.ID, service.UpdateTodoRequest{
		Title:  "test",
		Status: "invalid",
	})
	require.Error(t, err)
	var apiErr *apierror.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 400, apiErr.Code)
}

func TestTodoService_Delete(t *testing.T) {
	svc := service.NewTodoService(newFakeRepo())

	created, err := svc.Create(context.Background(), service.CreateTodoRequest{Title: "test"})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(context.Background(), created.ID))

	_, err = svc.GetByID(context.Background(), created.ID)
	require.ErrorIs(t, err, apierror.ErrNotFound)
}

func TestTodoService_List(t *testing.T) {
	svc := service.NewTodoService(newFakeRepo())

	_, err := svc.Create(context.Background(), service.CreateTodoRequest{Title: "a"})
	require.NoError(t, err)
	created, err := svc.Create(context.Background(), service.CreateTodoRequest{Title: "b"})
	require.NoError(t, err)

	_, err = svc.Update(context.Background(), created.ID, service.UpdateTodoRequest{
		Title:  "b",
		Status: domain.TodoStatusDone,
	})
	require.NoError(t, err)

	all, err := svc.List(context.Background(), domain.TodoFilter{})
	require.NoError(t, err)
	assert.Len(t, all, 2)

	done := domain.TodoStatusDone
	doneList, err := svc.List(context.Background(), domain.TodoFilter{Status: &done})
	require.NoError(t, err)
	assert.Len(t, doneList, 1)
}
