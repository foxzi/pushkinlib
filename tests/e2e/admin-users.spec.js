// @ts-check
const { test, expect } = require('@playwright/test');
const { BASE_URL, ADMIN_USER, ADMIN_PASS, isAuthEnabled, loginAsAdmin, gotoAndLogin } = require('./auth-helpers');

test.describe('Admin User Management', () => {

  test.beforeEach(async ({ request }) => {
    const enabled = await isAuthEnabled(request);
    test.skip(!enabled, 'Auth is not enabled, skipping admin user management tests');
    await loginAsAdmin(request);
  });

  test('GET /admin/users returns user list', async ({ request }) => {
    const resp = await request.get(`${BASE_URL}/api/v1/admin/users`);
    expect(resp.ok()).toBeTruthy();
    const users = await resp.json();
    expect(Array.isArray(users)).toBeTruthy();
    expect(users.length).toBeGreaterThanOrEqual(1);
    // Admin user should exist
    const admin = users.find(u => u.username === ADMIN_USER);
    expect(admin).toBeTruthy();
    expect(admin.is_admin).toBe(true);
    console.log(`PASS: Listed ${users.length} user(s)`);
  });

  test('Create, list, and delete a user via API', async ({ request }) => {
    const testUsername = `testuser_${Date.now()}`;

    // Create user
    const createResp = await request.post(`${BASE_URL}/api/v1/admin/users`, {
      data: {
        username: testUsername,
        password: 'testpass123',
        display_name: 'Test User',
        is_admin: false,
      },
    });
    expect(createResp.status()).toBe(201);
    const created = await createResp.json();
    expect(created.username).toBe(testUsername);
    expect(created.display_name).toBe('Test User');
    expect(created.is_admin).toBe(false);
    expect(created.id).toBeTruthy();
    console.log(`Created user: ${created.username} (${created.id})`);

    // List users — new user should appear
    const listResp = await request.get(`${BASE_URL}/api/v1/admin/users`);
    const users = await listResp.json();
    const found = users.find(u => u.username === testUsername);
    expect(found).toBeTruthy();
    console.log(`User found in list`);

    // New user can log in
    const loginResp = await request.post(`${BASE_URL}/api/v1/auth/login`, {
      data: { username: testUsername, password: 'testpass123' },
    });
    expect(loginResp.ok()).toBeTruthy();
    console.log(`New user logged in successfully`);

    // Re-login as admin (previous login changed the session)
    await loginAsAdmin(request);

    // Delete user
    const deleteResp = await request.delete(`${BASE_URL}/api/v1/admin/users/${created.id}`);
    expect(deleteResp.ok()).toBeTruthy();
    console.log(`User deleted`);

    // Verify user is gone
    const listResp2 = await request.get(`${BASE_URL}/api/v1/admin/users`);
    const users2 = await listResp2.json();
    const gone = users2.find(u => u.username === testUsername);
    expect(gone).toBeFalsy();
    console.log(`PASS: User no longer in list`);
  });

  test('Create user with duplicate username returns 409', async ({ request }) => {
    const resp = await request.post(`${BASE_URL}/api/v1/admin/users`, {
      data: {
        username: ADMIN_USER,
        password: 'secret123',
      },
    });
    expect(resp.status()).toBe(409);
    console.log(`PASS: Duplicate username rejected`);
  });

  test('Create user with short password returns 400', async ({ request }) => {
    const resp = await request.post(`${BASE_URL}/api/v1/admin/users`, {
      data: {
        username: 'shortpw',
        password: '12345',
      },
    });
    expect(resp.status()).toBe(400);
    console.log(`PASS: Short password rejected`);
  });

  test('Change user password via API', async ({ request }) => {
    const testUsername = `pwchange_${Date.now()}`;

    // Create user
    const createResp = await request.post(`${BASE_URL}/api/v1/admin/users`, {
      data: { username: testUsername, password: 'oldpass123' },
    });
    expect(createResp.status()).toBe(201);
    const user = await createResp.json();

    // Change password
    const pwResp = await request.put(`${BASE_URL}/api/v1/admin/users/${user.id}/password`, {
      data: { password: 'newpass789' },
    });
    expect(pwResp.ok()).toBeTruthy();
    console.log(`Password changed`);

    // Login with old password should fail
    const oldLoginResp = await request.post(`${BASE_URL}/api/v1/auth/login`, {
      data: { username: testUsername, password: 'oldpass123' },
    });
    expect(oldLoginResp.status()).toBe(401);

    // Login with new password should work
    const newLoginResp = await request.post(`${BASE_URL}/api/v1/auth/login`, {
      data: { username: testUsername, password: 'newpass789' },
    });
    expect(newLoginResp.ok()).toBeTruthy();
    console.log(`PASS: Password change verified`);

    // Cleanup: re-login as admin and delete user
    await loginAsAdmin(request);
    await request.delete(`${BASE_URL}/api/v1/admin/users/${user.id}`);
  });

  test('Non-admin user cannot access admin endpoints', async ({ request }) => {
    // Create a non-admin user
    const testUsername = `nonadmin_${Date.now()}`;
    const createResp = await request.post(`${BASE_URL}/api/v1/admin/users`, {
      data: { username: testUsername, password: 'secret123', is_admin: false },
    });
    const user = await createResp.json();

    // Login as non-admin
    await request.post(`${BASE_URL}/api/v1/auth/login`, {
      data: { username: testUsername, password: 'secret123' },
    });

    // Try to list users — should get 403
    const listResp = await request.get(`${BASE_URL}/api/v1/admin/users`);
    expect(listResp.status()).toBe(403);

    // Try to create user — should get 403
    const createResp2 = await request.post(`${BASE_URL}/api/v1/admin/users`, {
      data: { username: 'sneaky', password: 'secret123' },
    });
    expect(createResp2.status()).toBe(403);

    console.log(`PASS: Non-admin blocked from admin endpoints`);

    // Cleanup
    await loginAsAdmin(request);
    await request.delete(`${BASE_URL}/api/v1/admin/users/${user.id}`);
  });

  test('Admin panel UI shows user list', async ({ page }) => {
    await gotoAndLogin(page);

    // Click the admin/users button
    const adminBtn = page.locator('button', { hasText: 'Пользователи' });
    await expect(adminBtn).toBeVisible({ timeout: 5000 });
    await adminBtn.click();

    // User table should appear
    const table = page.locator('.admin-table');
    await expect(table).toBeVisible({ timeout: 5000 });

    // Admin user should be in the table
    const adminRow = page.locator('.admin-table tr', { hasText: ADMIN_USER });
    await expect(adminRow).toBeVisible();

    console.log('PASS: Admin panel shows user list');
  });

  test('Admin panel UI can create and delete a user', async ({ page }) => {
    await gotoAndLogin(page);

    // Navigate to admin panel
    await page.locator('button', { hasText: 'Пользователи' }).click();
    await expect(page.locator('.admin-table')).toBeVisible({ timeout: 5000 });

    const testUsername = `uitest_${Date.now()}`;

    // Fill create form
    await page.locator('.admin-form input[placeholder="login"]').fill(testUsername);
    await page.locator('.admin-form input[placeholder*="6 символов"]').fill('testpass123');
    await page.locator('.admin-form input[placeholder*="Фамилия"]').fill('UI Test User');

    // Click create
    await page.locator('.admin-form button', { hasText: 'Создать' }).click();

    // Wait for success message
    await expect(page.locator('.admin-alert-success')).toBeVisible({ timeout: 5000 });

    // New user should appear in the table
    const newUserRow = page.locator('.admin-table td', { hasText: testUsername });
    await expect(newUserRow).toBeVisible();

    console.log(`Created user "${testUsername}" via UI`);

    // Set up dialog handler before clicking delete
    page.once('dialog', dialog => dialog.accept());

    // Delete the user
    const row = page.locator('.admin-table tr', { hasText: testUsername });
    await row.locator('button', { hasText: 'Удалить' }).click();

    // Wait for the user to disappear from the table
    await expect(page.locator('.admin-table td', { hasText: testUsername })).toBeHidden({ timeout: 5000 });

    console.log(`PASS: User created and deleted via UI`);
  });

});
