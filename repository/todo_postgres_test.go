//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"golang-todo/db"
	"golang-todo/domain"
	"golang-todo/repository"
	"golang-todo/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// newPostgresRepo starts a real Postgres container and returns a repository
// connected to it. The container is terminated when the test finishes.
func newPostgresRepo(t *testing.T) service.TodoRepository {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	dsn := "postgres://testuser:testpass@" + host + ":" + port.Port() + "/testdb?sslmode=disable"

	database, err := db.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { database.Close() })

	return repository.NewTodoRepository(database, "postgres")
}

func TestTodoRepository_Postgres_CRUD(t *testing.T) {
	repo := newPostgresRepo(t)
	ctx := context.Background()

	// Create
	todo := &domain.Todo{
		ID:        uuid.New(),
		Title:     "Postgres integration test",
		Status:    domain.TodoStatusOpen,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, todo))

	// GetByID
	got, err := repo.GetByID(ctx, todo.ID)
	require.NoError(t, err)
	assert.Equal(t, todo.ID, got.ID)
	assert.Equal(t, todo.Title, got.Title)
	assert.Equal(t, domain.TodoStatusOpen, got.Status)

	// List
	todos, err := repo.List(ctx, domain.TodoFilter{})
	require.NoError(t, err)
	assert.Len(t, todos, 1)

	// List with filter
	open := domain.TodoStatusOpen
	filtered, err := repo.List(ctx, domain.TodoFilter{Status: &open})
	require.NoError(t, err)
	assert.Len(t, filtered, 1)

	// Update
	todo.Title = "Updated title"
	todo.Status = domain.TodoStatusDone
	todo.UpdatedAt = time.Now().UTC()
	require.NoError(t, repo.Update(ctx, todo))

	updated, err := repo.GetByID(ctx, todo.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated title", updated.Title)
	assert.Equal(t, domain.TodoStatusDone, updated.Status)

	// Delete
	require.NoError(t, repo.Delete(ctx, todo.ID))

	_, err = repo.GetByID(ctx, todo.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTodoRepository_Postgres_NotFound(t *testing.T) {
	repo := newPostgresRepo(t)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)

	err = repo.Update(ctx, &domain.Todo{ID: uuid.New(), Title: "x", Status: domain.TodoStatusOpen})
	require.ErrorIs(t, err, domain.ErrNotFound)

	err = repo.Delete(ctx, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)
}
