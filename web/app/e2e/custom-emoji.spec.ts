// Custom emoji in a real browser: the stickers draw, and a chat full
// of animated ones stays smooth, playing only those in view.
import { test, expect } from './fixtures.ts';
import { FORUM_GROUP, KINDS_GROUP } from '../standin/kinds.ts';

test('a message\'s custom emoji, reaction and effect draw as their stickers', async ({ page }) => {
	await page.goto(`/groups/${KINDS_GROUP.id}/chat`);
	await expect(page.locator('.a-livestate')).toBeVisible();
	const row = page.locator('.c-row').filter({ hasText: 'Custom emoji:' });
	await row.scrollIntoViewIfNeeded();
	const text = row.locator('.c-text');
	await expect(text.locator('.ce-tgs.ready svg')).toBeVisible();
	await expect(text.locator('.ce-tgs .ce-alt')).toHaveCount(0);
	const img = text.getByRole('img', { name: '🎉' });
	await expect.poll(() => img.evaluate((el) => (el as HTMLImageElement).naturalWidth)).toBeGreaterThan(0);
	await expect(row.locator('.k-reaction .ce-tgs.ready svg')).toBeVisible();
	await expect(row.locator('.c-meta .k-effect img')).toBeVisible();
});

test('a forum chat full of animated topic icons plays only those in view, smoothly', async ({ page }) => {
	await page.goto(`/groups/${FORUM_GROUP.id}/chat`);
	await expect(page.locator('.a-livestate')).toBeVisible();
	const icons = page.locator('.k-topictag .ce-tgs');
	await expect.poll(() => icons.count()).toBeGreaterThan(10);
	// The bottom of the chat holds none; jump to the middle of them.
	const n = await icons.count();
	const mid = icons.nth(Math.floor(n / 2));
	await mid.scrollIntoViewIfNeeded();
	await expect(mid).toHaveClass(/ready/);
	await expect(mid.locator('svg')).toBeVisible();
	// Those far from the view have not been loaded.
	const states = await icons.evaluateAll((els) => els.map((el) => {
		const r = el.getBoundingClientRect();
		return { ready: el.classList.contains('ready'), far: r.bottom < -1000 || r.top > innerHeight + 1000 };
	}));
	expect(states.filter((s) => s.far).length).toBeGreaterThan(0);
	expect(states.filter((s) => s.far && s.ready)).toEqual([]);
	// Frames drawn in two seconds while the ones in view play.
	const fps = await page.evaluate(() => new Promise<number>((done) => {
		let n = 0;
		const t0 = performance.now();
		const tick = (t: number) => { n++; if (t - t0 < 2000) requestAnimationFrame(tick); else done((n * 1000) / (t - t0)); };
		requestAnimationFrame(tick);
	}));
	test.info().annotations.push({ type: 'fps', description: fps.toFixed(1) });
	expect(fps).toBeGreaterThan(20);
});
