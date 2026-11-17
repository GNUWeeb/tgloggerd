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
