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

test('the grid shows a .tgs file as its type icon, a .webm by its still, and draws no broken picture', async ({ page }) => {
	await page.goto('/files?per=100');
	const card = (ext: string) => page.locator('.a-filecard', { has: page.locator('.a-uname', { hasText: new RegExp(`\\.${ext}$`) }) });
	const tgs = card('tgs').first();
	await expect(tgs).toBeVisible();
	await expect(tgs.locator('img')).toHaveCount(0);
	await expect(tgs.locator('.a-thumb-doc')).toBeVisible();
	// A .webm with a still shows that picture, never the video itself.
	const webm = card('webm').filter({ has: page.locator('img:not(.a-thumb-mini)') }).first();
	await expect(webm).toBeVisible();
	expect(await webm.locator('img:not(.a-thumb-mini)').getAttribute('src')).not.toMatch(/\.webm$/);
	const imgs = page.locator('.a-filecard img');
	await expect.poll(() => imgs.evaluateAll((els) => els.every((i) => (i as HTMLImageElement).complete))).toBe(true);
	expect(await imgs.evaluateAll((els) => els.filter((i) => (i as HTMLImageElement).naturalWidth === 0).map((i) => i.getAttribute('src')))).toEqual([]);
});

test('a video card shows its still with the play mark, or its blurred preview, and loads none of the video', async ({ page }) => {
	const asked: string[] = [];
	page.on('request', (r) => asked.push(new URL(r.url()).pathname));
	const q = encodeURIComponent(JSON.stringify([{ field: 'file_type', op: '=', value: 'video' }]));
	await page.goto(`/files?per=100&f=${q}`);
	const rows = (await (await page.request.get(`/api/page/files?per=100&f=${q}`)).json()).props.rows as { id: number; name: string; url: string | null; thumb: string | null; minithumb?: string }[];
	const withStill = rows.find((r) => r.thumb)!;
	const blurOnly = rows.find((r) => !r.thumb && r.minithumb)!;
	expect(withStill && blurOnly, 'stand-in videos with a still and with only a blurred preview').toBeTruthy();
	const card = (name: string) => page.locator('.a-filecard', { has: page.locator('.a-uname', { hasText: name }) }).first();
	const still = card(withStill.name).locator(`img[src="${withStill.thumb}"]`);
	await expect.poll(() => still.evaluate((i: HTMLImageElement) => i.complete && i.naturalWidth)).toBeGreaterThan(0);
	await expect(card(withStill.name).locator('.a-thumb-play')).toBeVisible();
	const mini = card(blurOnly.name).locator('img.a-thumb-mini');
	await expect.poll(() => mini.evaluate((i: HTMLImageElement) => i.complete && i.naturalWidth)).toBeGreaterThan(0);
	expect(await mini.evaluate((i) => getComputedStyle(i).filter)).toContain('blur');
	await expect(page.locator('.a-filegrid video')).toHaveCount(0);
	const own = new Set(rows.map((r) => r.url).filter(Boolean));
	expect(asked.filter((u) => own.has(u))).toEqual([]);
});

test('a video\'s preview shows its poster, loads none of it, and plays it on a click', async ({ page }) => {
	const asked: string[] = [];
	page.on('request', (r) => asked.push(new URL(r.url()).pathname));
	const q = encodeURIComponent(JSON.stringify([{ field: 'file_type', op: '=', value: 'video' }, { field: 'stored', op: 'is', value: 'yes' }]));
	const rows = (await (await page.request.get(`/api/page/files?per=100&f=${q}`)).json()).props.rows as { id: number; name: string; url: string; thumb: string | null }[];
	const f = rows.find((r) => r.thumb)!;
	expect(f, 'a stored video with a still').toBeTruthy();
	await page.goto(`/files?file=${f.id}`);
	const dialog = page.getByRole('dialog', { name: f.name });
	const box = dialog.getByRole('link', { name: 'Video' });
	await expect(box.locator('video')).toHaveAttribute('poster', f.thumb!);
	await expect(box.locator('video')).toHaveAttribute('preload', 'none');
	await expect(box.locator('.c-playbig')).toBeVisible();
	await expect.poll(() => asked.includes(f.thumb!)).toBe(true);
	expect(asked.filter((u) => u === f.url)).toEqual([]);
	await box.click();
	const viewer = page.getByRole('dialog', { name: 'Media viewer' });
	await expect(viewer.locator('video')).toHaveAttribute('src', f.url);
	await expect.poll(() => asked.includes(f.url)).toBe(true);
	// Esc closes the player, then the preview.
	await page.keyboard.press('Escape');
	await expect(viewer).toHaveCount(0);
	await expect(dialog).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(dialog).toHaveCount(0);
});

