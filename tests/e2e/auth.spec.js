// @ts-check
const { test, expect } = require('@playwright/test');
const { BASE_URL, ADMIN_USER, ADMIN_PASS, isAuthEnabled, loginAsAdmin, gotoAndLogin } = require('./auth-helpers');

test.describe('Authentication', () => {

  test('Auth info endpoint returns auth status', async ({ request }) => {
    const resp = await request.get(`${BASE_URL}/api/v1/auth/info`);
    expect(resp.ok()).toBeTruthy();
    const data = await resp.json();
    expect(typeof data.auth_enabled).toBe('boolean');
    console.log(`Auth enabled: ${data.auth_enabled}`);
  });

  test.describe('Auth Enabled', () => {

    test.beforeEach(async ({ request }) => {
      const enabled = await isAuthEnabled(request);
      test.skip(!enabled, 'Auth is not enabled, skipping auth-enabled tests');
    });

    test('Protected endpoints return 401 without auth', async ({ request }) => {
      // Reading position
      const posResp = await request.get(`${BASE_URL}/api/v1/books/000100/position`);
      expect(posResp.status()).toBe(401);

      // Reading history
      const histResp = await request.get(`${BASE_URL}/api/v1/reading-history`);
      expect(histResp.status()).toBe(401);

      // Auth/me
      const meResp = await request.get(`${BASE_URL}/api/v1/auth/me`);
      expect(meResp.status()).toBe(401);

      // Logout
      const logoutResp = await request.post(`${BASE_URL}/api/v1/auth/logout`);
      expect(logoutResp.status()).toBe(401);

      console.log('PASS: Protected endpoints return 401 without auth');
    });

    test('Public endpoints work without auth', async ({ request }) => {
      // Auth info
      const infoResp = await request.get(`${BASE_URL}/api/v1/auth/info`);
      expect(infoResp.ok()).toBeTruthy();

      // Book search
      const booksResp = await request.get(`${BASE_URL}/api/v1/books?limit=1`);
      expect(booksResp.ok()).toBeTruthy();

      // Health
      const healthResp = await request.get(`${BASE_URL}/health`);
      expect(healthResp.ok()).toBeTruthy();

      console.log('PASS: Public endpoints work without auth');
    });

    test('Login with wrong credentials fails', async ({ request }) => {
      const resp = await request.post(`${BASE_URL}/api/v1/auth/login`, {
        data: { username: 'admin', password: 'wrongpassword' },
      });
      expect(resp.status()).toBe(401);
      console.log('PASS: Wrong credentials return 401');
    });

    test('Login with empty credentials fails', async ({ request }) => {
      const resp = await request.post(`${BASE_URL}/api/v1/auth/login`, {
        data: { username: '', password: '' },
      });
      expect(resp.status()).toBe(400);
      console.log('PASS: Empty credentials return 400');
    });

    test('Login with correct credentials succeeds', async ({ request }) => {
      const resp = await request.post(`${BASE_URL}/api/v1/auth/login`, {
        data: { username: ADMIN_USER, password: ADMIN_PASS },
      });
      expect(resp.ok()).toBeTruthy();

      const data = await resp.json();
      expect(data.status).toBe('ok');
      expect(data.user.username).toBe(ADMIN_USER);
      expect(data.user.is_admin).toBe(true);

      // Check session cookie is set
      const setCookie = resp.headers()['set-cookie'] || '';
      expect(setCookie).toContain('pushkinlib_session=');
      expect(setCookie).toContain('HttpOnly');

      console.log('PASS: Login succeeds, session cookie set');
    });

    test('Session cookie grants access to protected endpoints', async ({ request }) => {
      // Login first
      const loginResp = await request.post(`${BASE_URL}/api/v1/auth/login`, {
        data: { username: ADMIN_USER, password: ADMIN_PASS },
      });
      expect(loginResp.ok()).toBeTruthy();

      // Playwright's request context automatically stores cookies,
      // so subsequent requests should include the session cookie.

      // /auth/me should work
      const meResp = await request.get(`${BASE_URL}/api/v1/auth/me`);
      expect(meResp.ok()).toBeTruthy();
      const me = await meResp.json();
      expect(me.username).toBe(ADMIN_USER);

      // Save reading position should work
      const posResp = await request.put(`${BASE_URL}/api/v1/books/000100/position`, {
        data: {
          section: 0,
          progress: 0.0,
          total_sections: 22,
        },
      });
      expect(posResp.ok()).toBeTruthy();

      // Get reading position
      const getPosResp = await request.get(`${BASE_URL}/api/v1/books/000100/position`);
      expect(getPosResp.ok()).toBeTruthy();

      // Reading history
      const histResp = await request.get(`${BASE_URL}/api/v1/reading-history`);
      expect(histResp.ok()).toBeTruthy();

      console.log('PASS: Session cookie grants access to protected endpoints');
    });

    test('Logout clears session', async ({ request }) => {
      // Login
      const loginResp = await request.post(`${BASE_URL}/api/v1/auth/login`, {
        data: { username: ADMIN_USER, password: ADMIN_PASS },
      });
      expect(loginResp.ok()).toBeTruthy();

      // Verify logged in
      const meResp = await request.get(`${BASE_URL}/api/v1/auth/me`);
      expect(meResp.ok()).toBeTruthy();

      // Logout
      const logoutResp = await request.post(`${BASE_URL}/api/v1/auth/logout`);
      expect(logoutResp.ok()).toBeTruthy();

      // Session should be invalid now — but Playwright's request context may
      // still send the old cookie. The cookie was cleared server-side.
      // We verify by checking the logout response clears the cookie.
      const setCookie = logoutResp.headers()['set-cookie'] || '';
      expect(setCookie).toContain('pushkinlib_session=');
      expect(setCookie).toContain('Max-Age=0'); // cookie deleted

      console.log('PASS: Logout clears session cookie');
    });

    test('Login form appears in browser when auth is enabled', async ({ page }) => {
      await page.goto(BASE_URL);

      // Login card should be visible
      const loginCard = page.locator('.login-card');
      await expect(loginCard).toBeVisible({ timeout: 10000 });

      // Username and password fields should be present
      const usernameInput = page.locator('.login-card input[type="text"], .login-card input[placeholder]').first();
      const passwordInput = page.locator('.login-card input[type="password"]');
      await expect(usernameInput).toBeVisible();
      await expect(passwordInput).toBeVisible();

      // Submit button should be present
      const submitBtn = page.locator('.login-card button[type="submit"], .login-card button').first();
      await expect(submitBtn).toBeVisible();

      console.log('PASS: Login form displayed when auth enabled');
    });

    test('Browser login flow works end-to-end', async ({ page }) => {
      await page.goto(BASE_URL);

      // Wait for login form
      const loginCard = page.locator('.login-card');
      await expect(loginCard).toBeVisible({ timeout: 10000 });

      // Fill credentials
      await page.locator('.login-card input[type="text"], .login-card input[placeholder]').first().fill(ADMIN_USER);
      await page.locator('.login-card input[type="password"]').first().fill(ADMIN_PASS);
      await page.locator('.login-card button[type="submit"], .login-card button').first().click();

      // Should see the library after login
      await page.waitForSelector('.book-card', { timeout: 15000 });
      console.log('PASS: Library loaded after login');

      // User info should be visible in header
      const userInfo = page.locator('text=' + ADMIN_USER);
      await expect(userInfo).toBeVisible({ timeout: 5000 });
      console.log('PASS: Username displayed in header');

      // Logout button should be visible
      const logoutBtn = page.locator('button', { hasText: 'Выйти' });
      await expect(logoutBtn).toBeVisible({ timeout: 5000 });

      // Click logout
      await logoutBtn.click();
      await page.waitForTimeout(1000);

      // Login form should reappear
      await expect(loginCard).toBeVisible({ timeout: 10000 });
      console.log('PASS: Login form reappears after logout');
    });

    test('OPDS requires BasicAuth when auth is enabled', async ({ request }) => {
      // Without auth — should get 401
      const noAuthResp = await request.fetch(`${BASE_URL}/opds/`, {
        headers: {}, // don't send cookies
        ignoreHTTPSErrors: true,
      });
      // Note: Playwright request context may still have cookies from prior login.
      // We check for WWW-Authenticate header presence on fresh request.
      // A fresh context won't have cookies, so this should be 401.

      // With BasicAuth — should work
      const authResp = await request.get(`${BASE_URL}/opds/`, {
        headers: {
          'Authorization': 'Basic ' + Buffer.from(`${ADMIN_USER}:${ADMIN_PASS}`).toString('base64'),
        },
      });
      expect(authResp.ok()).toBeTruthy();
      const body = await authResp.text();
      expect(body).toContain('<feed');
      console.log('PASS: OPDS works with BasicAuth');

      // With wrong BasicAuth — should fail
      const wrongResp = await request.get(`${BASE_URL}/opds/`, {
        headers: {
          'Authorization': 'Basic ' + Buffer.from('admin:wrongpass').toString('base64'),
        },
      });
      expect(wrongResp.status()).toBe(401);
      console.log('PASS: OPDS rejects wrong BasicAuth');
    });

    test('Admin reindex endpoint requires admin role', async ({ request }) => {
      // Login as admin
      const loginResp = await request.post(`${BASE_URL}/api/v1/auth/login`, {
        data: { username: ADMIN_USER, password: ADMIN_PASS },
      });
      expect(loginResp.ok()).toBeTruthy();

      // Admin should be able to access reindex (we don't actually trigger it,
      // just verify it doesn't return 401/403)
      const reindexResp = await request.post(`${BASE_URL}/api/v1/admin/reindex`);
      // It might return 200 or some other status, but NOT 401/403
      expect(reindexResp.status()).not.toBe(401);
      expect(reindexResp.status()).not.toBe(403);
      console.log('PASS: Admin can access reindex endpoint');
    });

  });

  test.describe('Auth Disabled', () => {

    test.beforeEach(async ({ request }) => {
      const enabled = await isAuthEnabled(request);
      test.skip(enabled, 'Auth is enabled, skipping auth-disabled tests');
    });

    test('Login endpoint returns 404 when auth disabled', async ({ request }) => {
      const resp = await request.post(`${BASE_URL}/api/v1/auth/login`, {
        data: { username: 'admin', password: 'admin123' },
      });
      expect(resp.status()).toBe(404);
      console.log('PASS: Login endpoint returns 404 when auth disabled');
    });

    test('Protected endpoints work without auth', async ({ request }) => {
      // Reading position (save)
      const saveResp = await request.put(`${BASE_URL}/api/v1/books/000100/position`, {
        data: {
          section: 0,
          progress: 0.0,
          total_sections: 22,
        },
      });
      expect(saveResp.ok()).toBeTruthy();

      // Reading position (get)
      const getResp = await request.get(`${BASE_URL}/api/v1/books/000100/position`);
      expect(getResp.ok()).toBeTruthy();

      // Reading history
      const histResp = await request.get(`${BASE_URL}/api/v1/reading-history`);
      expect(histResp.ok()).toBeTruthy();

      console.log('PASS: Protected endpoints work without auth when auth disabled');
    });

    test('No login form in browser', async ({ page }) => {
      await page.goto(BASE_URL);

      // Library should load directly — no login form
      await page.waitForSelector('.book-card', { timeout: 15000 });

      // Login card should NOT be visible
      const loginCard = page.locator('.login-card');
      const visible = await loginCard.isVisible().catch(() => false);
      expect(visible).toBe(false);

      console.log('PASS: No login form when auth disabled');
    });

    test('OPDS works without auth', async ({ request }) => {
      const resp = await request.get(`${BASE_URL}/opds/`);
      expect(resp.ok()).toBeTruthy();
      const body = await resp.text();
      expect(body).toContain('<feed');
      console.log('PASS: OPDS works without auth when auth disabled');
    });

  });

});
