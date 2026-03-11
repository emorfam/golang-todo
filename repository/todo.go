package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang-todo/domain"
	"golang-todo/metrics"

	"github.com/google/uuid"
)

type sqlTodoRepository struct {
	db     *sql.DB
	driver string // "sqlite" or "postgres" — used for placeholder rebinding
}

// NewTodoRepository returns a new SQL-backed todo repository.
// driver must be "sqlite" or "postgres" so that query placeholders are
// translated correctly (SQLite uses ?, Postgres uses $1, $2, …).
func NewTodoRepository(db *sql.DB, driver string) *sqlTodoRepository {
	return &sqlTodoRepository{db: db, driver: driver}
}

// rebind converts ? placeholders to $N for Postgres; is a no-op for SQLite.
func (r *sqlTodoRepository) rebind(query string) string {
	if r.driver != "postgres" {
		return query
	}
	n := 0
	var b strings.Builder
	for _, ch := range query {
		if ch == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func (r *sqlTodoRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Todo, error) {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("todo_get_by_id").Observe(time.Since(start).Seconds())
	}()

	q := r.rebind(`SELECT id, title, description, status, created_at, updated_at FROM todos WHERE id = ?`)

	row := r.db.QueryRowContext(ctx, q, id.String())
	todo, err := scanTodo(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("querying todo %s: %w", id, err)
	}
	return todo, nil
}

func (r *sqlTodoRepository) List(ctx context.Context, filter domain.TodoFilter) ([]domain.Todo, error) {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("todo_list").Observe(time.Since(start).Seconds())
	}()

	q := `SELECT id, title, description, status, created_at, updated_at FROM todos`
	var args []interface{}

	if filter.Status != nil {
		q += ` WHERE status = ?`
		args = append(args, string(*filter.Status))
	}
	q += ` ORDER BY created_at DESC`
	q = r.rebind(q)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing todos: %w", err)
	}
	defer rows.Close()

	var todos []domain.Todo
	for rows.Next() {
		todo, err := scanTodo(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning todo: %w", err)
		}
		todos = append(todos, *todo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating todo rows: %w", err)
	}
	return todos, nil
}

func (r *sqlTodoRepository) Create(ctx context.Context, todo *domain.Todo) error {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("todo_create").Observe(time.Since(start).Seconds())
	}()

	q := r.rebind(`INSERT INTO todos (id, title, description, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`)

	_, err := r.db.ExecContext(ctx, q,
		todo.ID.String(),
		todo.Title,
		todo.Description,
		string(todo.Status),
		todo.CreatedAt.UTC(),
		todo.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("inserting todo: %w", err)
	}
	return nil
}

func (r *sqlTodoRepository) Update(ctx context.Context, todo *domain.Todo) error {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("todo_update").Observe(time.Since(start).Seconds())
	}()

	q := r.rebind(`UPDATE todos SET title = ?, description = ?, status = ?, updated_at = ? WHERE id = ?`)

	res, err := r.db.ExecContext(ctx, q,
		todo.Title,
		todo.Description,
		string(todo.Status),
		todo.UpdatedAt.UTC(),
		todo.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("updating todo: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *sqlTodoRepository) Delete(ctx context.Context, id uuid.UUID) error {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("todo_delete").Observe(time.Since(start).Seconds())
	}()

	q := r.rebind(`DELETE FROM todos WHERE id = ?`)

	res, err := r.db.ExecContext(ctx, q, id.String())
	if err != nil {
		return fmt.Errorf("deleting todo: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// scanner abstracts sql.Row and sql.Rows for the scanTodo helper.
type scanner interface {
	Scan(dest ...interface{}) error
}

func scanTodo(s scanner) (*domain.Todo, error) {
	var (
		idStr       string
		title       string
		description string
		status      string
		createdAt   time.Time
		updatedAt   time.Time
	)
	if err := s.Scan(&idStr, &title, &description, &status, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("parsing uuid %q: %w", idStr, err)
	}
	return &domain.Todo{
		ID:          id,
		Title:       title,
		Description: description,
		Status:      domain.TodoStatus(status),
		CreatedAt:   createdAt.UTC(),
		UpdatedAt:   updatedAt.UTC(),
	}, nil
}
