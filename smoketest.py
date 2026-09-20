"""
WikiCollab Live API Smoke Tests
Requires: requests (pip install requests)
"""
import sys
import json
import requests

BASE = "http://localhost:8080"
PASS = "TestPassword123!"

s_alice = requests.Session()
s_bob   = requests.Session()

def ok(label, resp, expected_status):
    symbol = "✅" if resp.status_code == expected_status else "❌"
    print(f"  {symbol} [{resp.status_code}] {label}")
    if resp.status_code != expected_status:
        print(f"       body: {resp.text[:200]}")
        return False
    return True

def check(label, condition, detail=""):
    symbol = "✅" if condition else "❌"
    print(f"  {symbol} {label}" + (f" — {detail}" if detail else ""))
    return condition

errors = 0

print("\n══════════════════════════════════════")
print("   WikiCollab API Smoke Tests")
print("══════════════════════════════════════\n")

# ── 1. Health ──────────────────────────────
print("1. Health")
r = requests.get(f"{BASE}/health")
if not ok("GET /health → 200 {status:ok}", r, 200): errors += 1
check("status=ok", r.json().get("status") == "ok")
print()

# ── 2. Register ────────────────────────────
print("2. Register")
import time; ts = str(int(time.time()))
alice_email = f"alice_{ts}@example.com"
bob_email   = f"bob_{ts}@example.com"

r = s_alice.post(f"{BASE}/api/register", json={"username": f"Alice_{ts}", "email": alice_email, "password": PASS})
if not ok("POST /api/register Alice → 201", r, 201): errors += 1
body = r.json()
check("no password_hash in response", "password_hash" not in body)
check("no $2a$ hash in response", "$2a$" not in r.text)
alice_id = body.get("id")

r = s_bob.post(f"{BASE}/api/register", json={"username": f"Bob_{ts}", "email": bob_email, "password": PASS})
if not ok("POST /api/register Bob → 201", r, 201): errors += 1

# Duplicate
r = requests.post(f"{BASE}/api/register", json={"username": f"Alice_{ts}", "email": alice_email, "password": PASS})
if not ok("POST /api/register duplicate → 409", r, 409): errors += 1
print()

# ── 3. Login ───────────────────────────────
print("3. Login")
r = s_alice.post(f"{BASE}/api/login", json={"email": alice_email, "password": PASS})
if not ok("POST /api/login Alice → 200", r, 200): errors += 1
check("no password_hash in login response", "password_hash" not in r.text)
check("session cookie set", "wikicollab-session" in s_alice.cookies)

r = requests.post(f"{BASE}/api/login", json={"email": alice_email, "password": "wrongpassword"})
if not ok("POST /api/login wrong password → 401", r, 401): errors += 1
print()

# ── 4. /api/me ─────────────────────────────
print("4. /api/me")
r = s_alice.get(f"{BASE}/api/me")
if not ok("GET /api/me (authenticated) → 200", r, 200): errors += 1
check("email matches", r.json().get("email") == alice_email)

r = requests.get(f"{BASE}/api/me")
if not ok("GET /api/me (unauthenticated) → 401", r, 401): errors += 1
print()

# ── 5. Empty wiki name validation ──────────
print("5. Validation")
r = s_alice.post(f"{BASE}/api/wikis", json={"name": ""})
if not ok("POST /api/wikis empty name → 400", r, 400): errors += 1
print()

# ── 6. Create Wiki ─────────────────────────
print("6. Wiki CRUD")
r = s_alice.post(f"{BASE}/api/wikis", json={"name": "Computer Science Wiki"})
if not ok("POST /api/wikis → 201", r, 201): errors += 1
wiki = r.json()
wiki_id = wiki["id"]
check("wiki has id", bool(wiki_id))

r = s_alice.get(f"{BASE}/api/wikis")
if not ok("GET /api/wikis → 200", r, 200): errors += 1
check("wiki in list", any(w["id"] == wiki_id for w in r.json()))

