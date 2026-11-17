// The chat view in a real browser: scrolling back and live updates.
import { test, expect } from './fixtures.ts';

const G = -1001337794619;
const bubbles = (page: import('@playwright/test').Page) => page.locator('.c-row, .c-service');

test('scrolling to the top loads older messages without moving the view', async ({ page }) => {
	await page.goto(`/groups/${G}/chat`);
	// Wait for the page to take over (its scroll listener attached).
	await expect(page.locator('.a-livestate')).toBeVisible();
	const before = await bubbles(page).count();
	// Scroll to the top until older messages come: the scroll listener may
	// be attached a moment after the first scroll to the bottom.
	await expect.poll(async () => {
		await page.locator('.c-scroll').evaluate((el) => { el.scrollTop = 0; el.dispatchEvent(new Event('scroll')); });
		return bubbles(page).count();
	}).toBeGreaterThan(before);
	// The view is put back on the next frame, so the reader stays put.
	await expect.poll(() => page.locator('.c-scroll').evaluate((el) => el.scrollTop)).toBeGreaterThan(0);
});

test('new messages follow the live edge; scrolled up, they are counted instead', async ({ page, emit }) => {
	await page.goto(`/groups/${G}/chat`);
	await expect(page.locator('.a-livestate.is-live')).toBeVisible();
	const e1 = await emit({ kind: 'message.new', chat_id: G });
	await expect(page.locator(`#m${e1.message_id}`)).toBeVisible();
	await page.locator('.c-scroll').evaluate((el) => { el.scrollTop = el.scrollHeight - el.clientHeight - 1500; });
	const top = await page.locator('.c-scroll').evaluate((el) => el.scrollTop);
	await emit({ kind: 'message.new', chat_id: G });
	await emit({ kind: 'message.new', chat_id: G });
	await expect(page.locator('.c-unseen')).toHaveText('2');
	expect(await page.locator('.c-scroll').evaluate((el) => el.scrollTop)).toBe(top);
});

test('edits and deletions change the bubble in place', async ({ page, emit }) => {
	await page.goto(`/groups/${G}/chat`);
	await expect(page.locator('.a-livestate.is-live')).toBeVisible();
	const e = await emit({ kind: 'message.new', chat_id: G });
	await expect(page.locator(`#m${e.message_id}`)).toBeVisible();
	await emit({ kind: 'message.edited', chat_id: G, message_id: e.message_id });
	await expect(page.locator(`#m${e.message_id}`)).toContainText('(edited live)');
	await emit({ kind: 'message.deleted', chat_id: G, message_id: e.message_id });
	await expect(page.locator(`#m${e.message_id} .c-bubble.deleted`)).toBeVisible();
});

test('without the stream the chat polls, and goes live again when it returns', async ({ page, emit, standinUrl }) => {
	await page.goto(`/groups/${G}/chat`);
	await expect(page.locator('.a-livestate.is-live')).toBeVisible();
	await fetch(`${standinUrl}/api/_standin/events`, { method: 'POST', body: JSON.stringify({ off: true }) });
	await expect(page.locator('.a-livestate.is-polling')).toBeVisible();
	const e = await emit({ kind: 'message.new', chat_id: G });
	await expect(page.locator(`#m${e.message_id}`)).toBeAttached();
	await fetch(`${standinUrl}/api/_standin/events`, { method: 'POST', body: JSON.stringify({ off: false }) });
	await expect(page.locator('.a-livestate.is-live')).toBeVisible({ timeout: 15_000 });
});
