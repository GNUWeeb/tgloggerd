// Pages and lists that follow the live stream.
import { test, expect } from './fixtures.ts';

const G = -1001337794619;

test('a user page refreshes in place when the user is renamed', async ({ page, emit, request }) => {
	const uid = (await (await request.get('/api/chats/search?kind=user&limit=1', { headers: { cookie: 'tgd_as=superadmin' } })).json()).items[0].id;
	await page.goto(`/users/${uid}`);
	await expect(page.locator('.a-livestate.is-live')).toBeVisible();
	await page.evaluate(() => { (window as unknown as { __mark: number }).__mark = 1; });
	await emit({ kind: 'user.updated', user_id: uid });
	await expect(page.getByRole('heading', { level: 1 })).toContainText('(renamed');
	expect(await page.evaluate(() => (window as unknown as { __mark?: number }).__mark)).toBe(1);
});

test('a message list keeps its rows and offers a refresh', async ({ page, emit }) => {
	await page.goto('/group_messages');
	await expect(page.locator('.a-livestate.is-live')).toBeVisible();
	const first = await page.locator('tbody tr').first().textContent();
	await emit({ kind: 'message.new', chat_id: G });
	await expect(page.locator('.a-livenotice')).toBeVisible();
	expect(await page.locator('tbody tr').first().textContent()).toBe(first);
	await page.click('.a-livenotice button:has-text("Refresh")');
	await expect(page.locator('.a-livenotice')).toHaveCount(0);
	await expect(page.locator('tbody tr').first()).toContainText('Live message');
});

test('renames and counts change in lists in place, in one batched lookup', async ({ page, emit }) => {
	await page.goto('/users');
	await expect(page.locator('.a-livestate.is-live')).toBeVisible();
	const ids = await page.$$eval('tbody tr a[href^="/users/"]', (as) => [...new Set(as.map((a) => Number(a.getAttribute('href')!.split('/')[2])))].slice(0, 3));
	const lookups: string[] = [];
	page.on('request', (r) => { if (r.url().includes('/api/entities')) lookups.push(r.url()); });
	for (const id of ids) await emit({ kind: 'user.updated', user_id: id });
	const row = (id: number) => page.locator(`tbody tr:has(a[href="/users/${id}"])`).first();
	for (const id of ids) await expect(row(id)).toContainText('(renamed');
	expect(lookups).toHaveLength(1);
	const count = async () => Number((await row(ids[1]!).locator('.num').first().textContent())!.replace(/[^\d]/g, ''));
	const c0 = await count();
	await emit({ kind: 'stats.changed', user_id: ids[1] });
	await expect.poll(count).toBe(c0 + 1);
	const order = await page.$$eval('tbody tr a[href^="/users/"]', (as) => [...new Set(as.map((a) => a.getAttribute('href')))].slice(0, 3));
	expect(order).toEqual(ids.map((id) => `/users/${id}`));
});

test('a chat\'s header count and a sender\'s name follow the stream', async ({ page, emit }) => {
	await page.goto(`/groups/${G}/chat`);
	await expect(page.locator('.a-livestate.is-live')).toBeVisible();
	const sub = page.locator('.c-head .c-sub');
	const before = await sub.textContent();
	await emit({ kind: 'stats.changed', chat_id: G });
	await expect(sub).not.toHaveText(before!);
	const href = await page.locator('.c-sender').first().getAttribute('href');
	await emit({ kind: 'user.updated', user_id: Number(href!.split('/')[2]) });
	await expect(page.locator(`.c-sender[href="${href}"]`).first()).toContainText('(renamed');
});

test('pages with nothing to follow open no stream', async ({ page }) => {
	await page.goto('/settings');
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	await expect(page.locator('.a-livestate')).toHaveCount(0);
});

test('the dashboard counts new messages itself, as the server counts them', async ({ page, emit, request }) => {
	const as = { headers: { cookie: 'tgd_as=superadmin' } };
	const uid = (await (await request.get('/api/chats/search?kind=user&limit=1', as)).json()).items[0].id;
	await page.goto('/');
	await expect(page.locator('.a-livestate.is-live')).toBeVisible();
	const count = (label: string) => page.locator('.a-kpis .a-kpi').filter({ hasText: label }).locator('b');
	const before = { g: await count('Group messages').textContent(), p: await count('Private messages').textContent() };
	await page.evaluate(() => { (window as unknown as { __mark: number }).__mark = 1; });
	const pages: string[] = [];
	page.on('request', (r) => { if (/\/api\/page(\/|$|\?)/.test(new URL(r.url()).pathname)) pages.push(r.url()); });
	await emit({ kind: 'message.new', chat_id: G });
	await emit({ kind: 'message.new', chat_id: G });
	await emit({ kind: 'message.new', chat_id: uid });
	const num = (s: string | null) => Number(s!.replace(/,/g, ''));
	await expect.poll(async () => num(await count('Group messages').textContent())).toBe(num(before.g) + 2);
	await expect.poll(async () => num(await count('Private messages').textContent())).toBe(num(before.p) + 1);
	// What the page ticked up to is what the server now counts.
	const server = (await (await request.get('/api/dashboard/kpis', as)).json()).kpis as { key: string; total: number }[];
	expect(server.find((k) => k.key === 'group_messages')!.total).toBe(num(before.g) + 2);
	expect(server.find((k) => k.key === 'private_messages')!.total).toBe(num(before.p) + 1);
	expect(pages).toEqual([]);
	expect(await page.evaluate(() => (window as unknown as { __mark?: number }).__mark)).toBe(1);
});

test('the dashboard\'s number format is kept for the next visit', async ({ page }) => {
	await page.goto('/');
	const users = page.locator('.a-kpis .a-kpi').filter({ hasText: 'Users' }).locator('b');
	await expect(users).toHaveText(/^\d{1,3}(,\d{3})+$/);
	const saved = page.waitForResponse((r) => r.url().endsWith('/api/account/preferences') && r.request().method() === 'PATCH');
	await page.getByRole('button', { name: 'Compact' }).click();
	await expect(users).toHaveText(/^\d+(\.\d)?[kM]$/);
	expect((await saved).status()).toBe(200);
	await page.reload();
	await expect(users).toHaveText(/^\d+(\.\d)?[kM]$/);
	await expect(page.getByRole('button', { name: 'Compact' })).toHaveAttribute('aria-pressed', 'true');
});
