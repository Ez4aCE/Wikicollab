# WikiCollab

A collaborative Markdown wiki application — college minor project.

**Stack:** React · TypeScript · Go · PostgreSQL · Redis · WebSockets · Docker Compose

---

## ⚡ One-command setup (Docker)

```bash
# 1. Copy environment config
cp .env.example .env

# 2. Start all services (Postgres, Redis, Go backend, React frontend)
docker compose up --build
```

Then open **http://localhost:5173** in your browser.

> The `SESSION_SECRET` in `.env` must not be empty. The default from `.env.example` is fine for development.

---

## Manual / Development Setup

### Prerequisites

| Tool | Version |
|------|---------|
| Go   | 1.22+   |
| Node | 20+     |
| PostgreSQL | 16+ |
| Redis | 7+    |

### 1. Configure environment

```bash
cp .env.example .env
# Edit .env with your local Postgres and Redis URLs
```

### 2. Start PostgreSQL & Redis

```bash
# Using Docker for just the infrastructure:
docker compose up -d postgres redis
```

### 3. Run the backend

```bash
cd backend
go run main.go
```

Migrations run automatically on startup. The backend listens on `http://localhost:8080`.

### 4. Run the frontend

```bash
cd frontend
npm install
npm run dev
```

Open **http://localhost:5173**.

---

## Running Tests

### Backend tests (require a running Postgres)

```bash
cd backend
TEST_DATABASE_URL=postgres://wikicollab:wikicollab@localhost:5432/wikicollab_test?sslmode=disable \
  go test ./... -v
```

### Frontend type check + production build

```bash
cd frontend
npx tsc --noEmit
npm run build
```

### Playwright E2E tests (full stack must be running first)

```bash
# From repo root — install Playwright browsers once:
npx playwright install chromium

# Run all E2E tests:
npm run test:e2e

# Run with interactive UI:
npm run test:e2e:ui
```

---

## Application Routes

| Route | Description |
|-------|-------------|
| `/login` | Login page |
| `/register` | Register new account |
| `/dashboard` | Your wikis |
| `/wiki/:wikiID` | Wiki's pages |
| `/wiki/:wikiID/page/:pageID` | Collaborative Markdown editor |

Protected routes redirect unauthenticated users to `/login`.

---

## Architecture

```
Browser (React + TypeScript + Vite)
        │
        │   REST (credentials: include)
        │   WebSocket (session cookie)
        ▼
Go Backend (gorilla/mux)
        │
        ├── PostgreSQL — users, wikis, pages, revisions
        └── Redis — TTL block locks
```

### Real-time collaboration

- Opening a page connects to `ws://localhost:8080/ws/pages/:id`
- Click any Markdown block to acquire a Redis-backed TTL lock on that block
- While you hold the lock, other users see a **🔒 Locked by You** badge
- The lock auto-renews every 10 s; it expires in 30 s if the browser disconnects
- Saving broadcasts the updated Markdown to all connected users

---

## Project Members

| Member | Responsibility |
|--------|----------------|
| Member 1 | Go backend, REST APIs, PostgreSQL, migrations, authentication |
| Member 2 | Redis locking, WebSocket server, real-time events |
| Member 3 | React Markdown editor, block parsing, WebSocket client integration |
| Member 4 | React application structure, routing, auth UI, dashboard, wiki/page navigation |
| QA       | End-to-end testing, Playwright suite, bug fixes |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | — | PostgreSQL connection string (required) |
| `REDIS_URL` | `redis://localhost:6379` | Redis connection string |
| `SESSION_SECRET` | — | Cookie signing secret (required, keep secret!) |
| `CORS_ORIGIN` | `http://localhost:5173` | Allowed CORS origin |
| `PORT` | `8080` | Backend HTTP port |
| `LOCK_TTL` | `30s` | How long a block lock lives without renewal |
| `LOCK_RENEW_INTERVAL` | `10s` | How often the client renews its lock |
