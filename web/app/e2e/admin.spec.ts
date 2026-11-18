// Administration flows that need the real stand-in behind them.
import { test, expect } from './fixtures.ts';

test('an admin creates a member, granted a federation', async ({ page, as }) => {
	await as('admin');
	await page.goto('/settings/accounts');
	await page.click('button:has-text("New member")');
	const d = page.getByRole('dialog');
	const fields = d.locator('input:not([type=checkbox]):not([type=radio])');
	await fields.nth(0).fill('e2emember');
	await fields.nth(1).fill('E2E member');
	await d.locator('input[type=email]').fill('e2e@example.org');
	await expect(d.getByRole('button', { name: 'Create account' })).toBeDisabled();
	await d.locator('.a-fedgrant input.a-switch').first().check();
	await d.getByRole('button', { name: 'Create account' }).click();
	// The temporary password, shown once.
	await expect(page.getByLabel('Temporary password')).toHaveText(/^[A-Za-z0-9_-]{12,}$/);
	await page.getByRole('button', { name: 'Done' }).click();
	await expect(page.getByLabel('Temporary password')).toHaveCount(0);
	await expect(page.locator('tr:has-text("e2e@example.org")')).toContainText('invited');
});

test('a webhook is added for a chat found by search, and a bad URL is refused', async ({ page }) => {
	await page.goto('/platform-fwd/telegram-discord');
	const rows = await page.locator('tbody tr').count();
	await page.click('button:has-text("Add webhook")');
	const d = page.getByRole('dialog');
	await d.getByRole('button', { name: 'Choose a chat' }).click();
	await d.locator('.a-picker-search input').fill('rust');
	await d.locator('.a-picker-pop').getByRole('button', { name: /Rust Programming/ }).click();
	await d.locator('input[type=password]').fill('https://evil.example/hook');
	await d.getByRole('button', { name: 'Save' }).click();
	await expect(d.getByRole('alert')).toHaveText('That is not a Discord webhook URL.');
	await d.locator('input[type=password]').fill('https://discord.com/api/webhooks/1/abc');
	await d.getByRole('button', { name: 'Save' }).click();
	await expect(page.locator('tbody tr')).toHaveCount(rows + 1);
});

test('a group is pinned to the sidebar from its page', async ({ page }) => {
	await page.goto('/groups/-1001236918171');
	const pinned = page.locator('.a-navgroup:has(.a-navlabel:has-text("Pinned")) .a-navitem');
	const n = await pinned.count();
	await page.click('.a-head-actions button:has-text("Pin")');
	await expect(pinned).toHaveCount(n + 1);
	await page.click('.a-head-actions button:has-text("Unpin")');
	await expect(pinned).toHaveCount(n);
});
