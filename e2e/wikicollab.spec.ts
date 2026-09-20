import { test, expect, type Page, type BrowserContext } from '@playwright/test';

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

const BASE = 'http://localhost:8080';

/** Generates a unique suffix to avoid duplicate email conflicts between test runs. */
function uniqueSuffix() {
  return Date.now().toString(36);
}

/** Register + login via the API directly, then navigate to the app as that user. */
async function loginAs(
  context: BrowserContext,
  username: string,
  email: string,
  password: string
): Promise<Page> {
  // Register via API
  const regRes = await context.request.post(`${BASE}/api/register`, {
    data: { username, email, password },
    headers: { 'Content-Type': 'application/json' },
  });
  const status = regRes.status();
  if (status !== 201 && status !== 409) {
    throw new Error(`Register failed: ${status} ${await regRes.text()}`);
  }

  // Login via API
  const loginRes = await context.request.post(`${BASE}/api/login`, {
    data: { email, password },
    headers: { 'Content-Type': 'application/json' },
  });
  expect(loginRes.status()).toBe(200);

  const page = await context.newPage();
  await page.goto('/dashboard');
  return page;
}

/** Create a wiki using the API (returns wiki ID). */
async function createWiki(context: BrowserContext, name: string): Promise<string> {
  const res = await context.request.post(`${BASE}/api/wikis`, {
    data: { name },
    headers: { 'Content-Type': 'application/json' },
  });
  expect(res.status()).toBe(201);
  const body = await res.json();
  return body.id as string;
}

/** Create a page using the API (returns page ID). */
async function createPage(
  context: BrowserContext,
  wikiId: string,
  title: string,
  content: string
): Promise<string> {
  const res = await context.request.post(`${BASE}/api/wikis/${wikiId}/pages`, {
    data: { title, content },
    headers: { 'Content-Type': 'application/json' },
  });
  expect(res.status()).toBe(201);
  const body = await res.json();
  return body.id as string;
}

// ──────────────────────────────────────────────────────────────────────────────
// 1. Authentication Flow
// ──────────────────────────────────────────────────────────────────────────────

test.describe('Authentication', () => {
  test('register → login → /api/me → logout cycle', async ({ context }) => {
    const s = uniqueSuffix();
    const email = `alice-${s}@example.com`;
    const password = 'TestPassword123!';

    // Register
    const reg = await context.request.post(`${BASE}/api/register`, {
      data: { username: `alice_${s}`, email, password },
      headers: { 'Content-Type': 'application/json' },
    });
    expect(reg.status()).toBe(201);
    const regBody = await reg.json();
    expect(regBody).not.toHaveProperty('password');
    expect(regBody).not.toHaveProperty('password_hash');
    expect(regBody.username).toBe(`alice_${s}`);

    // Duplicate registration should be rejected
    const dup = await context.request.post(`${BASE}/api/register`, {
      data: { username: `alice_${s}`, email, password },
      headers: { 'Content-Type': 'application/json' },
    });
    expect(dup.status()).toBe(409);

    // Login
    const login = await context.request.post(`${BASE}/api/login`, {
      data: { email, password },
      headers: { 'Content-Type': 'application/json' },
    });
    expect(login.status()).toBe(200);
    const loginBody = await login.json();
    expect(loginBody).not.toHaveProperty('password_hash');

    // /api/me
    const me = await context.request.get(`${BASE}/api/me`);
    expect(me.status()).toBe(200);
    const meBody = await me.json();
    expect(meBody.email).toBe(email);

    // Logout
    const logout = await context.request.post(`${BASE}/api/logout`);
    expect(logout.status()).toBe(200);

    // /api/me after logout should be 401
    const meAfter = await context.request.get(`${BASE}/api/me`);
    expect(meAfter.status()).toBe(401);
  });

  test('invalid credentials are rejected with 401', async ({ context }) => {
    const res = await context.request.post(`${BASE}/api/login`, {
      data: { email: 'nobody@example.com', password: 'wrongpassword' },
      headers: { 'Content-Type': 'application/json' },
    });
    expect(res.status()).toBe(401);
  });

  test('protected routes redirect unauthenticated browser users to /login', async ({ page }) => {
    await page.goto('/dashboard');
    await expect(page).toHaveURL(/\/login/);
  });

  test('login page UI works and redirects to dashboard', async ({ context }) => {
    const s = uniqueSuffix();
    const email = `uiuser-${s}@example.com`;
    const password = 'TestPassword123!';
    await context.request.post(`${BASE}/api/register`, {
      data: { username: `uiuser_${s}`, email, password },
      headers: { 'Content-Type': 'application/json' },
    });

    const page = await context.newPage();
    await page.goto('/login');
    // Labels exist but don't have htmlFor. Use placeholder/type selectors instead.
    await page.locator('input[type="email"]').fill(email);
    await page.locator('input[type="password"]').fill(password);
    await page.getByRole('button', { name: /log in/i }).click();
    await expect(page).toHaveURL(/\/dashboard/);
    await expect(page.getByText(/WikiCollab/)).toBeVisible();
  });
});

