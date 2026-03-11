package repository_test

import (
	"context"
	"testing"

	"golang-todo/db"
	"golang-todo/domain"
	"golang-todo/repository"
	"golang-todo/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRepo opens an in-memory SQLite database, runs migrations, and returns
// the repository typed as service.TodoRepository (the interface it satisfies).
func newTestRepo(t *testing.T) service.TodoRepository {
	t.Helper()
	database, err := db.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { database.Close() })
	return repository.NewTodoRepository(database, "sqlite")
}

func TestTodoRepository_Create_GetByID(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	todo := &domain.Todo{
		ID:     uuid.New(),
		Title:  "Buy groceries",
		Status: domain.TodoStatusOpen,
	}
	require.NoError(t, repo.Create(ctx, todo))

	got, err := repo.GetByID(ctx, todo.ID)
	require.NoError(t, err)
	assert.Equal(t, todo.ID, got.ID)
	assert.Equal(t, todo.Title, got.Title)
	assert.Equal(t, domain.TodoStatusOpen, got.Status)
}

func TestTodoRepository_GetByID_NotFound(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTodoRepository_List(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	for range 3 {
		require.NoError(t, repo.Create(ctx, &domain.Todo{
			ID:     uuid.New(),
			Title:  "todo",
			Status: domain.TodoStatusOpen,
		}))
	}
	require.NoError(t, repo.Create(ctx, &domain.Todo{
		ID:     uuid.New(),
		Title:  "done todo",
		Status: domain.TodoStatusDone,
	}))

	all, err := repo.List(ctx, domain.TodoFilter{})
	require.NoError(t, err)
	assert.Len(t, all, 4)

	done := domain.TodoStatusDone
	filtered, err := repo.List(ctx, domain.TodoFilter{Status: &done})
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
}

func TestTodoRepository_Update(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	todo := &domain.Todo{ID: uuid.New(), Title: "original", Status: domain.TodoStatusOpen}
	require.NoError(t, repo.Create(ctx, todo))

	todo.Title = "updated"
	todo.Status = domain.TodoStatusDone
	require.NoError(t, repo.Update(ctx, todo))

	got, err := repo.GetByID(ctx, todo.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated", got.Title)
	assert.Equal(t, domain.TodoStatusDone, got.Status)
}

func TestTodoRepository_Update_NotFound(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.Update(ctx, &domain.Todo{ID: uuid.New(), Title: "x", Status: domain.TodoStatusOpen})
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTodoRepository_Delete(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	todo := &domain.Todo{ID: uuid.New(), Title: "to delete", Status: domain.TodoStatusOpen}
	require.NoError(t, repo.Create(ctx, todo))
	require.NoError(t, repo.Delete(ctx, todo.ID))

	_, err := repo.GetByID(ctx, todo.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTodoRepository_Delete_NotFound(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.Delete(ctx, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)
}
