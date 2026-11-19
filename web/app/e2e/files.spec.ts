// The files page's preview.
import { test, expect } from './fixtures.ts';

test('the preview opens and closes in place; a reload and back still work', async ({ page }) => {
	await page.goto('/users');
	await page.click('nav[aria-label="Main"] a:has-text("Files")');
	await expect(page.getByRole('heading', { name: 'Files', level: 1 })).toBeVisible();
	const pages: string[] = [];
	page.on('request', (r) => { if (new URL(r.url()).pathname.startsWith('/api/page/')) pages.push(r.url()); });
	const card = page.locator('.a-filecard').first();
	const name = (await card.locator('.a-uname').textContent())!;
	await card.click();
	const dialog = page.getByRole('dialog', { name });
	await expect(dialog).toBeVisible();
	await expect(page).toHaveURL(/[?&]file=\d+/);
	await page.reload();
	await expect(dialog).toBeVisible();
	await dialog.getByRole('button', { name: 'Close' }).click();
	await expect(dialog).toHaveCount(0);
	await expect(page).toHaveURL(/\/files$/);
	await card.click();
	await expect(dialog).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(dialog).toHaveCount(0);
	expect(pages).toEqual([]);
	await page.goBack();
	await expect(page.getByRole('heading', { name: 'Users', level: 1 })).toBeVisible();
	await page.goForward();
	await expect(page.getByRole('heading', { name: 'Files', level: 1 })).toBeVisible();
	await expect(dialog).toHaveCount(0);
});
