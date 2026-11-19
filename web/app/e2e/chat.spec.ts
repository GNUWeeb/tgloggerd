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

test('a photo opens full size in the viewer above the chat; Esc closes it', async ({ page }) => {
	await page.goto(`/groups/${G}/chat?at=555422`);
	await expect(page.locator('.a-livestate')).toBeVisible();
	const photo = page.locator('#m555422').getByRole('link', { name: 'Photo' }).first();
	await expect(photo.locator('img')).toHaveJSProperty('complete', true);
	expect(await photo.locator('img').evaluate((i: HTMLImageElement) => i.naturalWidth)).toBeGreaterThan(0);
	await photo.click();
	const viewer = page.getByRole('dialog', { name: 'Media viewer' });
	await expect(viewer.locator('img')).toBeVisible();
	// Above everything: the image's centre is the topmost element there.
	const top = await viewer.locator('img').evaluate((i) => {
		const r = i.getBoundingClientRect();
		return document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2) === i;
	});
	expect(top).toBe(true);
	await page.keyboard.press('Escape');
	await expect(viewer).toHaveCount(0);
});

test('a voice note fetches nothing until it is played', async ({ page }) => {
	const asked: string[] = [];
	page.on('request', (r) => r.url().includes('/files/') && asked.push(r.url()));
	await page.goto(`/groups/${G}/chat?at=555323`);
	await expect(page.locator('.a-livestate')).toBeVisible();
	const voice = page.locator('#m555323 .c-voice');
	await expect(voice.locator('.c-wave span').first()).toBeVisible();
	const src = await voice.locator('audio').getAttribute('src');
	expect(asked.filter((u) => u.endsWith(src!))).toEqual([]);
	await voice.getByRole('button', { name: 'Play' }).click();
	await expect.poll(() => asked.filter((u) => u.endsWith(src!)).length).toBeGreaterThan(0);
});

test('an animated sticker plays once in view, and only its chat loads the player', async ({ page }) => {
	const player: string[] = [];
	// The player itself, not the app's own loader module (lottie.ts).
	page.on('request', (r) => /lottie-web|lottie_light/.test(r.url()) && player.push(r.url()));
	await page.goto('/users');
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	expect(player).toEqual([]);
	await page.goto(`/groups/${G}/chat?at=555533`);
	const sticker = page.locator('#m555533 .c-lottie');
	await expect(sticker.locator('.c-lottie-box svg')).toBeVisible();
	await expect(sticker).toHaveClass(/ready/);
	await expect(sticker.locator('.c-lottie-emoji')).toHaveCount(0);
	expect(player.length).toBeGreaterThan(0);
});

test('an opened edit history sits clear of the edited mark and the time', async ({ page }) => {
	await page.goto(`/groups/${G}/chat`);
	await expect(page.locator('.a-livestate')).toBeVisible();
	const edited = page.locator('button.c-edited').last();
	await edited.scrollIntoViewIfNeeded();
	await edited.click();
	const bubble = edited.locator('xpath=ancestor::*[contains(@class,"c-bubble")][1]');
	const meta = await bubble.locator('.c-meta').boundingBox();
	const box = await bubble.locator('.c-edits').boundingBox();
	expect(box!.y - (meta!.y + meta!.height)).toBeGreaterThanOrEqual(8);
});

test('a chat shows each message kind, rendered by the server and in the browser', async ({ page }) => {
	// Assembly Lounge holds the stand-in's sample of each kind.
	const res = await page.request.get('/groups/-1001775585202/chat');
	const html = await res.text();
	for (const s of ['Which editor do you use?', 'Release checklist', 'Video chat . live now', 'A widget, which this archive does not show']) expect(html).toContain(s);
	await page.goto('/groups/-1001775585202/chat');
	await expect(page.getByText('Which editor do you use?')).toBeVisible();
	await expect(page.locator('.c-videowrap.round')).toHaveCount(1);
	await expect(page.locator('.k-reaction').first()).toBeVisible();
	await expect(page.getByRole('link', { name: 'pinned a message' })).toBeVisible();
});

test('a forum chat shows one topic at a time', async ({ page }) => {
	await page.goto('/groups/-1001126016141/chat');
	const strip = page.getByRole('navigation', { name: 'Forum topics' });
	await expect(strip.getByRole('link', { name: 'All topics' })).toHaveAttribute('aria-current', 'page');
	expect(await page.locator('.k-topictag').count()).toBeGreaterThan(0);
	await strip.getByRole('link', { name: 'Help and questions' }).click();
	await expect(page).toHaveURL(/\?topic=12$/);
	await expect(strip.getByRole('link', { name: 'Help and questions' })).toHaveAttribute('aria-current', 'page');
	await expect(page.locator('.k-topictag')).toHaveCount(0);
	// The window's own API keeps to the topic.
	const w = await (await page.request.get('/api/chats/group/-1001126016141/messages?topic=12&limit=10')).json();
	expect(w.messages.every((m: { topic_id: number }) => m.topic_id === 12)).toBe(true);
	// A private chat has no topics.
	const pm = (await (await page.request.get('/api/page/private_messages')).json()).props.rows[0].chat.id;
	expect((await page.request.get(`/api/chats/private/${pm}/messages`)).status()).toBe(200);
	expect((await page.request.get(`/api/chats/private/${pm}/messages?topic=12`)).status()).toBe(400);
});

test('a chat opened at its oldest messages loads on downwards to the newest', async ({ page, request }) => {
	const G = -1001337794619;
	const newest = (await (await request.get(`/api/chats/group/${G}/messages?limit=1`, { headers: { cookie: 'tgd_as=superadmin' } })).json()).messages[0].message_id as number;
	await page.goto(`/groups/${G}/chat?after=0`);
	const bubbles = page.locator('.c-feed [id^="m"]');
	await expect(bubbles.first()).toBeVisible();
	const first = await bubbles.count();
	const scroller = page.locator('.c-scroll');
	// Scroll to the bottom until the newest message is there; each time
	// the window grows below.
	for (let i = 0; i < 12 && !(await page.locator(`#m${newest}`).count()); i++) {
		await scroller.evaluate((e) => { e.scrollTop = e.scrollHeight; });
		await page.waitForTimeout(250);
	}
	await expect(page.locator(`#m${newest}`)).toBeAttached();
	expect(await bubbles.count()).toBeGreaterThan(first + 60);
	await expect(page.getByText('Loading newer messages...')).toHaveCount(0);
	await expect(page.getByText('Beginning of the archived history')).toBeAttached();
});
