// @vitest-environment happy-dom
// The dashboard's counts: exact or compact as the account chooses.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import { compact, fmt } from '../format.ts';
import type { DashboardProps } from './dashboard.tsx';

beforeEach(setup);

const kpi = (label: string) => within(within(document.querySelector<HTMLElement>('.a-kpis')!).getByRole('link', { name: new RegExp(`^${label}`) })).getByText((_, el) => el?.tagName === 'B');
const total = (key: string) => fixture<DashboardProps>('/').props.kpis.find((k) => k.key === key)!.total;

describe('dashboard numbers', () => {
	it('shows every digit by default', () => {
		fakeApi('superadmin');
		renderPage('/');
		expect(kpi('Group messages').textContent).toBe(fmt(total('group_messages')));
		expect(fmt(total('group_messages'))).toMatch(/,/);
		expect(screen.getByRole('button', { name: 'Exact' }).getAttribute('aria-pressed')).toBe('true');
	});

	it('renders compact numbers when the account chose them', () => {
		fakeApi('superadmin');
		const page = fixture('/');
		renderPage('/', 'superadmin', { viewer: { ...page.viewer, dashboard_numbers: 'compact' } });
		expect(kpi('Users').textContent).toBe(compact(total('users')));
		expect(kpi('Users').getAttribute('title')).toBe(`${fmt(total('users'))} users`);
	});

	it('switches at once and saves the choice', async () => {
		const calls = fakeApi('superadmin', { 'PATCH /api/account/preferences': () => ({ prefs: { dashboard_numbers: 'compact' } }) });
		renderPage('/');
		fireEvent.click(screen.getByRole('button', { name: 'Compact' }));
		expect(kpi('Users').textContent).toBe(compact(total('users')));
		await waitFor(() => expect(callsTo(calls, 'PATCH', '/api/account/preferences')).toHaveLength(1));
		expect(callsTo(calls, 'PATCH', '/api/account/preferences')[0]!.body).toEqual({ dashboard_numbers: 'compact' });
		expect(screen.queryByText(/Not saved/)).toBeNull();
	});

	it('keeps the choice on this page when saving fails', async () => {
		fakeApi('superadmin', { 'PATCH /api/account/preferences': () => json(500, { error: 'internal' }) });
		renderPage('/');
		fireEvent.click(screen.getByRole('button', { name: 'Compact' }));
		await screen.findByText(/Not saved; shown on this page only/);
		expect(kpi('Users').textContent).toBe(compact(total('users')));
	});

	it('does not save when the choice does not change', () => {
		const calls = fakeApi('superadmin');
		renderPage('/');
		fireEvent.click(screen.getByRole('button', { name: 'Exact' }));
		expect(callsTo(calls, 'PATCH', '/api/account/preferences')).toHaveLength(0);
	});
});
