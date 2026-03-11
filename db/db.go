package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open opens a database connection, runs migrations, and returns the *sql.DB.
func Open(driver, dsn string) (*sql.DB, error) {
	var driverName string
	var gooseDialect string

	switch driver {
	case "sqlite":
		driverName = "sqlite"
		gooseDialect = "sqlite3"
	case "postgres":
		driverName = "pgx"
		gooseDialect = "postgres"
	default:
		return nil, fmt.Errorf("unsupported DB driver %q", driver)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	goose.SetBaseFS(migrations)
	if err := goose.SetDialect(gooseDialect); err != nil {
		return nil, fmt.Errorf("setting goose dialect: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}
