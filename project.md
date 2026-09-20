# WikiCollab — Project Specification

## 1. Project Overview

WikiCollab is a small college/minor project: a Markdown-based wiki with simple real-time collaboration.

Users can create wikis and pages, edit Markdown, preview rendered Markdown, view revision history, and collaborate with other users in real time.

The collaboration model is intentionally simple:

- Users edit individual Markdown blocks/paragraphs.
- A block can be locked by one user at a time.
- Locks are stored in Redis with a short TTL.
- WebSockets broadcast lock status and saved block/page updates.
- We are NOT implementing OT or CRDT.

The project should prioritize simplicity, readability, maintainability, and a working end-to-end demo over production-scale features.

---

## 2. Goals

### Must Have

1. User registration/login/logout.
2. Create and view wikis.
3. Create, view, and edit wiki pages.
4. Markdown editing.
5. Markdown preview.
6. Basic page navigation.
7. Basic revision history.
8. Paragraph/block-level editing locks.
9. Redis TTL-based locks.
10. WebSocket-based real-time collaboration.
11. Multiple users can edit different blocks of the same page simultaneously.
12. A user cannot edit a block currently locked by another user.
13. Locks automatically expire if a client disappears.
14. Dockerized development environment.

### Explicitly Out of Scope

Do NOT implement these unless explicitly requested later:

- OT (Operational Transformation)
- CRDT
- Google Docs-style character-level conflict-free editing
- Offline editing
- Cursor/selection synchronization
- Comments
- Mentions
- Notifications
- File/image uploads
- Elasticsearch
- Microservices
- Kubernetes
- Redis clustering
- Advanced analytics
- Complex enterprise permissions
- Complex page trees
- Version branching
- Real-time synchronization of every keystroke

Keep the project appropriate for a minor college project.

---

## 3. Technology Stack

### Frontend

- React
- TypeScript
- Vite
- React Router if routing is needed
- Markdown parser compatible with GitHub-flavored Markdown
- CSS or a lightweight styling solution

### Backend

- Go
- HTTP REST API
- WebSockets

### Database

- PostgreSQL

### Real-time / Ephemeral State

- Redis
- Redis TTL keys for block locks

### Infrastructure

- Docker
- Docker Compose

---

## 4. High-Level Architecture

```text
                         React
                    /             \
                 REST           WebSocket
                  |                 |
                  +--------+--------+
                           |
                           v
                          Go
                       /      \
                      /        \
                     v          v
              PostgreSQL      Redis
              persistent      TTL locks
                 data         ephemeral state
```

Docker Compose should initially contain:

```text
frontend
backend
postgres
redis
```

Do not add infrastructure unless it is genuinely required.

---

## 5. Repository Structure

Preferred monorepo structure:

```text
wikicollab/
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── services/
│   │   ├── hooks/
│   │   ├── types/
│   │   └── App.tsx
│   ├── package.json
│   └── ...
│
├── backend/
│   ├── main.go
│   ├── handlers/
│   ├── services/
│   ├── middleware/
│   ├── websocket/
│   ├── redis/
│   ├── database/
│   └── ...
│
├── migrations/
├── docs/
│   ├── architecture.md
│   ├── api.md
│   └── websocket.md
│
├── docker-compose.yml
├── .env.example
├── README.md
└── project.md
```

Do not create excessive packages or abstractions.

---

## 6. Database Design

Keep the PostgreSQL schema small.

Core tables:

```text
users
wikis
pages
revisions
```

### users

```text
id
username
email
password_hash
created_at
```

### wikis

```text
id
name
owner_id
created_at
```

`owner_id` references `users.id`.

### pages

```text
id
wiki_id
title
content
version
created_at
updated_at
```

`content` contains the Markdown source.

`version` is an integer used for optimistic concurrency/version tracking.

### revisions

```text
id
page_id
content
version
created_by
created_at
```

Every meaningful save can create a revision.

