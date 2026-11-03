package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rohithsandiri/spendsense-backend/internal/db"
	"github.com/rohithsandiri/spendsense-backend/internal/router"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const jwtSecret = "test_secret"

var (
	pool *pgxpool.Pool
	srv  http.Handler
)

// TestMain brings up Postgres (TEST_DATABASE_URL or a testcontainer), applies the schema,
// and builds the router once for all tests.
func TestMain(m *testing.M) {
	ctx := context.Background()

	var (
		cleanup func()
		dsn     string
	)
	if env := os.Getenv("TEST_DATABASE_URL"); env != "" {
		dsn = env
	} else {
		container, err := tcpostgres.Run(ctx, "postgres:16",
			tcpostgres.WithDatabase("spendsense"),
			tcpostgres.WithUsername("spendsense"),
			tcpostgres.WithPassword("spendsense"),
			testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
		)
		if err != nil {
			panic("start postgres container: " + err.Error())
		}
		dsn, err = container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			panic("connection string: " + err.Error())
		}
		cleanup = func() { _ = container.Terminate(ctx) }
	}

	p, err := db.Connect(ctx, dsn)
	if err != nil {
		panic("connect db: " + err.Error())
	}
	pool = p
	srv = router.New(pool, jwtSecret)

	code := m.Run()

	pool.Close()
	if cleanup != nil {
		cleanup()
	}
	os.Exit(code)
}

// reset truncates all tables so each test starts clean.
func reset(t *testing.T) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`TRUNCATE users, expenses, budgets, groups, group_members, group_expenses, split_details, notifications RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
}

// do performs an HTTP request against the in-memory server and returns status + decoded JSON.
func do(t *testing.T, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var out map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

// doArray is like do but for endpoints returning a JSON array.
func doArray(t *testing.T, method, path, token string, body any) (int, []map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var out []map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

// registerUser creates a user and returns its token.
func registerUser(t *testing.T, name, email, password string) string {
	t.Helper()
	status, body := do(t, http.MethodPost, "/api/auth/register", "",
		map[string]string{"name": name, "email": email, "password": password})
	if status != http.StatusCreated {
		t.Fatalf("register %s: status %d body %v", email, status, body)
	}
	return body["token"].(string)
}
