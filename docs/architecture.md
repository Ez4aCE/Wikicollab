# WikiCollab Architecture

## Overview

WikiCollab is a Markdown-based wiki with real-time block-level collaboration.

```
React (Vite + TypeScript)
        |
        | REST API     WebSocket (/ws/pages/:id)
        |                   |
        +-------------------+
                    |
                   Go
                /       \
               /         \
          PostgreSQL     Redis
          (persistent)   (TTL locks — Member 2)
```

## Backend Layers

```
HTTP Request
     |
     v
[CORS Middleware]
     |
     v
[Auth Middleware] — reads session cookie, injects user into context
     |
     v
[Handler] — decodes request, calls service, encodes response
     |
     v
[Service] — business logic, validation, transactions
     |
     v
[PostgreSQL] — parameterized queries via database/sql
```

## Package Structure

```
backend/
├── main.go              — wires everything, starts server
├── config/
│   └── config.go        — loads env vars
├── database/
│   └── postgres.go      — connection + migration runner
├── models/
│   ├── user.go
│   ├── wiki.go
│   ├── page.go
│   └── revision.go
├── middleware/
│   └── auth.go          — session → user → context
├── services/
│   ├── errors.go        — sentinel errors
│   ├── auth.go          — register, login
│   ├── wiki.go          — CRUD + ownership checks
│   ├── page.go          — CRUD + version check + transactions
│   └── revision.go      — list revisions
└── handlers/
    ├── helpers.go        — writeJSON, writeError, decodeJSON
    ├── health.go
    ├── auth.go
    ├── wiki.go
    ├── page.go
    └── revision.go
```

## Database Schema

```
users
  id            UUID PK
  username      VARCHAR(50) UNIQUE NOT NULL
  email         VARCHAR(255) UNIQUE NOT NULL
  password_hash TEXT NOT NULL
  created_at    TIMESTAMPTZ NOT NULL

wikis
  id         UUID PK
  name       VARCHAR(255) NOT NULL
  owner_id   UUID FK→users.id NOT NULL
  created_at TIMESTAMPTZ NOT NULL

pages
  id         UUID PK
  wiki_id    UUID FK→wikis.id NOT NULL
  title      VARCHAR(255) NOT NULL
  content    TEXT NOT NULL          ← Markdown source of truth
  version    INTEGER NOT NULL DEFAULT 1
  created_at TIMESTAMPTZ NOT NULL
  updated_at TIMESTAMPTZ NOT NULL

revisions
  id         UUID PK
  page_id    UUID FK→pages.id NOT NULL
  content    TEXT NOT NULL
  version    INTEGER NOT NULL
  created_by UUID FK→users.id NOT NULL
  created_at TIMESTAMPTZ NOT NULL
```

## Session Strategy

Authentication uses **gorilla/sessions** with a signed cookie store (`SESSION_SECRET`).
Sessions are HTTP-only and cannot be accessed by JavaScript.
No Redis dependency for auth — Member 2 can add Redis-backed sessions later if needed.

## Optimistic Concurrency

Page updates use optimistic locking:

1. Client reads page at `version = N`
2. Client sends update with `expected_version = N`
3. Server does `SELECT version FROM pages WHERE id=? FOR UPDATE`
4. If `version != expected_version` → `409 Conflict`
5. If match → `UPDATE pages SET version = N+1` + `INSERT INTO revisions`
6. Both in a single transaction — atomically

## Integration Points for Other Members

### Member 2 (WebSocket + Redis)

`services.PageService` is intentionally decoupled from HTTP. Member 2 can call it from WebSocket handlers:

```go
pageSvc := services.NewPageService(db)
updatedPage, err := pageSvc.UpdatePage(ctx, services.UpdatePageInput{...})
```

The WebSocket endpoint (`/ws/pages/:id`) should be added alongside the existing routes in `main.go`.

Redis lock keys use the format:
```
lock:page:{pageID}:block:{blockID}
```

### Member 3 (Markdown Editor)

The `content` field of a page is the raw Markdown string. The editor should:
1. GET `/api/pages/:id` to load content
2. Parse Markdown into blocks in memory (not stored separately)
3. PUT `/api/pages/:id` to save with `expected_version`

### Member 4 (React Frontend)

All auth uses the `wikicollab-session` cookie — React does not need to manage tokens.
`credentials: 'include'` must be set on all fetch calls.

See `docs/api.md` for the full endpoint reference.
