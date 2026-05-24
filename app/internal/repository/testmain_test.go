package repository_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/line_claude_observer_test?sslmode=disable"
	}

	var err error
	testPool, err = pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("failed to connect to test DB: %v", err)
	}
	defer testPool.Close()

	if err := runMigrations(testPool); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	os.Exit(m.Run())
}

func runMigrations(pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, "../../db/migrations")
}

func truncateTables(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "TRUNCATE TABLE messages, groups CASCADE")
	require.NoError(t, err)
}