Do not store rendered HTML as the source of truth.

The source of truth is Markdown.

---

## 7. Markdown Model

The database stores one Markdown document per page.

Example:

```markdown
# Operating Systems

An operating system manages computer resources.

## Process Management

A process is a running program.
```

The frontend parses the Markdown into editable blocks in memory.

Conceptually:

```text
Markdown
   |
   v
React block representation
   |
   +-- block-1: heading
   +-- block-2: paragraph
   +-- block-3: heading
   +-- block-4: paragraph
```

Each editable block needs a stable ID for the current document/session.

Do NOT create a PostgreSQL row for every paragraph/block unless explicitly requested later.

The project is intentionally using block-level collaboration without creating a complex document storage model.

---

## 8. Collaboration Model

### Core Rule

Only one user can edit a specific block at a time.

Different users can edit different blocks simultaneously.

Example:

```text
Page
├── Block A — locked by Alice
├── Block B — unlocked
├── Block C — locked by Bob
└── Block D — unlocked
```

Alice can edit A while Bob edits C.

Bob cannot edit A while Alice owns its lock.

This is intentionally NOT conflict-free character-level collaboration.

---

## 9. Redis Lock Design

Redis is used only for ephemeral locks and possibly simple presence state.

Lock key:

```text
lock:page:{pageID}:block:{blockID}
```

Example:

```text
lock:page:42:block:abc123
```

The value should contain a unique lock token, and optionally user/connection information.

Recommended conceptual value:

```json
{
  "user_id": "user-123",
  "token": "unique-lock-token"
}
```

### TTL

Recommended defaults:

```text
LOCK_TTL=30s
LOCK_RENEW_INTERVAL=10s
```

Make these configurable through environment variables.

### Acquire

Use an atomic Redis operation equivalent to:

```text
SET key token NX EX 30
```

Do NOT implement:

```text
GET key
if missing:
    SET key
```

because that creates a race condition.

### Renew

Only the owner of the lock may renew it.

The server must verify the lock token before renewing.

### Release

Only the owner may release the lock.

Do not blindly `DEL` the key.

There is a race where a lock can expire, another user can acquire it, and the previous user can send a delayed release request.

Use an atomic ownership check before deleting the key.

A small Redis Lua script is acceptable for safe release.

### Expiration

If a browser crashes or disconnects without releasing its locks:

```text
no renewal
    |
    v
30 seconds
    |
    v
Redis expires lock
    |
    v
block becomes editable
```

TTL is the final safety mechanism.

---

## 10. WebSocket Design

WebSocket endpoint:

```text
/ws/pages/:pageID
```

WebSockets are used for:

1. Joining a page collaboration room.
2. Lock acquire/release/renew.
3. Broadcasting lock status.
4. Broadcasting saved block/page updates.
5. Basic connection/presence status if useful.

Do not send every keystroke to the server.

Changes can be sent when a block is saved or at a reasonable debounce/autosave interval.

---

## 11. WebSocket Messages

Use JSON messages.

### Join

```json
{
  "type": "join",
  "page_id": "42"
}
```

### Acquire Lock

```json
{
  "type": "lock",
  "block_id": "block-123"
}
```

### Lock Acquired

```json
{
  "type": "lock_acquired",
  "block_id": "block-123",
  "user_id": "user-42",
  "username": "Alice"
}
```

### Lock Denied

```json
{
  "type": "lock_denied",
  "block_id": "block-123",
  "user_id": "user-42",
  "username": "Alice"
}
```

The client should use this to make the block read-only and show who owns it.

### Renew Lock

```json
{
  "type": "lock_renew",
  "block_id": "block-123"
}
```

### Release Lock

```json
{
  "type": "unlock",
  "block_id": "block-123"
}
```

### Lock Released

```json
{
  "type": "lock_released",
  "block_id": "block-123"
}
```

### Block Update

```json
{
  "type": "block_update",
  "block_id": "block-123",
  "content": "Updated block content",
  "version": 8
}
```

