// Documents previewed by their extension, on the stand-in's samples: a
// PNG sent as a file opens in the photo viewer, a log and an HTML page
// show as text (the page never runs), and an svg or HTML file opened by
// its own URL runs nothing either (the CSP sandbox).
import { test, expect } from './fixtures.ts';
import type { Page } from '@playwright/test';

type Row = { id: number; name: string; url: string | null; message: { scope: string; id: number } | null };
const fileOf = async (page: Page, name: string) => {
	const rows = (await (await page.request.get(`/api/page/files?view=list&q=${encodeURIComponent(name)}`)).json()).props.rows as Row[];
	const f = rows.find((r) => r.name === name);
	expect(f, `the stand-in's ${name}`).toBeTruthy();
	return f!;
};

test('a PNG document opens in the photo viewer, from the files preview and the message page', async ({ page }) => {
	const f = await fileOf(page, 'screenshot.png');
	await page.goto(`/files?file=${f.id}`);
	const dialog = page.getByRole('dialog', { name: 'screenshot.png' });
	await dialog.getByRole('button', { name: 'View bigger' }).click();
	const viewer = page.getByRole('dialog', { name: 'Media viewer' });
	await expect(viewer.locator('img')).toHaveAttribute('src', f.url!);
	await expect.poll(() => viewer.locator('img').evaluate((i: HTMLImageElement) => i.complete && i.naturalWidth)).toBeGreaterThan(0);
	await page.keyboard.press('Escape');
	await expect(viewer).toHaveCount(0);
	await expect(dialog.getByRole('link', { name: 'Download' })).toHaveAttribute('href', `${f.url}?download=1`);

	await page.goto(`/messages/${f.message!.scope}/${f.message!.id}`);
	await page.locator('.a-msgmedia').getByRole('link', { name: 'Photo' }).click();
	await expect(viewer.locator('img')).toHaveAttribute('src', f.url!);
});

test('a log shows its text in the chat on demand, and an HTML page its source', async ({ page }) => {
	const f = await fileOf(page, 'build.log');
	await page.goto(`/messages/${f.message!.scope}/${f.message!.id}`);
	const box = page.locator('.a-msgmedia');
	await box.getByRole('button', { name: 'Show text' }).click();
	await expect(box.locator('pre')).toContainText('cc main.o util.o -o app');

	const h = await fileOf(page, 'page.html');
	await page.goto(`/files?file=${h.id}`);
	const dialog = page.getByRole('dialog', { name: 'page.html' });
	await expect(dialog.locator('pre')).toContainText('<script>document.title = "doc ran"</script>');
	await expect(dialog.locator('script, iframe, object, embed')).toHaveCount(0);
	expect(await page.title()).not.toBe('doc ran');
});

test('a big log is cut at 512 KiB with a note', async ({ page }) => {
	const f = await fileOf(page, 'trace-big.log');
	await page.goto(`/files?file=${f.id}`);
	const dialog = page.getByRole('dialog', { name: 'trace-big.log' });
	await expect(dialog.getByText(/Showing the first 512 KiB/)).toBeVisible();
	expect(await dialog.locator('pre').evaluate((p) => p.textContent!.length)).toBe(512 * 1024);
});

test('an svg or HTML document opened by its URL runs no script', async ({ page }) => {
	for (const name of ['diagram.svg', 'page.html']) {
		const f = await fileOf(page, name);
		const r = await page.goto(f.url!);
		expect(r!.headers()['content-security-policy']).toContain('sandbox');
		expect(r!.headers()['x-content-type-options']).toBe('nosniff');
		expect(await page.title()).not.toMatch(/ran/);
		const dl = await page.request.get(`${f.url}?download=1`);
		expect(dl.headers()['content-disposition']).toMatch(/^attachment/);
	}
});
