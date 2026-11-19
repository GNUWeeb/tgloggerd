// The search engine's options on the message, users and groups lists,
// with the stand-in's engine switched off, on and behind.
import { test, expect } from './fixtures.ts';

const engine = (request: import('@playwright/test').APIRequestContext, state: string) => request.post('/api/_standin/engine', { data: { state } });

test('without an engine, the message search offers nothing of it', async ({ page }) => {
	await page.goto('/group_messages?q=kernel');
	await expect(page.locator('.a-table tbody tr').first()).toBeVisible();
	await expect(page.getByRole('group', { name: 'Search options' })).toHaveCount(0);
	expect((await (await page.request.get('/api/search/capabilities')).json())).toEqual({ engine: null, entities: {} });
});

test('with the engine, the options highlight matches and list the chats with most hits', async ({ page, request }) => {
	await engine(request, 'available');
	const caps = await (await page.request.get('/api/search/capabilities')).json();
	expect(caps.engine).toBe('opensearch');
	expect(caps.entities.group_messages.available).toBe(true);
	await page.goto('/group_messages?q=kernel');
	const opts = page.getByRole('group', { name: 'Search options' });
	await opts.getByText('Highlight matches').click();
	await expect(page).toHaveURL(/highlight=1/);
	await expect(page.locator('.a-tablecard tbody mark').first()).toHaveText(/kernel/i);
	await opts.getByText('Chats with the most hits').click();
	await expect(page).toHaveURL(/facets=1/);
	const facet = page.locator('.a-facets .a-facet').first();
	await expect(facet).toBeVisible();
	await facet.click();
	await expect(page).toHaveURL(/chat_id/);
	// Both options survive a reload: they are the page's parameters.
	await page.reload();
	await expect(page.locator('.a-tablecard tbody mark').first()).toBeVisible();
});

test('while the engine is behind, the database answers and the page says so', async ({ page, request }) => {
	await engine(request, 'behind');
	await page.goto('/group_messages?q=kernel&highlight=1');
	await expect(page.getByRole('group', { name: 'Search options' })).toHaveCount(0);
	await expect(page.getByRole('status').filter({ hasText: 'still catching up' })).toContainText('Highlighted matches are not available for this search.');
	await expect(page.locator('.a-tablecard tbody mark.a-hl')).toHaveCount(0);
});

test('with the engine, a bio matches search highlights the bios, without facets', async ({ page, request }) => {
	await engine(request, 'available');
	const caps = await (await page.request.get('/api/search/capabilities')).json();
	expect(caps.entities.users.fields).toEqual(['bio']);
	expect(caps.entities.groups.options).not.toContain('facets');
	await page.goto('/users?f=' + encodeURIComponent(JSON.stringify([{ field: 'bio', op: 'matches', value: 'kernel' }])));
	const opts = page.getByRole('group', { name: 'Search options' });
	await expect(opts.getByText('Chats with the most hits')).toHaveCount(0);
	await opts.getByText('Highlight matches').click();
	await expect(page).toHaveURL(/highlight=1/);
	await expect(page.locator('.a-tablecard tbody mark').first()).toHaveText(/kernel/i);
});
