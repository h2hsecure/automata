import { chromium } from 'playwright';

const browser = await chromium.launch({ 
  headless: true,
  executablePath: '/home/hekemen/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome'
});
const context = await browser.newContext();
const page = await context.newPage();

// Rewrite IP to hostname for correct Traefik routing
await page.route('http://192.168.178.4/**', async route => {
  const url = route.request().url().replace('http://192.168.178.4', 'http://automata.home.arpa');
  await route.continue({ url });
});

console.log('=== Comprehensive UI Test ===\n');

const errors = [];
const warnings = [];

// Collect console messages
page.on('console', msg => {
  const text = msg.text();
  if (msg.type() === 'error') {
    errors.push(`${msg.type()}: ${text}`);
  }
});

// Collect HTTP errors
page.on('response', resp => {
  if (resp.status() >= 400) {
    errors.push(`HTTP ${resp.status()}: ${resp.url()}`);
  }
});

// ========== 1. LOGIN ==========
console.log('1. LOGIN PAGE');
await page.goto('http://192.168.178.4/', { waitUntil: 'networkidle', timeout: 30000 });
console.log('   URL:', page.url());
console.log('   Title:', await page.title());
console.log('   Has login form:', (await page.locator('form').count()) > 0);

// Fill and submit
await page.fill('input[type="email"]', 'admin@automata.local');
await page.fill('input[type="password"]', 'REDACTED');
await page.click('button[type="submit"]');
await page.waitForTimeout(3000);
console.log('   After login URL:', page.url());
console.log('   After login title:', await page.title());

// ========== 2. DASHBOARD ==========
console.log('\n2. DASHBOARD');
try {
  await page.goto('http://192.168.178.4/dashboard', { waitUntil: 'networkidle', timeout: 30000 });
  await page.waitForTimeout(3000);
  console.log('   URL:', page.url());
  const body = await page.textContent('body');
  console.log('   Page loaded:', body.length > 0);
  console.log('   Has content:', body.length > 50);
} catch (e) {
  console.log('   Dashboard error:', e.message);
  warnings.push('Dashboard load failed');
}

// ========== 3. CONTACTS ==========
console.log('\n3. CONTACTS');
try {
  await page.goto('http://192.168.178.4/contacts', { waitUntil: 'networkidle', timeout: 30000 });
  await page.waitForTimeout(3000);
  console.log('   URL:', page.url());
  const body = await page.textContent('body');
  console.log('   Page loaded:', body.length > 0);
} catch (e) {
  console.log('   Contacts error:', e.message);
  warnings.push('Contacts load failed');
}

// ========== 4. BANNERS ==========
console.log('\n4. BANNERS');
try {
  await page.goto('http://192.168.178.4/banners', { waitUntil: 'networkidle', timeout: 30000 });
  await page.waitForTimeout(3000);
  console.log('   URL:', page.url());
  const body = await page.textContent('body');
  console.log('   Page loaded:', body.length > 0);
} catch (e) {
  console.log('   Banners error:', e.message);
  warnings.push('Banners load failed');
}

// ========== 5. FORMS ==========
console.log('\n5. FORMS');
try {
  await page.goto('http://192.168.178.4/forms', { waitUntil: 'networkidle', timeout: 30000 });
  await page.waitForTimeout(3000);
  console.log('   URL:', page.url());
  const body = await page.textContent('body');
  console.log('   Page loaded:', body.length > 0);
} catch (e) {
  console.log('   Forms error:', e.message);
  warnings.push('Forms load failed');
}

// ========== 6. TRACKING ==========
console.log('\n6. TRACKING');
try {
  await page.goto('http://192.168.178.4/tracking', { waitUntil: 'networkidle', timeout: 30000 });
  await page.waitForTimeout(3000);
  console.log('   URL:', page.url());
  const body = await page.textContent('body');
  console.log('   Page loaded:', body.length > 0);
} catch (e) {
  console.log('   Tracking error:', e.message);
  warnings.push('Tracking load failed');
}

// ========== 7. TENANTS ==========
console.log('\n7. TENANTS');
try {
  await page.goto('http://192.168.178.4/tenants', { waitUntil: 'networkidle', timeout: 30000 });
  await page.waitForTimeout(3000);
  console.log('   URL:', page.url());
  const body = await page.textContent('body');
  console.log('   Page loaded:', body.length > 0);
} catch (e) {
  console.log('   Tenants error:', e.message);
  warnings.push('Tenants load failed');
}

// ========== 8. API KEYS ==========
console.log('\n8. API KEYS');
try {
  await page.goto('http://192.168.178.4/api-keys', { waitUntil: 'networkidle', timeout: 30000 });
  await page.waitForTimeout(3000);
  console.log('   URL:', page.url());
  const body = await page.textContent('body');
  console.log('   Page loaded:', body.length > 0);
} catch (e) {
  console.log('   API Keys error:', e.message);
  warnings.push('API Keys load failed');
}

// ========== SUMMARY ==========
await page.screenshot({ path: '/tmp/automata-screenshot.png' });
console.log('\n=== Summary ===');
console.log('Screenshot saved to /tmp/automata-screenshot.png');
console.log(`Console errors: ${errors.length}`);
console.log(`Warnings: ${warnings.length}`);
if (errors.length > 0) {
  console.log('\nErrors:');
  errors.forEach(e => console.log('  -', e));
}
if (warnings.length > 0) {
  console.log('\nWarnings:');
  warnings.forEach(w => console.log('  -', w));
}

await browser.close();
console.log('\n=== Done ===');
