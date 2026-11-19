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

test('a preview\'s photo opens bigger, whole, and Esc closes only that', async ({ page }) => {
	const q = encodeURIComponent(JSON.stringify([{ field: 'file_type', op: '=', value: 'photo' }, { field: 'stored', op: 'is', value: 'yes' }]));
	const f = ((await (await page.request.get(`/api/page/files?f=${q}`)).json()).props.rows as { id: number; name: string }[]).find((r) => r.id % 3 === 1)!;
	await page.setViewportSize({ width: 1280, height: 900 });
	await page.goto(`/files?file=${f.id}`);
	const dialog = page.getByRole('dialog', { name: f.name });
	await dialog.getByRole('button', { name: 'View bigger' }).click();
	const viewer = page.getByRole('dialog', { name: 'Media viewer' });
	const img = viewer.locator('img');
	await expect(img).toBeVisible();
	await expect.poll(() => img.evaluate((i: HTMLImageElement) => i.complete && i.naturalWidth)).toBeGreaterThan(0);
	const m = await img.evaluate((i: HTMLImageElement) => {
		const r = i.getBoundingClientRect();
		return { fit: getComputedStyle(i).objectFit, w: r.width, h: r.height, nw: i.naturalWidth, nh: i.naturalHeight };
	});
	// Up to its natural size, at its own shape.
	expect(m.fit).toBe('contain');
	expect(Math.abs(m.w - m.nw)).toBeLessThan(1);
	expect(Math.abs(m.h - m.nh)).toBeLessThan(1);
	await page.keyboard.press('Escape');
	await expect(viewer).toHaveCount(0);
	await expect(dialog).toBeVisible();
	await dialog.getByRole('button', { name: 'View bigger' }).click();
	await viewer.getByRole('button', { name: 'Close' }).click();
	await expect(viewer).toHaveCount(0);
	await expect(dialog).toBeVisible();
});

test('a very tall picture does not stretch its grid card or its row', async ({ page }) => {
	// Every picture a long screenshot: 100 x 2000.
	await page.route(/\/files\/standin-\d+$/, (r) => r.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="100" height="2000"><rect width="100" height="2000" fill="teal"/></svg>' }));
	await page.setViewportSize({ width: 1280, height: 900 });
	await page.goto('/files');
	const img = page.locator('.a-filecard-media img').first();
	await expect.poll(() => img.evaluate((i: HTMLImageElement) => i.complete && i.naturalHeight)).toBe(2000);
	const boxes = await page.locator('.a-filecard-media').evaluateAll((els) => els.map((e) => { const r = e.getBoundingClientRect(); return { w: r.width, h: r.height }; }));
	expect(boxes.length).toBeGreaterThan(1);
	for (const b of boxes) expect(Math.abs(b.h - b.w * 3 / 4)).toBeLessThan(1.5);
	// The picture is whole inside its box: contained, not cut.
	const m = await img.evaluate((i: HTMLImageElement) => {
		const box = i.closest('.a-filecard-media')!.getBoundingClientRect();
		const r = i.getBoundingClientRect();
		return { fit: getComputedStyle(i).objectFit, inside: r.top >= box.top - 0.5 && r.bottom <= box.bottom + 0.5 && r.left >= box.left - 0.5 && r.right <= box.right + 0.5 };
	});
	expect(m).toEqual({ fit: 'contain', inside: true });
});
