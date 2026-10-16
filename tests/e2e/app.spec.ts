import { test, expect } from '@playwright/test';

const BASE = 'http://localhost:8081';
const EMAIL = 'admin@demo.com';
const PASSWORD = 'REDACTED';

async function login(page: any) {
  await page.goto(`${BASE}/login`);
  await page.waitForLoadState('networkidle');
  await page.waitForSelector('form');
  
  // Fill fields by placeholder since labels aren't associated with inputs
  await page.fill('input[type="email"]', EMAIL);
  await page.fill('input[type="password"]', PASSWORD);
  
  // Listen for console messages to debug login
  const consoleMessages: string[] = [];
  page.on('console', msg => consoleMessages.push(msg.text()));
  
  await page.click('button[type="submit"]');
  
  // Wait for navigation with a longer timeout and better detection
  await page.waitForURL('**/dashboard**', { timeout: 15000 });
}

test.describe('Automata E2E Tests', () => {
  test('health endpoint returns ok', async ({ request }) => {
    const res = await request.get(`${BASE}/api/health`);
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body).toEqual({ status: 'ok' });
  });

  test('SPA serves index.html at root', async ({ page }) => {
    await page.goto(BASE);
    await page.waitForLoadState('networkidle');
    // The Vue app should have rendered something
    await expect(page.locator('#app')).toBeVisible();
  });

  test('API routes return JSON, not HTML', async ({ request }) => {
    const res = await request.get(`${BASE}/api/health`);
    expect(res.headers()['content-type']).toContain('application/json');
  });
});

test.describe('Authentication', () => {
  test('login page loads', async ({ page }) => {
    await page.goto(`${BASE}/login`);
    await page.waitForLoadState('networkidle');
    await page.waitForSelector('form');
    // Check that the form has the expected inputs
    await expect(page.locator('input[type="email"]')).toBeVisible();
    await expect(page.locator('input[type="password"]')).toBeVisible();
  });

  test('login with valid credentials redirects to dashboard', async ({ page }) => {
    await login(page);
    await expect(page).toHaveURL(/.*\/dashboard/);
  });

  test('login with invalid credentials shows error', async ({ page }) => {
    await page.goto(`${BASE}/login`);
    await page.waitForLoadState('networkidle');
    await page.waitForSelector('form');
    await page.fill('input[type="email"]', EMAIL);
    await page.fill('input[type="password"]', 'wrongpassword');
    await page.click('button[type="submit"]');
    // Should show an error message
    await expect(page.locator('.text-destructive')).toBeVisible({ timeout: 5000 });
  });

  test('dashboard loads after login', async ({ page }) => {
    await login(page);
    // Dashboard should be visible
    await expect(page.locator('#app')).toBeVisible();
  });
});

test.describe('Contacts', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('contacts page loads', async ({ page }) => {
    await page.goto(`${BASE}/contacts`);
    await page.waitForLoadState('networkidle');
    await expect(page.locator('#app')).toBeVisible();
  });

  test('contacts list loads (may be empty)', async ({ page }) => {
    await page.goto(`${BASE}/contacts`);
    await page.waitForLoadState('networkidle');
    // Either shows a table or a "no contacts" message
    await expect(page.locator('table')).toBeVisible({ timeout: 5000 });
  });

  test('search input is visible', async ({ page }) => {
    await page.goto(`${BASE}/contacts`);
    await page.waitForLoadState('networkidle');
    await expect(page.getByPlaceholder(/search/i)).toBeVisible();
  });
});

test.describe('Forms', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('forms page loads', async ({ page }) => {
    await page.goto(`${BASE}/forms`);
    await page.waitForLoadState('networkidle');
    await expect(page.locator('#app')).toBeVisible();
  });

  test('create form button is visible', async ({ page }) => {
    await page.goto(`${BASE}/forms`);
    await page.waitForLoadState('networkidle');
    await expect(page.getByRole('button', { name: /create form|add form/i, ignoreCase: true })).toBeVisible();
  });
});

test.describe('API Keys', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('api-keys page loads', async ({ page }) => {
    await page.goto(`${BASE}/api-keys`);
    await page.waitForLoadState('networkidle');
    await expect(page.locator('#app')).toBeVisible();
  });
});

test.describe('Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('sidebar shows all navigation items', async ({ page }) => {
    await page.goto(`${BASE}/dashboard`);
    await page.waitForLoadState('networkidle');
    await expect(page.getByRole('link', { name: 'Dashboard' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Contacts' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Forms' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'API Keys' })).toBeVisible();
  });

  test('navigation between pages works', async ({ page }) => {
    await page.goto(`${BASE}/dashboard`);
    await page.waitForLoadState('networkidle');
    
    await page.getByRole('link', { name: 'Contacts' }).click();
    await page.waitForURL('**/contacts**');
    await expect(page.locator('#app')).toBeVisible();

    await page.getByRole('link', { name: 'Forms' }).click();
    await page.waitForURL('**/forms**');
    await expect(page.locator('#app')).toBeVisible();

    await page.getByText('API Keys').click();
    await page.waitForFunction(() => window.location.pathname.includes('/api-keys'));
    await expect(page.locator('#app')).toBeVisible();
  });
});

test.describe('API Endpoints', () => {
  test('admin api-keys endpoint exists', async ({ request }) => {
    const loginRes = await request.post(`${BASE}/api/auth/login`, {
      data: { email: 'admin@demo.com', password: 'REDACTED' }
    });
    const { token } = await loginRes.json();
    
    // POST creates an API key (returns 201)
    const res = await request.post(`${BASE}/api/admin/api-keys`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { name: `test-key-${Date.now()}` }
    });
    expect(res.status()).toBe(201);
  });
});
