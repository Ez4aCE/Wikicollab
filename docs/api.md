# WikiCollab REST API

Base URL: `http://localhost:8080`

All request/response bodies use JSON. All authenticated endpoints require the `wikicollab-session` HTTP-only cookie set by `POST /api/login`.

---

## Health

### GET /health

No authentication required.

**Response 200**
```json
{ "status": "ok" }
```

---

## Authentication

### POST /api/register

Register a new user.

**Request**
```json
{
  "username": "alice",
  "email": "alice@example.com",
  "password": "password123"
}
```

**Validation**
- `username`: required, 2–50 characters
- `email`: required, valid email format, unique
- `password`: required, minimum 8 characters

**Response 201**
```json
{
  "id": "a1b2c3d4-...",
  "username": "alice",
  "email": "alice@example.com",
  "created_at": "2026-09-17T10:00:00Z"
}
```

**Errors**
| Status | Reason |
|--------|--------|
| 400 | Validation failed (missing fields, invalid email, short password) |
| 409 | Email or username already in use |

---

### POST /api/login

Authenticate and receive a session cookie.

**Request**
```json
{
  "email": "alice@example.com",
  "password": "password123"
}
```

**Response 200**

Sets `wikicollab-session` HTTP-only cookie (7-day session).

```json
{
  "id": "a1b2c3d4-...",
  "username": "alice",
  "email": "alice@example.com",
  "created_at": "2026-09-17T10:00:00Z"
}
```

**Errors**
| Status | Reason |
|--------|--------|
| 400 | Missing fields |
| 401 | Invalid email or password |

---

### POST /api/logout

Invalidate the current session.

**Auth**: Required (session cookie)

**Response 200**
```json
{ "message": "logged out" }
```

---

### GET /api/me

Return the currently authenticated user.

**Auth**: Required

**Response 200**
```json
{
  "id": "a1b2c3d4-...",
  "username": "alice",
  "email": "alice@example.com",
  "created_at": "2026-09-17T10:00:00Z"
}
```

**Errors**
| Status | Reason |
|--------|--------|
| 401 | Not authenticated |

---

## Wikis

### GET /api/wikis

List wikis owned by the authenticated user.

**Auth**: Required

**Response 200**
```json
[
  {
    "id": "w1-...",
    "name": "Computer Science",
    "owner_id": "a1b2c3d4-...",
    "created_at": "2026-09-17T10:00:00Z"
  }
]
```

Returns an empty array if the user has no wikis.

---

### POST /api/wikis

Create a new wiki owned by the current user.

**Auth**: Required

**Request**
```json
{ "name": "Computer Science" }
```

**Validation**
- `name`: required, max 255 characters

**Response 201**
```json
{
  "id": "w1-...",
  "name": "Computer Science",
  "owner_id": "a1b2c3d4-...",
  "created_at": "2026-09-17T10:00:00Z"
}
```

**Errors**
| Status | Reason |
|--------|--------|
| 400 | Missing or invalid name |
| 401 | Not authenticated |

---

### GET /api/wikis/:id

Get a specific wiki. Only the owner may access it.

**Auth**: Required

**Response 200**
```json
{
  "id": "w1-...",
  "name": "Computer Science",
  "owner_id": "a1b2c3d4-...",
  "created_at": "2026-09-17T10:00:00Z"
}
```

**Errors**
| Status | Reason |
|--------|--------|
| 401 | Not authenticated |
| 403 | Not the wiki owner |
| 404 | Wiki not found |

---

## Pages

### GET /api/wikis/:id/pages

List all pages in a wiki.

**Auth**: Required (must own the wiki)

**Response 200**
```json
[
  {
    "id": "p1-...",
    "wiki_id": "w1-...",
    "title": "Operating Systems",
    "content": "# Operating Systems\n\n...",
    "version": 3,
    "created_at": "2026-09-17T10:00:00Z",
    "updated_at": "2026-09-17T11:00:00Z"
  }
]
```

