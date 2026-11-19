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

test('the preview shows a photo whole, at its own shape', async ({ page }) => {
	const q = encodeURIComponent(JSON.stringify([{ field: 'file_type', op: '=', value: 'photo' }, { field: 'stored', op: 'is', value: 'yes' }]));
	const rows = (await (await page.request.get(`/api/page/files?f=${q}`)).json()).props.rows as { id: number; name: string; thumb: string | null }[];
	// The stand-in draws a file whose id is 1 more than a multiple of 3
	// portrait, which a 4:3 crop would cut.
	const f = rows.find((r) => r.id % 3 === 1 && r.thumb)!;
	expect(f, 'a stored portrait photo in the stand-in').toBeTruthy();
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto(`/files?file=${f.id}`);
	const img = page.getByRole('dialog', { name: f.name }).locator('.a-preview img');
	await expect(img).toBeVisible();
	await expect.poll(() => img.evaluate((i: HTMLImageElement) => i.complete && i.naturalWidth)).toBeGreaterThan(0);
	const m = await img.evaluate((i: HTMLImageElement) => {
		const r = i.getBoundingClientRect();
		return { fit: getComputedStyle(i).objectFit, w: r.width, h: r.height, nw: i.naturalWidth, nh: i.naturalHeight };
	});
	expect(m.fit).toBe('contain');
	expect(m.nh).toBeGreaterThan(m.nw);
	expect(Math.abs(m.w / m.h - m.nw / m.nh)).toBeLessThan(0.02);
	expect(m.h).toBeLessThanOrEqual(800 * 0.7 + 1);
});
