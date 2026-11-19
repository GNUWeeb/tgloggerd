// A group's Telegram members: the count in the chat's head, the list
// prefetched by the chat and shown on the group page's Members tab, and
// a superadmin hiding a channel's list.
import { test, expect } from './fixtures.ts';
import { MEMBER_SAMPLES as M } from '../standin/members.ts';

test('a group chat prefetches its members, and the Members tab pages them', async ({ page }) => {
	const id = M.LONG;
	const prefetch = page.waitForRequest((r) => r.url().includes(`/api/groups/${id}/members`));
	await page.goto(`/groups/${id}/chat`);
	await prefetch;
	await expect(page.locator('.c-head .c-sub')).toHaveText(/^supergroup \. [\d,]+ members \. [\d,]+ members seen \. [\d,]+ messages$/);
	await page.locator('.c-head a.c-head-link').click();
	await page.getByRole('link', { name: /^Members/ }).click();
	await expect(page).toHaveURL(new RegExp(`/groups/${id}/members$`));
	const rows = page.locator('.a-table tbody tr');
	await expect(rows).toHaveCount(50);
	await expect(rows.first().getByText('owner')).toBeVisible();
	await page.getByRole('button', { name: 'Load more' }).click();
	await expect(rows).toHaveCount(100);
});

test('a superadmin hides a channel\'s list from admins', async ({ page, as }) => {
	const id = M.HIDDEN;
	await page.goto(`/groups/${id}/members`);
	const toggle = page.getByRole('button', { name: 'Hide member list' });
	await expect(toggle).toHaveAttribute('aria-pressed', 'true');
	await toggle.click();
	await expect(toggle).toHaveAttribute('aria-pressed', 'false');
	await as('admin');
	await page.goto(`/groups/${id}/members`);
	await expect(page.locator('.a-table tbody tr').first()).toBeVisible();
	await as('superadmin');
	await page.goto(`/groups/${id}/members`);
	await page.getByRole('button', { name: 'Hide member list' }).click();
	await expect(page.getByRole('button', { name: 'Hide member list' })).toHaveAttribute('aria-pressed', 'true');
	await as('admin');
	await page.goto(`/groups/${id}/members`);
	await expect(page.locator('.a-membersoff')).toContainText('Hidden by a superadmin');
	await expect(page.getByRole('button', { name: 'Hide member list' })).toHaveCount(0);
});
