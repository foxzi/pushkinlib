// @ts-check

/**
 * Auth helpers for e2e tests.
 * Provides utilities for detecting auth mode and logging in.
 */

const BASE_URL = 'http://localhost:9090';
const ADMIN_USER = 'admin';
const ADMIN_PASS = 'admin123';

/**
 * Check if auth is enabled on the server.
 * @param {import('@playwright/test').APIRequestContext} request
 * @returns {Promise<boolean>}
 */
async function isAuthEnabled(request) {
  const resp = await request.get(`${BASE_URL}/api/v1/auth/info`);
  const data = await resp.json();
  return data.auth_enabled === true;
}

/**
 * Login as admin and return the session cookie.
 * Returns null if auth is disabled.
 * @param {import('@playwright/test').APIRequestContext} request
 * @returns {Promise<string|null>} session cookie value
 */
async function loginAsAdmin(request) {
  const authEnabled = await isAuthEnabled(request);
  if (!authEnabled) return null;

  const resp = await request.post(`${BASE_URL}/api/v1/auth/login`, {
    data: { username: ADMIN_USER, password: ADMIN_PASS },
  });
  if (!resp.ok()) {
    throw new Error(`Login failed: ${resp.status()} ${await resp.text()}`);
  }

  // Extract session cookie from response headers
  const setCookie = resp.headers()['set-cookie'] || '';
  const match = setCookie.match(/pushkinlib_session=([^;]+)/);
  return match ? match[1] : null;
}

/**
 * Login via the browser page (fills the login form).
 * No-op if auth is disabled or user is already logged in.
 * @param {import('@playwright/test').Page} page
 */
async function loginViaUI(page) {
  // Check if login form is shown
  const loginForm = page.locator('.login-card');
  const isLoginVisible = await loginForm.isVisible({ timeout: 3000 }).catch(() => false);

  if (!isLoginVisible) return; // no login needed

  await page.locator('input[type="text"], input[placeholder*="Имя"]').first().fill(ADMIN_USER);
  await page.locator('input[type="password"]').first().fill(ADMIN_PASS);
  await page.locator('button[type="submit"], .login-card button').first().click();

  // Wait for login to complete — library should load
  await page.waitForSelector('.book-card', { timeout: 15000 });
}

/**
 * Navigate to the app and handle auth if needed.
 * @param {import('@playwright/test').Page} page
 */
async function gotoAndLogin(page) {
  await page.goto(BASE_URL);
  await loginViaUI(page);
}

module.exports = {
  BASE_URL,
  ADMIN_USER,
  ADMIN_PASS,
  isAuthEnabled,
  loginAsAdmin,
  loginViaUI,
  gotoAndLogin,
};
