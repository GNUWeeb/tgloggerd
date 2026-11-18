// @vitest-environment happy-dom
// The top bar's theme and sidebar buttons keep their choice: as the
// account's preference when signed in, the theme in the browser when
// signed out.
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import type { GroupsProps } from '../routes/groups.tsx';

beforeEach(setup);
afterEach(() => { delete document.documentElement.dataset.theme; try { localStorage.clear(); } catch { /* none */ } });
const patches = (calls: ReturnType<typeof fakeApi>) => callsTo(calls, 'PATCH', '/api/account/preferences');
const ok = { 'PATCH /api/account/preferences': (c: { body: unknown }) => ({ prefs: c.body }) };

describe('kept preferences', () => {
	it('saves the theme to the account when signed in', async () => {
		const calls = fakeApi('superadmin', ok);
		renderPage('/');
		fireEvent.click(screen.getByRole('button', { name: 'Theme: system' }));
		expect(document.documentElement.dataset.theme).toBe('light');
		await waitFor(() => expect(patches(calls)).toHaveLength(1));
		expect(patches(calls)[0]!.body).toEqual({ theme: 'light' });
		expect(patches(calls)[0]!.headers.get('x-csrf-token')).toBeTruthy();
		fireEvent.click(screen.getByRole('button', { name: 'Theme: light' }));
		await waitFor(() => expect(patches(calls)).toHaveLength(2));
		expect(patches(calls)[1]!.body).toEqual({ theme: 'dark' });
	});

	it('saves the sidebar state to the account', async () => {
		const calls = fakeApi('superadmin', ok);
		renderPage('/');
		fireEvent.click(screen.getByRole('button', { name: 'Collapse sidebar' }));
		await waitFor(() => expect(patches(calls)).toHaveLength(1));
		expect(patches(calls)[0]!.body).toEqual({ sidebar: 'collapsed' });
		fireEvent.click(screen.getByRole('button', { name: 'Expand sidebar' }));
		await waitFor(() => expect(patches(calls)).toHaveLength(2));
		expect(patches(calls)[1]!.body).toEqual({ sidebar: 'open' });
	});

	it('opens collapsed when the account keeps it so', () => {
		const page = fixture('/');
		renderPage('/', 'superadmin', { viewer: { ...page.viewer, sidebar: 'collapsed' } });
		expect(screen.getByRole('button', { name: 'Expand sidebar' })).toBeTruthy();
	});

	it('keeps the theme in the browser when signed out, and brings it back', async () => {
		const g = fixture<GroupsProps>('/groups').props.rows.find((x) => x.exposed)!;
		const calls = fakeApi('public', ok);
		const first = renderPage(`/groups/${g.id}/chat`, 'public');
		fireEvent.click(screen.getByRole('button', { name: 'Theme: system' }));
		expect(localStorage.getItem('tgd-theme')).toBe('light');
		expect(patches(calls)).toHaveLength(0);
		first.unmount();
		delete document.documentElement.dataset.theme;
		renderPage(`/groups/${g.id}/chat`, 'public');
		await waitFor(() => expect(document.documentElement.dataset.theme).toBe('light'));
		expect(screen.getByRole('button', { name: 'Theme: light' })).toBeTruthy();
	});
});
