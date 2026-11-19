// "Fetch from Telegram": a big unstored file has only its fetch path;
// the button fetches it (the stand-in streams it: state partial) and
// the file then opens by the URL answered.
import { test, expect } from './fixtures.ts';

const NOT_STORED = encodeURIComponent('[{"field":"stored","op":"is","value":"no"}]');

test('a big unstored file is fetched from its list row and then downloads from its preview', async ({ page }) => {
	await page.goto(`/files?view=list&per=100&f=${NOT_STORED}`);
	const rows = page.locator('.a-table tbody tr', { has: page.getByRole('button', { name: 'Fetch from Telegram' }) });
	await expect(rows.first()).toBeVisible();
	// One file in five is gone from Telegram in the stand-in.
	const ids = (await rows.locator('td:first-child').allTextContents()).map(Number);
	const i = ids.findIndex((id) => id % 5 !== 0);
	expect(i).toBeGreaterThanOrEqual(0);
	const row = rows.nth(i);
	const name = (await row.locator('.a-uname').textContent())!;
	const posts: string[] = [];
	page.on('request', (r) => { if (r.method() === 'POST' && /\/api\/files\/[^/]+\/fetch$/.test(new URL(r.url()).pathname)) posts.push(r.url()); });
	await row.getByRole('button', { name: 'Fetch from Telegram' }).click();
	const done = page.locator('.a-table tbody tr', { hasText: name }).first();
	await expect(done.getByText('fetchable')).toBeVisible();
	expect(posts).toHaveLength(1);
	await done.locator('.a-usercell').click();
	const dialog = page.getByRole('dialog', { name });
	await expect(dialog.getByRole('link', { name: 'Download' })).toHaveAttribute('href', /^\/files\/standin-\d+/);
	expect(posts).toHaveLength(1);
});

test('a file gone from Telegram says so and keeps its button disabled', async ({ page }) => {
	await page.goto(`/files?view=list&per=100&f=${NOT_STORED}`);
	const rows = page.locator('.a-table tbody tr', { has: page.getByRole('button', { name: 'Fetch from Telegram' }) });
	await expect(rows.first()).toBeVisible();
	const ids = (await rows.locator('td:first-child').allTextContents()).map(Number);
	const i = ids.findIndex((id) => id % 5 === 0);
	expect(i).toBeGreaterThanOrEqual(0);
	const row = rows.nth(i);
	await row.getByRole('button', { name: 'Fetch from Telegram' }).click();
	await expect(row.getByRole('alert')).toHaveText('Telegram no longer has this file.');
	await expect(row.getByRole('button', { name: 'Fetch from Telegram' })).toBeDisabled();
});
