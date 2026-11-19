// A group's Files and Polls tabs, in the browser.
import type { Page } from '@playwright/test';
import { test, expect } from './fixtures.ts';

// The kinds sample group: every kind of file, more than a page, polls.
async function sampleGroup(page: Page): Promise<number> {
	const rows = (await (await page.request.get('/api/page/groups?q=Assembly')).json()).props.rows as { id: number; title: string }[];
	const g = rows.find((r) => r.title === 'Assembly Lounge');
	expect(g, 'the kinds sample group in the stand-in').toBeTruthy();
	return g!.id;
}

test('the Files tab lists, pages, previews and filters a group\'s files', async ({ page }) => {
	const id = await sampleGroup(page);
	await page.goto(`/groups/${id}`);
	await page.locator('.a-tabs').getByRole('link', { name: /^Files/ }).click();
	await expect(page).toHaveURL(new RegExp(`/groups/${id}/files$`));
	const kinds = page.getByRole('navigation', { name: 'File kinds' });
	await expect(kinds.getByRole('link', { name: /^Video notes/ })).toBeVisible();
	const cards = page.locator('.a-groupfile');
	const first = await cards.count();
	expect(first).toBeGreaterThanOrEqual(50);
	// Load more appends the next page in place.
	await page.getByRole('button', { name: 'Load more' }).click();
	await expect.poll(() => cards.count()).toBeGreaterThan(first);
	// A card opens its preview in place.
	const name = (await cards.first().locator('.a-filecard .a-uname').textContent())!;
	await cards.first().locator('.a-filecard').click();
	const dialog = page.getByRole('dialog', { name });
	await expect(dialog).toBeVisible();
	await expect(dialog.getByRole('link', { name: 'In the chat' })).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(dialog).toHaveCount(0);
	// A sub tab shows one kind.
	await kinds.getByRole('link', { name: /^Stickers/ }).click();
	await expect(page).toHaveURL(new RegExp(`/groups/${id}/files\\?kind=sticker$`));
	await expect(kinds.getByRole('link', { name: /^Stickers/ })).toHaveAttribute('aria-current', 'page');
	await expect(cards.first()).toBeVisible();
	// The chat link opens the chat at the file's message.
	await cards.first().getByRole('link', { name: 'In the chat' }).click();
	await expect(page).toHaveURL(new RegExp(`/groups/${id}/chat\\?at=\\d+$`));
});

test('the Polls tab shows a group\'s polls with links to their messages', async ({ page }) => {
	const id = await sampleGroup(page);
	await page.goto(`/groups/${id}`);
	await page.locator('.a-tabs').getByRole('link', { name: /^Polls/ }).click();
	await expect(page).toHaveURL(new RegExp(`/groups/${id}/polls$`));
	const cards = page.locator('.a-pollcard');
	await expect(cards).toHaveCount(3);
	await expect(cards.filter({ hasText: 'Who wrote the first Linux kernel?' }).getByText(/Quiz . closed/)).toBeVisible();
	await cards.first().getByRole('link', { name: 'Message' }).click();
	await expect(page).toHaveURL(/\/messages\/group\/\d+$/);
	await expect(page.locator('.k-poll')).toBeVisible();
});

test('signed out, an exposed group\'s Files and Polls tabs are public', async ({ page, as }) => {
	const rows = (await (await page.request.get('/api/page/groups?q=CTF')).json()).props.rows as { id: number; title: string; exposed?: boolean }[];
	const g = rows.find((r) => r.title === 'CTF Practice' && r.exposed);
	expect(g, 'an exposed sample group with polls in the stand-in').toBeTruthy();
	await as('public');
	await page.goto(`/groups/${g!.id}`);
	await expect(page.locator('.c-publicpage')).toBeVisible();
	await page.locator('.a-tabs').getByRole('link', { name: /^Files/ }).click();
	await expect(page).toHaveURL(new RegExp(`/groups/${g!.id}/files$`));
	const cards = page.locator('.a-groupfile');
	const first = await cards.count();
	expect(first).toBeGreaterThan(0);
	await expect(cards.first().getByRole('link', { name: 'In the chat' })).toBeVisible();
	await expect(page.locator('a[href^="/messages/"], a[href^="/users/"]')).toHaveCount(0);
	await page.getByRole('button', { name: 'Load more' }).click();
	await expect.poll(() => cards.count()).toBeGreaterThan(first);
	await page.locator('.a-tabs').getByRole('link', { name: /^Polls/ }).click();
	await expect(page.locator('.a-pollcard')).toHaveCount(2);
	await expect(page.locator('.c-publicpage')).toBeVisible();
	await page.locator('.a-pollcard').first().getByRole('link', { name: 'In the chat' }).click();
	await expect(page).toHaveURL(new RegExp(`/groups/${g!.id}/chat\\?at=\\d+$`));
});