The exact message structure may be refined during implementation, but keep the protocol small.

---

## 12. WebSocket Room Model

The Go backend maintains an in-memory room per page.

Conceptually:

```text
WebSocket Hub
|
+-- page-1
|    +-- Alice
|    +-- Bob
|
+-- page-2
|    +-- Alice
|
+-- page-3
     +-- Charlie
```

A room manages:

- Connected clients.
- Broadcasting messages.
- Basic cleanup on disconnect.

Redis owns the actual distributed lock state.

For this minor project, assume one Go backend instance.

Do NOT design for multi-instance backend deployment.

---

## 13. Save Flow

When a user edits a block:

```text
React
  |
  | block update
  v
Go WebSocket/API
  |
  +-- verify authentication
  |
  +-- verify block lock ownership
  |
  +-- verify page version
  |
  +-- update PostgreSQL
  |
  +-- increment version
  |
  +-- create revision if appropriate
  |
  +-- broadcast update
  |
  v
Other connected clients
```

If the lock is no longer owned by the user, reject the update.

---

## 14. Optimistic Versioning

Locks prevent normal simultaneous edits, but versioning provides another safety layer.

Example:

```text
Current page version = 7
```

Client starts editing at version 7.

Save request contains:

```text
expected_version = 7
```

Server checks:

```text
current_version == expected_version
```

If true:

```text
7 -> 8
```

If false:

```text
409 Conflict
```

The client should refresh/synchronize the page rather than silently overwrite newer content.

Keep this mechanism simple.

---

## 15. Authentication

Use:

- Password hashing with bcrypt or an appropriate password hashing library.
- Secure HTTP-only authentication cookies/session mechanism.
- Authentication middleware in Go.

Required endpoints:

```text
POST /api/register
POST /api/login
POST /api/logout
GET  /api/me
```

The frontend must never be trusted for authorization.

The Go backend must verify permissions.

For this minor project, wiki permissions can remain simple:

```text
Owner
Viewer
```

If an editor role is desired, add it only if needed.

---

## 16. REST API

Minimal API:

```text
POST /api/register
POST /api/login
POST /api/logout
GET  /api/me

GET  /api/wikis
POST /api/wikis

GET  /api/wikis/:id/pages
POST /api/wikis/:id/pages

GET  /api/pages/:id
PUT  /api/pages/:id

GET  /api/pages/:id/revisions
```

Every protected endpoint requires authentication.

Every wiki/page endpoint must verify that the current user has access to the relevant wiki.

---

## 17. Frontend Pages

Minimum routes/screens:

```text
/login
/register
/dashboard
/wiki/:wikiID
/wiki/:wikiID/page/:pageID
```

### Dashboard

Show:

- User
- Wikis
- Create wiki

### Wiki page

Show:

- Wiki name
- Page list/sidebar
- Create page
- Current page

### Editor

Show:

- Page title
- Markdown blocks
- Lock state
- Save state
- Markdown preview
- Basic collaboration status

---

## 18. Editor UX

Simple UI is preferred.

Example:

```text
+--------------------------------------------------+
| Operating Systems                                |
+--------------------------------------------------+
|                                                  |
| # Operating Systems                              |
|                                                  |
| This paragraph is being edited by Alice. 🔒      |
|                                                  |
| Another paragraph available for editing.         |
|                                                  |
| ## Process Management                            |
|                                                  |
+--------------------------------------------------+
```

When a block is locked by another user:

```text
🔒 Alice is editing this block
```

The block should be read-only.

When the current user owns the lock:

```text
✎ You are editing this block
```

When unlocked:

```text
Click to edit
```

Do not implement complicated cursor visualization.

---

## 19. Real-Time Behavior

### User A opens page

```text
connect WebSocket
     |
     v
join page room
     |
     v
receive current collaboration state
```

### User A locks block