r = s_alice.get(f"{BASE}/api/wikis/{wiki_id}")
if not ok("GET /api/wikis/:id → 200", r, 200): errors += 1

r = s_alice.get(f"{BASE}/api/wikis/00000000-0000-0000-0000-000000000000")
if not ok("GET /api/wikis/bad-id → 404", r, 404): errors += 1
print()

# ── 7. Cross-user access ───────────────────
print("7. Authorization")
r = s_bob.post(f"{BASE}/api/login", json={"email": bob_email, "password": PASS})
if not ok("Login Bob → 200", r, 200): errors += 1

r = s_bob.get(f"{BASE}/api/wikis/{wiki_id}")
if not ok("Bob GET Alice's wiki → 403", r, 403): errors += 1
print()

# ── 8. Page CRUD ───────────────────────────
print("8. Page CRUD")
content = "# Introduction\n\nThis is a paragraph.\n\n## Algorithms\n\n- Sorting\n- Searching"
r = s_alice.post(f"{BASE}/api/wikis/{wiki_id}/pages",
                 json={"title": "Intro to Algorithms", "content": content})
if not ok("POST /api/wikis/:id/pages → 201", r, 201): errors += 1
page = r.json()
page_id = page["id"]
check("page version=1", page["version"] == 1)
check("page content stored", page["content"] == content)

r = s_alice.get(f"{BASE}/api/pages/{page_id}")
if not ok("GET /api/pages/:id → 200", r, 200): errors += 1

r = s_alice.get(f"{BASE}/api/wikis/{wiki_id}/pages")
if not ok("GET /api/wikis/:id/pages → 200", r, 200): errors += 1
check("page in list", any(p["id"] == page_id for p in r.json()))
print()

# ── 9. Optimistic versioning ───────────────
print("9. Optimistic Versioning")
r = s_alice.put(f"{BASE}/api/pages/{page_id}",
                json={"title": "Intro to Algorithms", "content": "# Intro\n\nUpdated.", "expected_version": 1})
if not ok("PUT /api/pages/:id v1 → 200", r, 200): errors += 1
updated = r.json()
check("version incremented to 2", updated["version"] == 2)

r = s_alice.put(f"{BASE}/api/pages/{page_id}",
                json={"title": "Intro", "content": "stale", "expected_version": 1})
if not ok("PUT /api/pages/:id stale v1 → 409 conflict", r, 409): errors += 1

r = s_alice.put(f"{BASE}/api/pages/{page_id}",
                json={"title": "Intro v3", "content": "# Intro\n\nSaved again.", "expected_version": 2})
if not ok("PUT /api/pages/:id v2 → 200", r, 200): errors += 1
check("version incremented to 3", r.json()["version"] == 3)
print()

# ── 10. Revisions ─────────────────────────
print("10. Revisions")
r = s_alice.get(f"{BASE}/api/pages/{page_id}/revisions")
if not ok("GET /api/pages/:id/revisions → 200", r, 200): errors += 1
revisions = r.json()
check(f"3 revisions exist (got {len(revisions)})", len(revisions) == 3)
check("revisions sorted ascending", revisions[0]["version"] < revisions[-1]["version"])
print()

# ── 11. Logout ────────────────────────────
print("11. Logout")
r = s_alice.post(f"{BASE}/api/logout")
if not ok("POST /api/logout → 200", r, 200): errors += 1

r = s_alice.get(f"{BASE}/api/me")
if not ok("GET /api/me after logout → 401", r, 401): errors += 1
print()

# ── Summary ───────────────────────────────
print("══════════════════════════════════════")
if errors == 0:
    print("  ✅ ALL TESTS PASSED")
else:
    print(f"  ❌ {errors} TEST(S) FAILED")
print("══════════════════════════════════════\n")
sys.exit(0 if errors == 0 else 1)
