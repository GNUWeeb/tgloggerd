// A forwarding route's content types against the real stand-in: a
// group made mixed by hand, saved in option order, and summed up in the
// route list.
import { test, expect } from './fixtures.ts';

test('a route\'s types are picked by group and summed up in the list', async ({ page }) => {
	await page.goto('/platform-fwd/telegram-discord');
	const cells = page.locator('tbody tr .a-wrapcell > span');
	const i = (await cells.allTextContents()).indexOf('All content');
	expect(i).toBeGreaterThanOrEqual(0);
	const row = page.locator('tbody tr').nth(i);
	await row.getByRole('button', { name: 'Edit' }).click();
	const d = page.getByRole('dialog');
	await expect(d.getByRole('radio', { checked: true })).toHaveText('Everything');
	await d.getByRole('button', { name: 'Show Media' }).click();
	// The chip's box is hidden; its label takes the click.
	await d.locator('.a-checkchip', { hasText: 'GIFs' }).click();
	await expect(d.getByRole('checkbox', { name: 'GIFs' })).not.toBeChecked();
	const media = d.getByRole('checkbox', { name: 'Media' });
	await expect(media).toHaveAttribute('aria-checked', 'mixed');
	expect(await media.evaluate((x) => (x as HTMLInputElement).indeterminate)).toBe(true);
	await expect(d.getByRole('radio', { checked: true })).toHaveText('Custom');
	await d.getByRole('checkbox', { name: 'Service messages' }).uncheck();
	await d.getByRole('button', { name: 'Save' }).click();
	await expect(d).toHaveCount(0);
	const cell = cells.nth(i);
	await expect(cell).toHaveText('All except GIFs, Service messages');
	await expect(cell).toHaveAttribute('title', /^Text, Photos, Videos, Video messages,/);
});