```text
A -> server -> Redis
               |
               v
            acquired
               |
               v
server broadcasts
               |
               v
B sees "Alice is editing"
```

### User A saves

```text
A -> server
       |
       v
PostgreSQL
       |
       v
broadcast
       |
       v
B updates displayed content
```

### User A disconnects unexpectedly

```text
disconnect
    |
    v
server attempts cleanup
    |
    v
TTL remains as fallback
    |
    v
30 sec
    |
    v
lock expires
```

---

## 20. Error Handling

The backend should return meaningful HTTP errors.

Examples:

```text
400 Bad Request
401 Unauthorized
403 Forbidden
404 Not Found
409 Conflict
500 Internal Server Error
```

WebSocket errors should use structured JSON messages.

Do not expose database errors or internal stack traces to users.

---

## 21. Testing

Keep testing proportional to the project.

### Backend

Test:

- Registration.
- Login.
- Authentication middleware.
- Wiki creation.
- Page creation/update.
- Revision creation.
- Lock acquisition.
- Lock denial.
- Lock renewal.
- Lock release.
- Lock expiration.
- Version conflict.

### Frontend

Test important UI behavior:

- Login.
- Page navigation.
- Markdown rendering.
- Locked block becomes read-only.
- Lock owner is displayed.
- Real-time update is displayed.

### Collaboration demo test

Two browser windows:

```text
Browser A                    Browser B

Open page                    Open page
     |                            |
Lock Block 1                     |
     |                            |
     +-------------------------->|
                                  |
                         Block 1 locked by A
                                  |
Edit Block 1                      |
     |                            |
     +-------------------------->|
                                  |
                         Content updates
                                  |
Unlock Block 1                    |
     |                            |
     +-------------------------->|
                                  |
                         B can edit Block 1
```

---

## 22. Docker

Use Docker Compose for local development.

Required services:

```text
frontend
backend
postgres
redis
```

PostgreSQL and Redis should use volumes where appropriate.

Configuration should be provided through `.env`.

Provide:

```text
.env.example
```

Never commit secrets.

---

## 23. Suggested Environment Variables

Example:

```text
DATABASE_URL=postgres://...
REDIS_URL=redis://...
SESSION_SECRET=change-me
LOCK_TTL=30s
LOCK_RENEW_INTERVAL=10s
PORT=8080
```

Use safe development defaults where appropriate, but never hard-code production secrets.

---

## 24. Team Responsibilities

### Member 1 — Go Backend/API

Owns:

- Go application setup.
- HTTP server/router.
- Authentication.
- Authorization.
- Wiki APIs.
- Page APIs.
- Revision APIs.
- Backend error handling.

### Member 2 — Database + Collaboration

Owns:

- PostgreSQL schema/migrations.
- PostgreSQL integration.
- Redis integration.
- TTL lock implementation.
- WebSocket server.
- Collaboration rooms.
- WebSocket message protocol.

### Member 3 — Markdown Editor

Owns:

- Markdown editor.
- Markdown parsing/preview.
- Block representation.
- Block IDs.
- Lock-aware editing.
- Save/update integration.
- Editor UX.

### Member 4 — Frontend/Product Integration

Owns:

- React application structure.
- Login/register UI.
- Dashboard.
- Wiki navigation.
- Page navigation.
- Layout/sidebar/navbar.
- API integration.
- Editor integration.
- Frontend testing and overall UI polish.

All members should participate in integration, testing, documentation, and final demo preparation.

---

## 25. Development Strategy

Do not wait for one person to finish before others start.

Define contracts first.

### API contract

Document:

```text
endpoint
request
response
error
```

### WebSocket contract

Document:

```text
message type
payload
server response
broadcast behavior
```

### Database contract

Document:

```text
tables
columns
relationships
constraints
```

Then members can develop independently.

Use mock data where necessary.

---

## 26. Git Workflow

Use a single repository.

Branches:

```text
main
feature/backend-api
feature/collaboration
feature/editor
feature/frontend
```

