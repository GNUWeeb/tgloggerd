// A group's Telegram members: the count in the chat's head, the list
// on the group page's Members tab (asked for there only, never by the
// chat), and a superadmin hiding a channel's list.
import { test, expect } from './fixtures.ts';
import { MEMBER_SAMPLES as M, membersApi } from '../standin/members.ts';

test('a group chat asks for no member list, and the Members tab pages them', async ({ page }) => {
	const id = M.LONG;
	const asked: string[] = [];
	page.on('request', (r) => { if (r.url().includes(`/api/groups/${id}/members`)) asked.push(r.url()); });
	await page.goto(`/groups/${id}/chat`);
	await expect(page.locator('.c-head .c-sub')).toHaveText(/^supergroup \. [\d,]+ members \. [\d,]+ members seen \. [\d,]+ messages$/);
	// The prefetch went out at once after the page drew; give it time.
	await page.waitForTimeout(1000);
	expect(asked).toEqual([]);
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

test('the Members tab searches by name, @username and id', async ({ page }) => {
	const id = M.LONG;
	type Member = { user: { id: number; name: string; username: string } };
	const all = (membersApi(String(id), new URLSearchParams('limit=200'), 'superadmin', () => true)[1] as { members: Member[] }).members;
	const u = all.find((m) => m.user.username && m.user.name)!.user;
	const unstored = all.at(-1)!.user;
	await page.goto(`/groups/${id}/members`);
	const rows = page.locator('.a-table tbody tr');
	await expect(rows).toHaveCount(50);
	const box = page.getByRole('searchbox', { name: 'Search members by name, @username or ID' });

	await box.fill(u.name.split(' ')[0]!);
	await expect(page).toHaveURL(/\?q=/);
	await expect(rows.filter({ hasText: u.name }).first()).toBeVisible();

	await box.fill('@' + u.username.toUpperCase());
	await expect(page).toHaveURL(new RegExp(`\\?q=${encodeURIComponent('@' + u.username.toUpperCase())}$`));
	await expect(rows.first()).toContainText(u.name);

	await box.fill(String(unstored.id));
	await expect(rows.first()).toContainText(`User ${unstored.id}`);
	await expect(rows.first()).toContainText('matched by ID');
	// A reload keeps the search.
	await page.reload();
	await expect(box).toHaveValue(String(unstored.id));
	await expect(rows.first()).toContainText('matched by ID');

	await box.fill('zzzqqqxxx');
	await expect(page.locator('.a-empty')).toContainText('No member matches zzzqqqxxx.');
	await box.press('Escape');
	await expect(box).toHaveValue('');
	await expect(page).toHaveURL(new RegExp(`/groups/${id}/members$`));
	await expect(rows).toHaveCount(50);
});
