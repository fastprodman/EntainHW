package pgtestutil

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/file"
)

const (
	BaseDSN       = "postgres://myuser:mypassword@localhost:5432/postgres?sslmode=disable"
	migrationsDir = "cmd/migrator/migrations"
)

func NewTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	adminDSN, err := ReplaceDBInDSN(BaseDSN, "postgres")
	if err != nil {
		t.Fatalf("admin dsn: %v", err)
	}
	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}

	dbName := sanitizeForPgIdent(uniqueDBName("testdb", t.Name()))

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	const maxAttempts = 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		_, err = admin.ExecContext(ctx,
			fmt.Sprintf(`CREATE DATABASE "%s" WITH TEMPLATE template0 ENCODING 'UTF8'`, dbName))
		if err == nil {
			break
		}
		if !isUniqueViolation(err) || attempt == maxAttempts {
			_ = admin.Close()
			t.Fatalf("create database: %v", err)
		}
		dbName = sanitizeForPgIdent(uniqueDBName("testdb", t.Name()))
	}

	testDSN, err := ReplaceDBInDSN(BaseDSN, dbName)
	if err != nil {
		_ = admin.Close()
		t.Fatalf("test dsn: %v", err)
	}

	t.Logf("test dsn: %s", testDSN)

	db, err := sql.Open("pgx", testDSN)
	if err != nil {
		_ = admin.Close()
		t.Fatalf("open test db: %v", err)
	}

	db.SetConnMaxIdleTime(100 * time.Millisecond)
	db.SetConnMaxLifetime(30 * time.Second)

	// -- Apply migrations using file source with ABSOLUTE PATH (no file:// URL) --
	absPath, err := migrationsAbsPath()
	if err != nil {
		_ = db.Close()
		_ = admin.Close()
		t.Fatalf("resolve migrations path: %v", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		_ = db.Close()
		_ = admin.Close()
		t.Fatalf("postgres driver: %v", err)
	}

	src, err := (&file.File{}).Open(absPath)
	if err != nil {
		_ = db.Close()
		_ = admin.Close()
		t.Fatalf("open migrations dir: %v", err)
	}

	m, err := migrate.NewWithInstance("file", src, "postgres", driver)
	if err != nil {
		_ = db.Close()
		_ = admin.Close()
		t.Fatalf("migrate instance: %v", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		_ = db.Close()
		_ = admin.Close()
		t.Fatalf("migrate up: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
		time.Sleep(50 * time.Millisecond)

		dctx, dcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dcancel()

		_, derr := admin.ExecContext(dctx,
			fmt.Sprintf(`DROP DATABASE IF EXISTS "%s" WITH (FORCE)`, dbName))
		if derr == nil {
			_ = admin.Close()
			return
		}
		_, _ = admin.ExecContext(dctx, `
			SELECT pg_terminate_backend(pid)
			FROM pg_stat_activity
			WHERE datname = $1 AND pid <> pg_backend_pid()
		`, dbName)
		_, _ = admin.ExecContext(dctx,
			fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName))
		_ = admin.Close()
	}

	return db, cleanup
}

// ReplaceDBInDSN swaps the database name in a Postgres DSN (URL or keyword form).
func ReplaceDBInDSN(dsn, newDB string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse url fallback: %w", err)
	}

	u.Path = "/" + newDB
	return u.String(), nil
}

func migrationsAbsPath() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	// internal/infra/pgtestutil -> internal/infra -> internal -> repo root
	baseDir := filepath.Dir(thisFile)
	repoRoot := filepath.Join(baseDir, "..", "..", "..")
	abs := filepath.Join(repoRoot, migrationsDir)

	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", fmt.Errorf("abs migrations path: %w", err)
	}
	return abs, nil
}

func uniqueDBName(prefix, testName string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(testName))
	var rnd [6]byte
	_, _ = rand.Read(rnd[:])
	return fmt.Sprintf("%s_%08x_%s", prefix, h.Sum32(), hex.EncodeToString(rnd[:]))
}

func sanitizeForPgIdent(s string) string {
	s = strings.ToLower(s)
	repl := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_")
	s = repl.Replace(s)
	if len(s) <= 63 {
		return s
	}
	return s[:31] + "_" + s[len(s)-31:]
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return false
}