Workflow:

```text
feature branch
      |
      v
Pull Request
      |
      v
review
      |
      v
main
```

Avoid directly pushing unfinished work to `main`.

Commit messages should be short and descriptive.

---

## 27. Implementation Order

### Phase 1 — Project setup

- Docker Compose.
- React/Vite.
- Go server.
- PostgreSQL.
- Redis.
- Basic health endpoint.

### Phase 2 — Backend foundation

- Database migrations.
- User model.
- Authentication.
- Wiki APIs.
- Page APIs.

### Phase 3 — Frontend foundation

- Login/register.
- Dashboard.
- Wiki navigation.
- Page navigation.

### Phase 4 — Markdown

- Editor.
- Markdown preview.
- Block representation.
- Page saving.
- Revisions.

### Phase 5 — Collaboration

- WebSocket connection.
- Page rooms.
- Redis locks.
- Lock TTL.
- Lock renewal.
- Lock release.
- Lock status broadcasting.
- Saved content broadcasting.

### Phase 6 — Testing and polish

- Error handling.
- Collaboration testing with two browsers.
- UI polish.
- Documentation.
- Docker verification.
- Demo preparation.

---

## 28. Definition of Done

The project is complete when a user can:

```text
1. Register.
2. Login.
3. Create a wiki.
4. Create a page.
5. Write Markdown.
6. Preview Markdown.
7. Save the page.
8. View revision history.
9. Open the same page in another browser.
10. Lock a block for editing.
11. See the lock from the other browser in real time.
12. Prevent the second user from editing that locked block.
13. Edit another block simultaneously.
14. See saved changes from the other browser without refreshing.
15. Close/crash a browser and have its lock eventually expire.
```

The two-browser collaboration demonstration is the most important project demo.

---

## 29. Engineering Principles

Keep the implementation:

- Simple.
- Readable.
- Explicit.
- Easy to debug.
- Easy for college teammates to understand.

Prefer straightforward code over abstraction.

Avoid premature optimization.

Avoid unnecessary dependencies.

Avoid unnecessary database tables.

Avoid unnecessary services.

Use documentation for design decisions.

Use comments only when explaining why something non-obvious is necessary.

Handle errors explicitly.

Validate user input.

Use parameterized SQL queries.

Never trust client-side authorization.

Never blindly delete a Redis lock without verifying ownership.

---

## 30. Important Design Decisions

These decisions are intentional:

### Markdown is the source of truth

Do not store rendered HTML as the canonical page content.

### Redis stores active locks

PostgreSQL stores persistent data.

Redis stores temporary collaboration state.

### TTL locks

Locks automatically expire to handle crashed/disconnected clients.

### Block-level collaboration

Only one user edits a block at a time.

Different blocks can be edited simultaneously.

### No OT/CRDT

The project does not attempt character-level conflict-free collaboration.

### No multi-server support

Assume one Go backend instance.

### No real-time keystroke streaming

Save meaningful block updates and broadcast them.

---

## 31. Security Basics

At minimum:

- Hash passwords securely.
- Use HTTP-only authentication cookies/sessions.
- Validate request data.
- Use parameterized SQL.
- Verify wiki/page access on the backend.
- Verify lock ownership server-side.
- Sanitize Markdown-rendered HTML against XSS.
- Do not trust block IDs or page IDs supplied by the client.
- Do not expose secrets in Git.
- Configure CORS appropriately.

---

## 32. Final Product Concept

The final product should feel like:

```text
              WikiCollab

       Simple Markdown Wiki
                +
       Real-Time Collaboration
                +
        Block-Level Locking
```

The core technical story is:

```text
React
  |
  | REST + WebSocket
  v
Go
  |
  +---- PostgreSQL
  |       persistent wiki data
  |
  +---- Redis
          TTL block locks
```

Keep the project small, stable, and demonstrable. Do not add features just to make the architecture look more sophisticated.
