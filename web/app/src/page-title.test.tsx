// @vitest-environment happy-dom
// The browser's title and the head's meta follow the page shown, when
// the app moves to another page without a reload and when it goes back.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from './test/page.tsx';
import { routeOf } from './App.tsx';

beforeEach(() => setup());

const headOf = (url: string) => {
	const page = fixture(url);
	return routeOf(page).head(page);
};
const meta = (name: string) => document.head.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)?.content;

describe('page title', () => {
	it('follows an in-app link, and going back', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/settings/federations');
		const list = headOf('/settings/federations').title, one = headOf('/settings/federations/1').title;
		expect(list).not.toBe(one);
		await waitFor(() => expect(document.title).toBe(list));
		fireEvent.click(screen.getByRole('link', { name: 'GNU/Weeb network' }));
		await waitFor(() => expect(callsTo(calls, 'GET', '/api/page/settings/federations/1')).toHaveLength(1));
		await waitFor(() => expect(document.title).toBe(one));
		history.back();
		await waitFor(() => expect(document.title).toBe(list));
	});

	it('keeps the description and robots meta to the page shown', async () => {
		fakeApi('superadmin');
		renderPage('/settings/federations');
		const h = headOf('/settings/federations');
		await waitFor(() => expect(document.title).toBe(h.title));
		expect(meta('description')).toBe(h.description);
		expect(meta('robots')).toBe(h.robots);
	});
});
