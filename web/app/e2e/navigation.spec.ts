// Server rendering, hydration and navigation without reloads.
import { test, expect } from './fixtures.ts';

test('the server sends the rendered page, which the browser then takes over', async ({ page, request }) => {
	const html = await (await request.get('/groups', { headers: { cookie: 'tgd_as=superadmin' } })).text();
	expect(html).toContain('<h1');
	expect(html).toContain('id="__tgd"');
	expect(html).toContain('data-ssr="1"');
	await page.goto('/groups');
	await expect(page.getByRole('heading', { name: 'Groups', level: 1 })).toBeVisible();
});

test('links navigate without reloading, and back returns', async ({ page }) => {
	await page.goto('/');
	await page.evaluate(() => { (window as unknown as { __mark: number }).__mark = 1; });
	await page.click('nav[aria-label="Main"] a:has-text("Users")');
	await expect(page.getByRole('heading', { name: 'Users', level: 1 })).toBeVisible();
	await expect(page).toHaveURL(/\/users$/);
	expect(await page.evaluate(() => (window as unknown as { __mark?: number }).__mark)).toBe(1);
	await page.goBack();
	await expect(page.getByRole('heading', { name: 'Dashboard', level: 1 })).toBeVisible();
	expect(await page.evaluate(() => (window as unknown as { __mark?: number }).__mark)).toBe(1);
});

test('without the render service the browser renders the page itself', async ({ page }) => {
	await page.goto('/users?ssr=0');
	await expect(page.locator('#root')).toHaveAttribute('data-ssr', '0');
	await expect(page.getByRole('heading', { name: 'Users', level: 1 })).toBeVisible();
});

test('a list search lives in the URL and survives a reload', async ({ page }) => {
	await page.goto('/group_messages');
	await page.fill('input[aria-label="Search"]', 'rust');
	await page.press('input[aria-label="Search"]', 'Enter');
	await expect(page).toHaveURL(/q=rust/);
	await page.reload();
	await expect(page.locator('input[aria-label="Search"]')).toHaveValue('rust');
});

test('Ctrl-K jumps to a chat by name', async ({ page }) => {
	await page.goto('/');
	await expect(page.locator('.a-livestate')).toBeVisible();
	await page.keyboard.press('Control+k');
	const box = page.getByRole('dialog', { name: 'Search everything' });
	await expect(box.getByRole('combobox')).toBeFocused();
	await box.getByRole('combobox').fill('Linux Kernel');
	await expect(box.getByRole('option', { name: /Linux Kernel Indonesia/ })).toBeVisible();
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(/\/groups\/-\d+$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Linux Kernel Indonesia');
	await expect(box).toHaveCount(0);
});

test('the theme chosen in the top bar is kept across a reload', async ({ page }) => {
	await page.goto('/');
	await expect(page.locator('.a-livestate')).toBeVisible();
	const saved = page.waitForResponse((r) => r.url().endsWith('/api/account/preferences') && r.request().method() === 'PATCH');
	await page.getByRole('button', { name: 'Theme: system' }).click();
	expect((await saved).status()).toBe(200);
	await page.reload();
	// Rendered by the server already: no flash back to the old theme.
	await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
	await expect(page.getByRole('button', { name: 'Theme: light' })).toBeVisible();
});

test('the dashboard\'s per-day bars give the day and count on hover', async ({ page }) => {
	await page.goto('/');
	await expect(page.locator('.a-livestate')).toBeVisible();
	const today = page.getByRole('img', { name: /^Today, / });
	await today.hover();
	const tip = page.locator('.a-daybars-tip');
	await expect(tip).toBeVisible();
	await expect(tip).toContainText('Today');
	await expect(tip).toContainText('messages');
	// Inside the card, not cut off at its right edge.
	const card = await page.locator('.a-bigchart').boundingBox();
	const box = await tip.boundingBox();
	expect(box!.x + box!.width).toBeLessThanOrEqual(card!.x + card!.width + 1);
	await expect(page.locator('.a-daybars-axis')).toContainText('Today');
});

test('the dashboard fits the window: nothing hidden makes the page taller', async ({ page }) => {
	await page.setViewportSize({ width: 1920, height: 1080 });
	await page.goto('/');
	await expect(page.getByRole('img', { name: /^Today, / })).toBeVisible();
	// The shell fills the window and scrolls inside; the page itself
	// never does.
	expect(await page.evaluate(() => document.documentElement.scrollHeight - innerHeight)).toBeLessThanOrEqual(0);
});
