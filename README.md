# SpendSense

A full-stack personal and group expense tracker: a **Go REST API** backed by
**PostgreSQL**, with a React frontend. Features JWT authentication, role-based access,
categorized expense tracking, monthly budgets with spending progress, equal/unequal/
percentage expense splitting with member settlements, spending analytics via SQL
aggregation, and CSV export. Containerized with Docker and shipped through a Jenkins
CI/CD pipeline to Kubernetes.

## Stack

| Layer        | Technology |
|--------------|------------|
| Frontend     | React, React Router, Tailwind CSS, Axios, Recharts |
| Backend      | Go (chi router, pgx), JWT (`golang-jwt`), bcrypt |
| Database     | PostgreSQL 16 |
| Logging      | slog (JSON) → optional Logstash → ELK |
| Tests        | Go `testing` + `httptest`, testcontainers-go |
| Container/CI | Docker, Docker Compose, Kubernetes, Jenkins, Vault |

## Backend layout

```
backend/
  main.go                  entrypoint: config, DB connect, HTTP server
  internal/
    config/                env configuration
    db/                    pgx pool + schema.sql (applied on startup)
    router/                all /api routes
    middleware/            JWT auth (Protect, AdminOnly)
    auth/ expense/ budget/ group/ analytics/ notification/ admin/ export/
    notify/                shared notification helper
    logger/ httputil/ models/
    apitest/               HTTP integration tests
```

## Running locally

With Docker Compose (Postgres + backend + frontend):

```bash
docker compose up --build
# frontend → http://localhost:3000, API → http://localhost:5000/api
```

Backend only (needs a running Postgres):

```bash
cd backend
cp .env.example .env         # edit DATABASE_URL / JWT_SECRET
go run .
```

## Tests

```bash
cd backend
go test ./...                # spins up Postgres via testcontainers (Docker required)
# or point at an existing database:
TEST_DATABASE_URL="postgres://user@localhost:5432/spendsense_test?sslmode=disable" go test ./...
```

## API

Base path `/api`. JWT in `Authorization: Bearer <token>`.

- `POST /auth/register`, `POST /auth/login`
- `GET|POST /expenses`, `PUT|DELETE /expenses/{id}`
- `GET|POST /budgets`, `DELETE /budgets/{id}`
- `GET|POST /groups`, `POST /groups/{id}/members`, `GET|POST /groups/{id}/expenses`,
  `PUT /groups/{id}/expenses/{expenseId}/split/{memberId}/pay`, `POST /groups/{id}/settle`
- `GET /analytics/{category-breakdown,monthly-trend,income-vs-expense}`
- `GET /notifications`, `PUT /notifications/{id}/read`, `PUT /notifications/read-all`
- `GET /admin/{users,expenses,notifications}`, `PUT /admin/users/{id}/toggle` (admin only)
- `GET /export/csv`
- `GET /api/health`
