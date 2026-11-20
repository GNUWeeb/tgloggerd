// Signing in and out through real redirects and cookies, and the pages
// email links lead to.
import { test, expect } from './fixtures.ts';

test('a signed-out visit goes to sign-in, then back to the page asked for; signing out ends there', async ({ page, as }) => {
	await as('public');
	await page.goto('/users');
	await expect(page).toHaveURL(/\/login\?next=%2Fusers$/);
	await page.fill('input[name=username]', 'gw_admin');
	await page.fill('input[name=password]', 'wrong');
	await page.click('button:has-text("Sign in")');
	await expect(page.getByRole('alert')).toHaveText('Wrong username or password.');
	await page.fill('input[name=password]', 'standin');
	await page.click('button:has-text("Sign in")');
	await expect(page).toHaveURL(/\/users$/);
	await expect(page.locator('.a-me-name')).toContainText('gw_admin');
	await page.click('button[aria-label="Sign out"]');
	await expect(page).toHaveURL(/\/login$/);
});

test('a set-password link takes the token out of the address bar and sets the password', async ({ page, as }) => {
	await as('public');
	await page.goto('/password/set#token=standin-token');
	await expect(page.getByRole('heading', { name: 'Choose a password' })).toBeVisible();
	await expect.poll(() => new URL(page.url()).hash).toBe('');
	await page.fill('input[name=new_password]', 'a long passphrase');
	await page.fill('input[name=confirm_password]', 'a long passphrase');
	await page.click('button:has-text("Set password")');
	await expect(page.getByRole('status')).toHaveText('Your password is set.');
	await page.click('a:has-text("Sign in")');
	await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
});

test('an account without an email address is never asked for one', async ({ page, as }) => {
	await as('noemail');
	await page.goto('/users');
	await expect(page.getByRole('heading', { name: 'Users', level: 1 })).toBeVisible();
});

test('signing in with a temporary password asks for an own one first, on the page asked for', async ({ page, as }) => {
	await as('public');
	await page.goto('/groups');
	await page.fill('input[name=username]', 'mod_kernel');
	await page.fill('input[name=password]', 'standin');
	await page.click('button:has-text("Sign in")');
	await expect(page.getByRole('heading', { name: 'Choose your own password' })).toBeVisible();
	await expect(page).toHaveURL(/\/groups$/);
	await page.fill('input[name=current_password]', 'standin');
	await page.fill('input[name=new_password]', 'my own passphrase');
	await page.fill('input[name=confirm_password]', 'my own passphrase');
	await page.click('button:has-text("Set my password")');
	await expect(page.getByRole('heading', { name: 'Groups', level: 1 })).toBeVisible();
	await expect(page).toHaveURL(/\/groups$/);
	await page.reload();
	await expect(page.getByRole('heading', { name: 'Groups', level: 1 })).toBeVisible();
});
