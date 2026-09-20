# WikiCollab — WebSocket Protocol

## Endpoint

```
GET /ws/pages/:pageID
```

`:pageID` is the UUID of the page being collaborated on.

**Authentication**: The request must include the `wikicollab-session` HTTP-only cookie set by `POST /api/login`. Unauthenticated connections receive `401 Unauthorized` and are closed.

**Authorization**: The user must own the wiki that the page belongs to. Non-owners receive `403 Forbidden`.

---

## Connection Lifecycle

```
Client                          Server
  |                               |
  |  GET /ws/pages/:id            |
  |------------------------------>|
  |  (with session cookie)        |
  |                               |-- authenticate session
  |                               |-- verify page access
  |                               |-- upgrade HTTP → WebSocket
  |                               |-- join page room
  |<------ connection open -------|
  |                               |
  |  (send/receive messages)      |
  |                               |
  |  close / disconnect           |
  |------------------------------>|
  |                               |-- release all held locks
  |                               |-- broadcast lock_released for each
  |                               |-- leave room
```

If the client crashes without closing the connection, held locks expire automatically via Redis TTL (default: 30 seconds).

---

## Message Format

All messages are JSON objects. The `type` field determines the message kind.

### Client → Server Messages

#### `lock` — Acquire a block lock

```json
{
  "type": "lock",
  "block_id": "block-123"
}
```

Attempts to acquire an exclusive editing lock for the specified block. The server responds with either `lock_acquired` (broadcast) or `lock_denied` (to requester only).

---

#### `lock_renew` — Renew an existing lock

```json
{
  "type": "lock_renew",
  "block_id": "block-123"
}
```

Resets the TTL on a lock the client currently holds. Must be sent before the TTL expires (default renewal interval: 10 seconds). No broadcast is sent for renewals.

---

#### `unlock` — Release a block lock

```json
{
  "type": "unlock",
  "block_id": "block-123"
}
```

Releases a lock the client holds. The server broadcasts `lock_released` to the room.

---

#### `page_update` — Save page content

```json
{
  "type": "page_update",
  "title": "Operating Systems",
  "content": "# Operating Systems\n\nUpdated content.",
  "expected_version": 4
}
```

Saves the page to PostgreSQL using the same optimistic-concurrency check as `PUT /api/pages/:id`. The server broadcasts `page_updated` to all clients in the room (including the sender, so the sender receives the confirmed new version number).

If `expected_version` does not match the current page version, the server sends an `error` message with `"version conflict"` to the requester only.

---

### Server → Client Messages

#### `lock_acquired` — Broadcast: block is now locked

Sent to **all clients** in the room.

```json
{
  "type": "lock_acquired",
  "block_id": "block-123",
  "user_id": "a1b2c3d4-...",
  "username": "alice"
}
```

Clients should mark the block as read-only and show who is editing it.

---

#### `lock_denied` — Lock request refused

Sent to the **requester only**.

```json
{
  "type": "lock_denied",
  "block_id": "block-123",
  "user_id": "a1b2c3d4-...",
  "username": "alice"
}
```

The block is already locked by another user. The requester should keep the block read-only.

---

#### `lock_released` — Broadcast: block is now free

Sent to **all clients** in the room.

```json
{
  "type": "lock_released",
  "block_id": "block-123"
}
```

The block is available for editing. Also sent when a client disconnects and their locks are cleaned up.

---

#### `page_updated` — Broadcast: page content changed

Sent to **all clients** in the room (including the saving client).

```json
{
  "type": "page_updated",
  "page_id": "p1-...",
  "title": "Operating Systems",
  "content": "# Operating Systems\n\nUpdated content.",
  "version": 5
}
```

Clients should update their local document and note the new version number (needed for the next `page_update` request).

---

#### `error` — Error message

Sent to the **affected client only**.

```json
{
  "type": "error",
  "message": "version conflict — refresh and retry"
}
```

Common error messages:

| Message | Cause |
|---------|-------|
| `"block_id is required"` | Message missing `block_id` |
| `"you do not hold the lock for block X"` | Renew/unlock for a block not owned by this client |
| `"lock expired for block X"` | TTL expired before renewal arrived |
| `"lock service unavailable"` | Redis error |
| `"version conflict — refresh and retry"` | `expected_version` does not match current page version |
| `"title and content are required"` | `page_update` missing fields |
| `"unknown message type: X"` | Unrecognized `type` field |

---

## Redis Lock Design

Lock keys are stored in Redis with the format:

```
lock:page:{pageID}:block:{blockID}
```

Each lock value is a unique random token (16-byte hex string) generated per acquisition. This token is stored server-side per client connection — the client never sees it.

### Acquire

Uses `SET key token NX EX {ttl}` — atomic, no GET-then-SET race.

### Renew

Uses a Lua script: `PEXPIRE` only if the stored token matches the caller's token.

### Release

Uses a Lua script: `DEL` only if the stored token matches the caller's token — prevents a delayed release from evicting a lock acquired by a different user.

### TTL Safety

If a client disconnects (or crashes) without releasing its locks:
1. The server's disconnect handler attempts immediate cleanup.
2. Any locks the handler cannot release (e.g., Redis error) automatically expire via Redis TTL.
3. Default TTL: 30 seconds. Configurable via `LOCK_TTL` environment variable.

---

## Integration Guide for Members 3 & 4

### Member 3 — Markdown Editor

1. Parse Markdown into blocks in memory (not stored in DB — as designed).
2. When the user clicks a block to edit: send `lock`.
3. On receiving `lock_acquired` (with your own `user_id`): enable editing.
4. On receiving `lock_denied`: show `"🔒 <username> is editing this block"` and keep it read-only.
5. On receiving `lock_acquired` from another user: mark that block read-only.
6. Send `lock_renew` every ~10 seconds while editing.
7. When the user finishes: send `page_update`, then `unlock`.
8. On receiving `page_updated` from another user: update the displayed content.
9. On receiving `lock_released`: unmark the block, show "Click to edit".

### Member 4 — React Frontend

Connect example (JavaScript):

```javascript
// After successful login (session cookie is set automatically)
const ws = new WebSocket(`ws://localhost:8080/ws/pages/${pageID}`);

ws.onopen = () => {
  console.log("Connected to page room");
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  switch (msg.type) {
    case "lock_acquired":
      // mark msg.block_id as locked by msg.username
      break;
    case "lock_denied":
      // show read-only indicator for msg.block_id
      break;
    case "lock_released":
      // unmark msg.block_id
      break;
    case "page_updated":
      // update document content and store msg.version
      break;
    case "error":
      console.error("WS error:", msg.message);
      break;
  }
};

// Acquire lock
ws.send(JSON.stringify({ type: "lock", block_id: "block-1" }));

// Renew lock (call every 10s while editing)
ws.send(JSON.stringify({ type: "lock_renew", block_id: "block-1" }));

// Save page
ws.send(JSON.stringify({
  type: "page_update",
  title: "My Page",
  content: "# Updated content",
  expected_version: currentVersion
}));

// Release lock
ws.send(JSON.stringify({ type: "unlock", block_id: "block-1" }));
```

> **Important**: The browser WebSocket API sends cookies automatically when connecting to the same origin. No manual token management is required — the session cookie handles authentication.

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_URL` | `redis://localhost:6379` | Redis connection URL |
| `LOCK_TTL` | `30s` | How long a lock lasts without renewal |
| `LOCK_RENEW_INTERVAL` | `10s` | Suggested client renewal interval |
