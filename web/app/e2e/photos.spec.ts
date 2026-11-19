// A person's photos: one no longer stored, whose fetch fails, shows a
// placeholder in the history and in the viewer, never a broken image.
import { test, expect } from './fixtures.ts';

test('a photo that cannot be loaded shows a placeholder, not a broken image', async ({ page }) => {
	const rows = (await (await page.request.get('/api/page/users')).json()).props.rows as { id: number }[];
	let id = 0;
	for (const r of rows) {
		const h = await (await page.request.get(`/api/users/${r.id}/history?limit=500`)).json();
		if (h.entries?.some((e: { photo?: string }) => e.photo?.includes('standin-gone-'))) { id = r.id; break; }
	}
	expect(id, 'a user with an unavailable photo in the stand-in').toBeTruthy();
	await page.goto(`/users/${id}/history`);
	const timeline = page.locator('.a-timeline');
	await expect(timeline.getByRole('img', { name: 'Photo not available' }).first()).toBeVisible();
	await expect(timeline.locator('.a-photo.loading')).toHaveCount(0);
	// Every picture left on the page has drawn.
	const broken = async (sel: string) => page.locator(sel).evaluateAll((xs) => (xs as HTMLImageElement[]).filter((i) => !i.complete || i.naturalWidth === 0).length);
	expect(await broken('.a-timeline img')).toBe(0);

	await timeline.getByRole('button', { name: /^Show this photo/ }).first().click();
	const box = page.getByRole('dialog', { name: /^Photos of / });
	const strip = box.getByRole('group', { name: 'All photos' });
	await expect(strip.getByRole('img', { name: 'Photo not available' }).first()).toBeVisible();
	await expect(strip.locator('.a-photo.loading')).toHaveCount(0);
	expect(await broken('.a-carousel-strip img')).toBe(0);
	// The failed one on the stage: a note instead of the picture.
	await strip.getByRole('img', { name: 'Photo not available' }).first().click();
	await expect(box.locator('.a-carousel-stage').getByText('Photo not available')).toBeVisible();
	await expect(box.locator('.a-carousel-stage img')).toHaveCount(0);
});