test('the grid fills every row of a batch and scrolls for more', async ({ page }) => {
	const cols = () => page.locator('.a-filegrid').evaluate((g) => getComputedStyle(g).gridTemplateColumns.split(' ').length);
	// Its columns divide every batch, at any width.
	for (const width of [600, 900, 1100, 1600]) {
		await page.setViewportSize({ width, height: 900 });
		await page.goto('/files');
		expect([2, 3, 4, 6, 8]).toContain(await cols());
	}
	// A width at which a free grid fitted 7 columns.
	await page.setViewportSize({ width: 1360, height: 900 });
	await page.goto('/files');
	const n = await cols();
	expect([2, 3, 4, 6, 8]).toContain(n);
	await expect(page.locator('.a-pager')).toHaveCount(0);
	const cards = page.locator('.a-filecard');
	const first = await cards.count();
	expect([24, 48, 96]).toContain(first);
	// Every row of the batch is full: the last card ends the last column.
	const tops = await cards.evaluateAll((els) => els.map((e) => Math.round(e.getBoundingClientRect().top)));
	const rows = new Map<number, number>();
	for (const t of tops) rows.set(t, (rows.get(t) ?? 0) + 1);
	expect([...rows.values()].every((c) => c === n)).toBe(true);
	// Scrolling to the end loads the next batch into the same grid.
	const pages: string[] = [];
	page.on('request', (r) => { if (new URL(r.url()).pathname.startsWith('/api/page/')) pages.push(r.url()); });
	await page.mouse.wheel(0, 100000);
	await expect.poll(() => cards.count()).toBeGreaterThan(first);
	expect(new URL(pages[0]!).searchParams.get('page')).toBe('2');
	await expect(page).toHaveURL(/\/files$/);
});

test('closing a preview, by any path and after its bigger view, leaves the page scrolling for more', async ({ page }) => {
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/users');
	await page.click('nav[aria-label="Main"] a:has-text("Files")');
	await expect(page.getByRole('heading', { name: 'Files', level: 1 })).toBeVisible();
	const cards = page.locator('.a-filecard');
	await expect.poll(() => cards.count()).toBeGreaterThan(0);
	// A stored photo in the first batch: its preview opens a bigger view.
	const rows = (await (await page.request.get('/api/page/files')).json()).props.rows as { name: string; file_type: string; stored: boolean; ext: string }[];
	const photo = rows.find((r) => r.file_type === 'photo' && r.stored && !/^(webm|tgs)$/i.test(r.ext))!;
	expect(photo, 'a stored photo on the first page').toBeTruthy();
	// The page is unlocked and scrolls, and its end loads the next batch.
	const scrollsForMore = async () => {
		expect(await page.evaluate(() => getComputedStyle(document.body).overflow)).not.toBe('hidden');
		const n = await cards.count();
		const y = await page.evaluate(() => scrollY);
		await page.mouse.wheel(0, 100000);
		await expect.poll(() => page.evaluate(() => scrollY)).toBeGreaterThan(y);
		await expect.poll(() => cards.count()).toBeGreaterThan(n);
	};
	const open = async (name = photo.name) => {
		await page.locator('.a-filecard', { has: page.locator('.a-uname', { hasText: name }) }).first().click();
		const d = page.getByRole('dialog', { name });
		await expect(d).toBeVisible();
		expect(await page.evaluate(() => getComputedStyle(document.body).overflow)).toBe('hidden');
		return d;
	};
	const bigger = async (d: ReturnType<typeof page.getByRole>, close: 'button' | 'esc' | 'backdrop') => {
		await d.getByRole('button', { name: 'View bigger' }).click();
		const viewer = page.getByRole('dialog', { name: 'Media viewer' });
		await expect(viewer).toBeVisible();
		if (close === 'button') await viewer.getByRole('button', { name: 'Close' }).click();
		else if (close === 'esc') await page.keyboard.press('Escape');
		else await viewer.click({ position: { x: 5, y: 300 } });
		await expect(viewer).toHaveCount(0);
		await expect(d).toBeVisible();
	};
	const backdrop = () => page.locator('.a-modal-wrap').click({ position: { x: 5, y: 300 } });
	const paths: [string, (d: ReturnType<typeof page.getByRole>) => Promise<void>][] = [
		['close button', (d) => d.getByRole('button', { name: 'Close' }).click()],
		['Esc', () => page.keyboard.press('Escape')],
		['backdrop', backdrop],
	];
	for (const viaBigger of ['', 'button', 'esc', 'backdrop'] as const) {
		for (const [label, close] of paths) {
			await test.step(`${label}${viaBigger ? ` after the bigger view closed by ${viaBigger}` : ''}`, async () => {
				await page.evaluate(() => scrollTo(0, 0));
				const d = await open();
				if (viaBigger) await bigger(d, viaBigger);
				await close(d);
				await expect(d).toHaveCount(0);
				await scrollsForMore();
			});
		}
	}
	// Leaving the page with the preview (and its bigger view) open.
	await page.evaluate(() => scrollTo(0, 0));
	const d = await open();
	await d.getByRole('button', { name: 'View bigger' }).click();
	await page.goBack();
	await expect(page.getByRole('heading', { name: 'Users', level: 1 })).toBeVisible();
	expect(await page.evaluate(() => getComputedStyle(document.body).overflow)).not.toBe('hidden');
});
