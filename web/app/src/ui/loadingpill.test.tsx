// @vitest-environment happy-dom
// The loading pill: shown once a navigation, or data someone waits for,
// has taken longer than DELAY_MS; never for a fast one.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, screen } from '@testing-library/react';
import { fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import { DELAY_MS } from './LoadingPill.tsx';
import type { GroupsProps } from '../routes/groups.tsx';

beforeEach(setup);
afterEach(() => vi.useRealTimers());

// An answer held until release() is called.
function held<T>(value: () => T) {
	let release!: () => void;
	const p = new Promise<T>((r) => { release = () => r(value()); });
	return { p, release: () => act(async () => { release(); await p; await Promise.resolve(); }) };
}
const pill = () => screen.queryByText('Loading...');
const wait = (ms: number) => act(async () => { vi.advanceTimersByTime(ms); });

describe('loading pill', () => {
	it('shows during a slow navigation, and goes once the page is there', async () => {
		vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
		const h = held(() => json(200, fixture('/groups')));
		fakeApi('superadmin', { 'GET /api/page/groups': () => h.p });
		renderPage('/');
		fireEvent.click(document.querySelector<HTMLAnchorElement>('a[href="/groups"]')!);
		await wait(DELAY_MS - 50);
		expect(pill()).toBeNull();
		await wait(100);
		expect(pill()).toBeTruthy();
		await h.release();
		expect(pill()).toBeNull();
		expect(screen.getByRole('heading', { level: 1 }).textContent).toBe('Groups');
	});

	it('does not show for a navigation faster than that', async () => {
		vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
		const h = held(() => json(200, fixture('/groups')));
		fakeApi('superadmin', { 'GET /api/page/groups': () => h.p });
		renderPage('/');
		fireEvent.click(document.querySelector<HTMLAnchorElement>('a[href="/groups"]')!);
		await wait(DELAY_MS - 100);
		await h.release();
		await wait(DELAY_MS);
		expect(pill()).toBeNull();
	});
});