---

### POST /api/wikis/:id/pages

Create a new page in a wiki.

**Auth**: Required (must own the wiki)

**Request**
```json
{
  "title": "Operating Systems",
  "content": "# Operating Systems\n\nAn OS manages resources."
}
```

**Validation**
- `title`: required, max 255 characters
- `content`: required

**Response 201**
```json
{
  "id": "p1-...",
  "wiki_id": "w1-...",
  "title": "Operating Systems",
  "content": "# Operating Systems\n\nAn OS manages resources.",
  "version": 1,
  "created_at": "2026-09-17T10:00:00Z",
  "updated_at": "2026-09-17T10:00:00Z"
}
```

The initial revision (version 1) is created automatically.

**Errors**
| Status | Reason |
|--------|--------|
| 400 | Missing or invalid title/content |
| 401 | Not authenticated |
| 403 | Not the wiki owner |
| 404 | Wiki not found |

---

### GET /api/pages/:id

Get a single page by ID.

**Auth**: Required (must own the wiki this page belongs to)

**Response 200**
```json
{
  "id": "p1-...",
  "wiki_id": "w1-...",
  "title": "Operating Systems",
  "content": "# Operating Systems\n\n...",
  "version": 5,
  "created_at": "2026-09-17T10:00:00Z",
  "updated_at": "2026-09-17T12:00:00Z"
}
```

**Errors**
| Status | Reason |
|--------|--------|
| 401 | Not authenticated |
| 403 | Not the wiki owner |
| 404 | Page not found |

---

### PUT /api/pages/:id

Update a page. Uses optimistic concurrency — the client must send the version it last saw.

**Auth**: Required (must own the wiki this page belongs to)

**Request**
```json
{
  "title": "Operating Systems",
  "content": "# Operating Systems\n\nUpdated content.",
  "expected_version": 4
}
```

The server checks `expected_version == current version`. If they match:
- Content and title are updated
- `version` is incremented by 1
- A new revision is created

**Response 200**
```json
{
  "id": "p1-...",
  "wiki_id": "w1-...",
  "title": "Operating Systems",
  "content": "# Operating Systems\n\nUpdated content.",
  "version": 5,
  "created_at": "2026-09-17T10:00:00Z",
  "updated_at": "2026-09-17T13:00:00Z"
}
```

**Errors**
| Status | Reason |
|--------|--------|
| 400 | Missing fields or invalid expected_version |
| 401 | Not authenticated |
| 403 | Not the wiki owner |
| 404 | Page not found |
| 409 | Version conflict — page was updated by someone else since you loaded it. Refresh and retry. |

---

## Revisions

### GET /api/pages/:id/revisions

List the revision history for a page, newest first.

**Auth**: Required (must own the wiki this page belongs to)

**Response 200**
```json
[
  {
    "id": "r2-...",
    "page_id": "p1-...",
    "content": "# Updated content",
    "version": 5,
    "created_by": "a1b2c3d4-...",
    "created_at": "2026-09-17T13:00:00Z"
  },
  {
    "id": "r1-...",
    "page_id": "p1-...",
    "content": "# Original content",
    "version": 1,
    "created_by": "a1b2c3d4-...",
    "created_at": "2026-09-17T10:00:00Z"
  }
]
```

**Errors**
| Status | Reason |
|--------|--------|
| 401 | Not authenticated |
| 403 | Not the wiki owner |
| 404 | Page not found |

---

## Error Format

All errors use the same JSON structure:

```json
{ "error": "descriptive message" }
```

Common status codes:

| Code | Meaning |
|------|---------|
| 400 | Bad Request — invalid input |
| 401 | Unauthorized — not authenticated |
| 403 | Forbidden — authenticated but not authorized |
| 404 | Not Found |
| 409 | Conflict — version mismatch or duplicate value |
| 500 | Internal Server Error |

Internal error details (SQL errors, stack traces) are never exposed to clients.
