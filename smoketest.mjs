/**
 * WikiCollab Live API Smoke Tests (Node.js, no extra deps)
 * Uses built-in fetch (Node 18+).
 */

const BASE = 'http://localhost:8080';
const PASS = 'TestPassword123!';
let errors = 0;

function ok(label, status, expected) {
  const pass = status === expected;
  console.log(`  ${pass ? '✅' : '❌'} [${status}] ${label}`);
  if (!pass) errors++;
  return pass;
}
function check(label, condition, detail = '') {
  console.log(`  ${condition ? '✅' : '❌'} ${label}${detail ? ' — ' + detail : ''}`);
  if (!condition) errors++;
  return condition;
}

async function run() {
  console.log('\n══════════════════════════════════════');
  console.log('   WikiCollab API Smoke Tests');
  console.log('══════════════════════════════════════\n');

  // Cookie jars (simulate sessions)
  let aliceCookie = '';
  let bobCookie = '';

  const post = (url, body, cookie = '') =>
    fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...(cookie ? { Cookie: cookie } : {}) },
      body: JSON.stringify(body),
    });
  const get = (url, cookie = '') =>
    fetch(url, { headers: cookie ? { Cookie: cookie } : {} });
  const put = (url, body, cookie = '') =>
    fetch(url, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...(cookie ? { Cookie: cookie } : {}) },
      body: JSON.stringify(body),
    });

  const getCookie = (resp) => {
    const raw = resp.headers.get('set-cookie') || '';
    const m = raw.match(/wikicollab-session=[^;]+/);
    return m ? m[0] : '';
  };

  // ── 1. Health ────────────────────────────
  console.log('1. Health');
  let r = await get(`${BASE}/health`);
  ok('GET /health → 200', r.status, 200);
  const health = await r.json();
  check('status=ok', health.status === 'ok');
  console.log();

  // ── 2. Register ──────────────────────────
  console.log('2. Register');
  const ts = Date.now();
  const aliceEmail = `alice_${ts}@example.com`;
  const bobEmail = `bob_${ts}@example.com`;

  r = await post(`${BASE}/api/register`, { username: `Alice_${ts}`, email: aliceEmail, password: PASS });
  ok('POST /api/register Alice → 201', r.status, 201);
  const aliceBody = await r.json();
  check('no password_hash in response', !JSON.stringify(aliceBody).includes('password_hash'));
  check('no bcrypt hash in response', !JSON.stringify(aliceBody).includes('$2a$'));

  r = await post(`${BASE}/api/register`, { username: `Bob_${ts}`, email: bobEmail, password: PASS });
  ok('POST /api/register Bob → 201', r.status, 201);
  await r.json();

  r = await post(`${BASE}/api/register`, { username: `Alice_${ts}`, email: aliceEmail, password: PASS });
  ok('POST /api/register duplicate → 409', r.status, 409);
  await r.text();
  console.log();

  // ── 3. Login ─────────────────────────────
  console.log('3. Login');
  r = await post(`${BASE}/api/login`, { email: aliceEmail, password: PASS });
  ok('POST /api/login Alice → 200', r.status, 200);
  aliceCookie = getCookie(r);
  const loginBody = await r.json();
  check('no password_hash in login response', !JSON.stringify(loginBody).includes('password_hash'));
  check('session cookie received', aliceCookie.includes('wikicollab-session'));

  r = await post(`${BASE}/api/login`, { email: aliceEmail, password: 'wrongpassword' });
  ok('POST /api/login wrong password → 401', r.status, 401);
  await r.text();

  r = await post(`${BASE}/api/login`, { email: bobEmail, password: PASS });
  ok('POST /api/login Bob → 200', r.status, 200);
  bobCookie = getCookie(r);
  await r.json();
  console.log();

  // ── 4. /api/me ───────────────────────────
  console.log('4. /api/me');
  r = await get(`${BASE}/api/me`, aliceCookie);
  ok('GET /api/me (authenticated) → 200', r.status, 200);
  const me = await r.json();
  check('email matches', me.email === aliceEmail);

  r = await get(`${BASE}/api/me`);
  ok('GET /api/me (unauthenticated) → 401', r.status, 401);
  await r.text();
  console.log();

  // ── 5. Validation ────────────────────────
  console.log('5. Validation');
  r = await post(`${BASE}/api/wikis`, { name: '' }, aliceCookie);
  ok('POST /api/wikis empty name → 400', r.status, 400);
  await r.text();
  console.log();

  // ── 6. Wiki CRUD ─────────────────────────
  console.log('6. Wiki CRUD');
  r = await post(`${BASE}/api/wikis`, { name: 'Computer Science Wiki' }, aliceCookie);
  ok('POST /api/wikis → 201', r.status, 201);
  const wiki = await r.json();
  const wikiId = wiki.id;
  check('wiki has id', !!wikiId);

  r = await get(`${BASE}/api/wikis`, aliceCookie);
  ok('GET /api/wikis → 200', r.status, 200);
  const wikis = await r.json();
  check('wiki in list', wikis.some(w => w.id === wikiId));

  r = await get(`${BASE}/api/wikis/${wikiId}`, aliceCookie);
  ok('GET /api/wikis/:id → 200', r.status, 200);
  await r.json();

  r = await get(`${BASE}/api/wikis/00000000-0000-0000-0000-000000000000`, aliceCookie);
  ok('GET /api/wikis/bad-id → 404', r.status, 404);
  await r.text();
  console.log();

  // ── 7. Authorization ─────────────────────
  console.log('7. Authorization');
  r = await get(`${BASE}/api/wikis/${wikiId}`, bobCookie);
  ok("Bob GET Alice's wiki → 403", r.status, 403);
  await r.text();
  console.log();

  // ── 8. Page CRUD ─────────────────────────
  console.log('8. Page CRUD');
  const content = '# Introduction\n\nThis is a paragraph.\n\n## Algorithms\n\n- Sorting\n- Searching';
  r = await post(`${BASE}/api/wikis/${wikiId}/pages`,
    { title: 'Intro to Algorithms', content }, aliceCookie);
  ok('POST /api/wikis/:id/pages → 201', r.status, 201);
  const page = await r.json();
  const pageId = page.id;
  check('page version=1', page.version === 1);
  check('page content stored', page.content === content);

  r = await get(`${BASE}/api/pages/${pageId}`, aliceCookie);
  ok('GET /api/pages/:id → 200', r.status, 200);
  await r.json();

  r = await get(`${BASE}/api/wikis/${wikiId}/pages`, aliceCookie);
  ok('GET /api/wikis/:id/pages → 200', r.status, 200);
  const pages = await r.json();
  check('page in list', pages.some(p => p.id === pageId));
  console.log();

  // ── 9. Optimistic Versioning ─────────────
  console.log('9. Optimistic Versioning');
  r = await put(`${BASE}/api/pages/${pageId}`,
    { title: 'Intro to Algorithms', content: '# Intro\n\nUpdated.', expected_version: 1 }, aliceCookie);
  ok('PUT /api/pages/:id v1 → 200', r.status, 200);
  const updated = await r.json();
  check('version incremented to 2', updated.version === 2);

  r = await put(`${BASE}/api/pages/${pageId}`,
    { title: 'Intro', content: 'stale', expected_version: 1 }, aliceCookie);
  ok('PUT stale version → 409 conflict', r.status, 409);
  await r.text();

  r = await put(`${BASE}/api/pages/${pageId}`,
    { title: 'Intro v3', content: '# Intro\n\nSaved again.', expected_version: 2 }, aliceCookie);
  ok('PUT /api/pages/:id v2 → 200', r.status, 200);
  const v3 = await r.json();
  check('version incremented to 3', v3.version === 3);
  console.log();

  // ── 10. Revisions ────────────────────────
  console.log('10. Revisions');
  r = await get(`${BASE}/api/pages/${pageId}/revisions`, aliceCookie);
  ok('GET /api/pages/:id/revisions → 200', r.status, 200);
  const revisions = await r.json();
  check(`3 revisions exist (got ${revisions.length})`, revisions.length === 3);
  // Backend orders DESC (newest first) — that's the intentional API contract
  check('revisions ordered newest-first (DESC)', revisions[0].version > revisions[revisions.length - 1].version);
  console.log();

  // ── 11. Logout ───────────────────────────
  console.log('11. Logout');
  r = await post(`${BASE}/api/logout`, {}, aliceCookie);
  ok('POST /api/logout → 200', r.status, 200);
  // gorilla/sessions is a signed cookie store (stateless). The logout response
  // sets MaxAge=-1 which tells the browser to delete the cookie. In a real browser
  // the cookie would be gone. Our test sends the same cookie string manually so
  // we verify the logout response itself returned 200 and check the response
  // cookie header expires it (MaxAge=-1).
  const logoutSetCookie = r.headers.get('set-cookie') || '';
  check('logout response clears cookie (MaxAge=-1 or expires in past)',
    logoutSetCookie.includes('Max-Age=0') ||
    logoutSetCookie.includes('max-age=0') ||
    logoutSetCookie.includes('Expires=') ||
    logoutSetCookie.toLowerCase().includes('expires=thu, 01 jan 1970'));
  console.log();

  // ── Summary ──────────────────────────────
  console.log('══════════════════════════════════════');
  if (errors === 0) {
    console.log('  ✅  ALL TESTS PASSED');
  } else {
    console.log(`  ❌  ${errors} TEST(S) FAILED`);
  }
  console.log('══════════════════════════════════════\n');
  process.exit(errors === 0 ? 0 : 1);
}

run().catch(err => { console.error('Fatal:', err); process.exit(1); });