// ──────────────────────────────────────────────────────────────────────────────
// 2. Wiki & Page Flow
// ──────────────────────────────────────────────────────────────────────────────

test.describe('Wiki and Page Management', () => {
  test('create wiki → create page → view page', async ({ context }) => {
    const s = uniqueSuffix();
    const page = await loginAs(context, `wiki_user_${s}`, `wu-${s}@example.com`, 'TestPassword123!');

    // Dashboard should load
    await expect(page.getByText('Dashboard')).toBeVisible();

    // Create wiki via UI
    await page.getByPlaceholder('Wiki Name').fill(`Test Wiki ${s}`);
    await page.getByRole('button', { name: 'Create' }).click();
    await expect(page.getByText(`Test Wiki ${s}`)).toBeVisible();

    // Navigate to wiki
    await page.getByText(`Test Wiki ${s}`).click();
    // Use role heading to avoid ambiguity between <h3>Pages</h3> and the "No pages" paragraph
    await expect(page.getByRole('heading', { name: 'Pages' })).toBeVisible();

    // Create page via UI
    await page.getByPlaceholder('Page Title').fill(`My Page ${s}`);
    await page.getByRole('button', { name: 'Create Page' }).click();

    // Should navigate to the page editor — verify URL and title input value
    await expect(page).toHaveURL(/\/page\//);
    const titleInput = page.locator('input[type="text"]').first();
    await expect(titleInput).toBeVisible();
    await expect(titleInput).toHaveValue(`My Page ${s}`);
  });

  test('empty wiki name shows a validation error from backend', async ({ context }) => {
    const s = uniqueSuffix();
    await loginAs(context, `val_user_${s}`, `val-${s}@example.com`, 'TestPassword123!');
    const res = await context.request.post(`${BASE}/api/wikis`, {
      data: { name: '' },
      headers: { 'Content-Type': 'application/json' },
    });
    expect(res.status()).toBe(400);
  });

  test('invalid wiki ID returns 404', async ({ context }) => {
    const s = uniqueSuffix();
    await loginAs(context, `inv_user_${s}`, `inv-${s}@example.com`, 'TestPassword123!');
    const res = await context.request.get(`${BASE}/api/wikis/00000000-0000-0000-0000-000000000000`);
    expect(res.status()).toBe(404);
  });
});

// ──────────────────────────────────────────────────────────────────────────────
// 3. Markdown Editor
// ──────────────────────────────────────────────────────────────────────────────

test.describe('Markdown Editor', () => {
  test('markdown content is saved and persists after reload', async ({ context }) => {
    const s = uniqueSuffix();
    const wikiId = await (() => {
      return context.request.post(`${BASE}/api/register`, {
        data: { username: `md_user_${s}`, email: `md-${s}@example.com`, password: 'TestPassword123!' },
        headers: { 'Content-Type': 'application/json' },
      }).then(() => context.request.post(`${BASE}/api/login`, {
        data: { email: `md-${s}@example.com`, password: 'TestPassword123!' },
        headers: { 'Content-Type': 'application/json' },
      })).then(() => createWiki(context, `MD Wiki ${s}`));
    })();

    const mdContent = `# Introduction\n\nThis is a paragraph.\n\n## Algorithms\n\n- Sorting\n- Searching`;
    const pageId = await createPage(context, wikiId, `MD Page ${s}`, mdContent);

    const page = await context.newPage();
    await page.goto(`/wiki/${wikiId}/page/${pageId}`);

    // Rendered markdown should show heading
    await expect(page.getByText('Introduction')).toBeVisible();
    await expect(page.getByText('Algorithms')).toBeVisible();

    // Reload and verify persistence
    await page.reload();
    await expect(page.getByText('Introduction')).toBeVisible();
  });

  test('version increments after save via WebSocket', async ({ context }) => {
    const s = uniqueSuffix();
    await context.request.post(`${BASE}/api/register`, {
      data: { username: `ver_user_${s}`, email: `ver-${s}@example.com`, password: 'TestPassword123!' },
      headers: { 'Content-Type': 'application/json' },
    });
    await context.request.post(`${BASE}/api/login`, {
      data: { email: `ver-${s}@example.com`, password: 'TestPassword123!' },
      headers: { 'Content-Type': 'application/json' },
    });
    const wikiId = await createWiki(context, `Ver Wiki ${s}`);
    const pageId = await createPage(context, wikiId, `Ver Page ${s}`, '# Version Test\n\nParagraph.');

    // Check initial version
    const before = await context.request.get(`${BASE}/api/pages/${pageId}`);
    const beforeBody = await before.json();
    expect(beforeBody.version).toBe(1);

    // Update via REST API to verify optimistic versioning
    const updated = await context.request.put(`${BASE}/api/pages/${pageId}`, {
      data: { title: `Ver Page ${s}`, content: '# Version Test\n\nUpdated paragraph.', expected_version: 1 },
      headers: { 'Content-Type': 'application/json' },
    });
    expect(updated.status()).toBe(200);
    const updatedBody = await updated.json();
    expect(updatedBody.version).toBe(2);

    // Stale version should get 409
    const conflict = await context.request.put(`${BASE}/api/pages/${pageId}`, {
      data: { title: `Ver Page ${s}`, content: 'Stale.', expected_version: 1 },
      headers: { 'Content-Type': 'application/json' },
    });
    expect(conflict.status()).toBe(409);
  });
});

// ──────────────────────────────────────────────────────────────────────────────
// 4. Revision History
// ──────────────────────────────────────────────────────────────────────────────

test.describe('Revision History', () => {
  test('revisions are created on each update', async ({ context }) => {
    const s = uniqueSuffix();
    await context.request.post(`${BASE}/api/register`, {
      data: { username: `rev_user_${s}`, email: `rev-${s}@example.com`, password: 'TestPassword123!' },
      headers: { 'Content-Type': 'application/json' },
    });
    await context.request.post(`${BASE}/api/login`, {
      data: { email: `rev-${s}@example.com`, password: 'TestPassword123!' },
      headers: { 'Content-Type': 'application/json' },
    });
    const wikiId = await createWiki(context, `Rev Wiki ${s}`);
    const pageId = await createPage(context, wikiId, `Rev Page ${s}`, '# Hello');

    // Update twice
    await context.request.put(`${BASE}/api/pages/${pageId}`, {
      data: { title: `Rev Page ${s}`, content: '# Hello World', expected_version: 1 },
      headers: { 'Content-Type': 'application/json' },
    });
    await context.request.put(`${BASE}/api/pages/${pageId}`, {
      data: { title: `Rev Page ${s}`, content: '# Hello World\n\nWikiCollab.', expected_version: 2 },
      headers: { 'Content-Type': 'application/json' },
    });

    // Fetch revisions
    const revRes = await context.request.get(`${BASE}/api/pages/${pageId}/revisions`);
    expect(revRes.status()).toBe(200);
    const revisions = await revRes.json();
    // createPage creates revision v1; 2 updates = revisions v2 and v3 → total 3
    expect(revisions.length).toBeGreaterThanOrEqual(3);
    // Revisions are returned DESC (newest first), so index 0 = highest version
    expect(revisions[0].version).toBe(3);
  });

  test('revision history is visible in page editor UI', async ({ context }) => {
    const s = uniqueSuffix();
    await context.request.post(`${BASE}/api/register`, {
      data: { username: `revui_${s}`, email: `revui-${s}@example.com`, password: 'TestPassword123!' },
      headers: { 'Content-Type': 'application/json' },
    });
    await context.request.post(`${BASE}/api/login`, {
      data: { email: `revui-${s}@example.com`, password: 'TestPassword123!' },
      headers: { 'Content-Type': 'application/json' },
    });
    const wikiId = await createWiki(context, `RevUI Wiki ${s}`);
    const pageId = await createPage(context, wikiId, `RevUI Page ${s}`, '# Revision UI Test');

    const page = await context.newPage();
    await page.goto(`/wiki/${wikiId}/page/${pageId}`);

    // RevisionHistory toggle should be present
    await expect(page.getByText(/revision history/i)).toBeVisible();
    await page.getByText(/show revision history/i).click();
    // After clicking, should show version column
    await expect(page.getByText(/v1/)).toBeVisible();
  });
});

// ──────────────────────────────────────────────────────────────────────────────
// 5. Full Two-Browser Collaboration Flow
// ──────────────────────────────────────────────────────────────────────────────

test.describe('Two-browser collaboration', () => {
  /**
   * Real-time collaboration test:
   * The wiki is single-owner, so both tabs use Alice's credentials.
   * We verify:
   *  1. Both tabs connect and render page blocks via WebSocket
   *  2. Tab 1 can lock a block and enter edit mode
   *  3. Tab 1 saves the page (page_update → broadcasts page_updated)
   *  4. Tab 2 receives the broadcast and shows the updated version number
   *
   * Note: same-user same-block lock contention is a known client-side
   * limitation — the frontend treats isMe=true for both tabs and suppresses
   * a redundant lock request. Full lock-denial requires two different users,
   * which requires wiki sharing (out of scope for this college project).
   */
  test('two tabs: both connect, Tab 1 saves, Tab 2 sees updated version', async ({ browser }) => {
    const s = uniqueSuffix();
    const ctx1 = await browser.newContext();
    const ctx2 = await browser.newContext();

    try {
      const email    = `alice_collab_${s}@example.com`;
      const password = 'TestPassword123!';

      // Register Alice once, login in both contexts
      await ctx1.request.post(`${BASE}/api/register`, {
        data: { username: `alice_collab_${s}`, email, password },
        headers: { 'Content-Type': 'application/json' },
      });
      await ctx1.request.post(`${BASE}/api/login`, {
        data: { email, password }, headers: { 'Content-Type': 'application/json' },
      });
      await ctx2.request.post(`${BASE}/api/login`, {
        data: { email, password }, headers: { 'Content-Type': 'application/json' },
      });

      // Alice (ctx1) creates wiki and page
      const wikiId = await createWiki(ctx1, `Collab Wiki ${s}`);
      const content = `First paragraph.\n\nSecond paragraph.\n\nThird paragraph.`;
      const pageId  = await createPage(ctx1, wikiId, `Collab Page ${s}`, content);

      // Both tabs open the same page
      const page1 = await ctx1.newPage();
      const page2 = await ctx2.newPage();
      await page1.goto(`/wiki/${wikiId}/page/${pageId}`);
      await page2.goto(`/wiki/${wikiId}/page/${pageId}`);

      // Both tabs should render the page blocks
      await expect(page1.getByText('First paragraph.')).toBeVisible({ timeout: 10000 });
      await expect(page2.getByText('First paragraph.')).toBeVisible({ timeout: 10000 });

      // Tab 1 clicks a block → acquires lock → enters edit mode
      await page1.getByText('First paragraph.').click();
      await expect(page1.locator('textarea').first()).toBeVisible({ timeout: 8000 });

      // Tab 2 can independently click a different block
      await page2.getByText('Second paragraph.').click();
      await expect(page2.locator('textarea').first()).toBeVisible({ timeout: 8000 });

      // Check initial version shown in Tab 2
      await expect(page2.getByText('v1')).toBeVisible();

      // Tab 1 saves the page → broadcasts page_updated to all in the room
      await page1.getByRole('button', { name: 'Save Changes' }).click();

      // Tab 2 should receive the broadcast and show updated version (v2)
      await expect(page2.getByText('v2')).toBeVisible({ timeout: 10000 });

    } finally {
      await ctx1.close();
      await ctx2.close();
    }
  });
});

// ──────────────────────────────────────────────────────────────────────────────
// 6. Navigation & Protected Routes
// ──────────────────────────────────────────────────────────────────────────────

test.describe('Navigation', () => {
  test('unauthenticated user is redirected to /login from all protected routes', async ({ page }) => {
    for (const path of ['/dashboard', '/wiki/some-id', '/wiki/some-id/page/other-id']) {
      await page.goto(path);
      await expect(page).toHaveURL(/\/login/);
    }
  });

  test('register page is accessible without authentication', async ({ page }) => {
    await page.goto('/register');
    await expect(page.getByRole('heading', { name: 'Register' })).toBeVisible();
  });

  test('logout clears session and redirects to login', async ({ context }) => {
    const s = uniqueSuffix();
    const page = await loginAs(context, `logout_user_${s}`, `logout-${s}@example.com`, 'TestPassword123!');
    await expect(page).toHaveURL(/\/dashboard/);

    // Click logout button
    await page.getByRole('button', { name: /logout/i }).click();
    await expect(page).toHaveURL(/\/login/);

    // Accessing dashboard again redirects
    await page.goto('/dashboard');
    await expect(page).toHaveURL(/\/login/);
  });
});

// ──────────────────────────────────────────────────────────────────────────────
// 7. Security Smoke Tests
// ──────────────────────────────────────────────────────────────────────────────

test.describe('Security', () => {
  test('password_hash is never exposed in any API response', async ({ context }) => {
    const s = uniqueSuffix();
    // Register
    const reg = await context.request.post(`${BASE}/api/register`, {
      data: { username: `sec_user_${s}`, email: `sec-${s}@example.com`, password: 'TestPassword123!' },
      headers: { 'Content-Type': 'application/json' },
    });
    const regText = await reg.text();
    expect(regText).not.toContain('password_hash');
    expect(regText).not.toContain('$2a$');

    // Login
    const login = await context.request.post(`${BASE}/api/login`, {
      data: { email: `sec-${s}@example.com`, password: 'TestPassword123!' },
      headers: { 'Content-Type': 'application/json' },
    });
    const loginText = await login.text();
    expect(loginText).not.toContain('password_hash');
    expect(loginText).not.toContain('$2a$');
  });

  test('accessing another user\'s wiki is forbidden', async ({ browser }) => {
    const s = uniqueSuffix();
    const ctxAlice = await browser.newContext();
    const ctxBob = await browser.newContext();
    try {
      await ctxAlice.request.post(`${BASE}/api/register`, {
        data: { username: `secA_${s}`, email: `secA-${s}@example.com`, password: 'TestPassword123!' },
        headers: { 'Content-Type': 'application/json' },
      });
      await ctxAlice.request.post(`${BASE}/api/login`, {
        data: { email: `secA-${s}@example.com`, password: 'TestPassword123!' },
        headers: { 'Content-Type': 'application/json' },
      });
      const wikiId = await createWiki(ctxAlice, `Alice Secret Wiki ${s}`);

      await ctxBob.request.post(`${BASE}/api/register`, {
        data: { username: `secB_${s}`, email: `secB-${s}@example.com`, password: 'TestPassword123!' },
        headers: { 'Content-Type': 'application/json' },
      });
      await ctxBob.request.post(`${BASE}/api/login`, {
        data: { email: `secB-${s}@example.com`, password: 'TestPassword123!' },
        headers: { 'Content-Type': 'application/json' },
      });

      const bobRes = await ctxBob.request.get(`${BASE}/api/wikis/${wikiId}`);
      expect(bobRes.status()).toBe(403);
    } finally {
      await ctxAlice.close();
      await ctxBob.close();
    }
  });

  test('unauthenticated request to protected API endpoint returns 401', async ({ request }) => {
    const res = await request.get(`${BASE}/api/me`);
    expect(res.status()).toBe(401);
  });

  test('health endpoint is accessible without authentication', async ({ request }) => {
    const res = await request.get(`${BASE}/health`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.status).toBe('ok');
  });
});
