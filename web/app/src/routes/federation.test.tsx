// @vitest-environment happy-dom
// A federation's own page: reached from its card, its groups page by
// page and searched, its admins and members, and renaming and
// describing it as the viewer may.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import { federationDetail, federationGroups } from '../../standin/fixtures.ts';
import type { FederationDetailProps } from './federation.tsx';

beforeEach(setup);
const serveGroups = { 'GET /api/federations/:id/groups': (c: { path: string; query: URLSearchParams }) => { const [st, b] = federationGroups(Number(c.path.split('/')[3]), c.query, 'superadmin'); return json(st, b); } };
const dialog = () => screen.getByRole('dialog');

describe('federation page', () => {
	it('opens from its card on the federations page', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/settings/federations');
		fireEvent.click(screen.getByRole('link', { name: 'GNU/Weeb network' }));
		await waitFor(() => expect(callsTo(calls, 'GET', '/api/page/settings/federations/1')).toHaveLength(1));
	});

	it('shows its groups, admins and members', () => {
		const p = fixture<FederationDetailProps>('/settings/federations/1').props;
		renderPage('/settings/federations/1');
		expect(screen.getByRole('heading', { level: 1 }).textContent).toBe('GNU/Weeb network');
		const list = document.querySelector('.a-fedlist') as HTMLElement;
		for (const g of p.groups.items) expect(within(list).getByRole('link', { name: g.title }).getAttribute('href')).toBe(`/groups/${g.id}`);
		for (const a of [...p.admins, ...p.members]) expect(screen.getAllByText(a.name).length).toBeGreaterThan(0);
	});

	it('loads more of its groups, and searches them on the server', async () => {
		const calls = fakeApi('superadmin', serveGroups);
		const p = fixture<FederationDetailProps>('/settings/federations/0').props;
		renderPage('/settings/federations/0');
		const before = document.querySelectorAll('.a-fedlist li').length;
		expect(before).toBe(p.groups.items.length);
		fireEvent.click(screen.getByRole('button', { name: /Load more/ }));
		await waitFor(() => expect(document.querySelectorAll('.a-fedlist li').length).toBeGreaterThan(before));
		expect(callsTo(calls, 'GET', '/api/federations/0/groups')[0]!.query.get('cursor')).toBe(p.groups.next_cursor);
		const term = p.groups.items[0]!.title.slice(0, 5);
		fireEvent.change(screen.getByRole('textbox', { name: /Search this federation/ }), { target: { value: term } });
		await waitFor(() => expect(callsTo(calls, 'GET', '/api/federations/0/groups').some((c) => c.query.get('q') === term)).toBe(true));
		await waitFor(() => expect([...document.querySelectorAll('.a-fedlist li a')].every((a) => a.textContent!.includes(term))).toBe(true));
	});

	it('lets a superadmin describe Default, but not rename it', async () => {
		const calls = fakeApi('superadmin', { 'PATCH /api/federations/:id': () => ({}) });
		renderPage('/settings/federations/0');
		fireEvent.click(screen.getByRole('button', { name: 'Edit' }));
		expect((within(dialog()).getByRole('textbox', { name: /^Name/ }) as HTMLInputElement).disabled).toBe(true);
		expect(within(dialog()).getByText('The Default federation keeps its name.')).toBeTruthy();
		fireEvent.change(within(dialog()).getByRole('textbox', { name: /Description/ }), { target: { value: '  Every group\nnot elsewhere.  ' } });
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save changes' }));
		await waitFor(() => expect(callsTo(calls, 'PATCH', '/api/federations/0')).toHaveLength(1));
		expect(callsTo(calls, 'PATCH', '/api/federations/0')[0]!.body).toEqual({ description: 'Every group\nnot elsewhere.' });
	});

	it('lets an admin of a federation rename it, sending only the name', async () => {
		const id = [1, 2, 3].find((i) => federationDetail(i, 'admin'))!;
		const calls = fakeApi('admin', { 'PATCH /api/federations/:id': () => ({}) });
		renderPage(`/settings/federations/${id}`, 'admin');
		fireEvent.click(screen.getByRole('button', { name: 'Edit' }));
		fireEvent.change(within(dialog()).getByRole('textbox', { name: 'Name' }), { target: { value: 'Renamed by its admin' } });
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save changes' }));
		await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/federations/${id}`)).toHaveLength(1));
		expect(callsTo(calls, 'PATCH', `/api/federations/${id}`)[0]!.body).toEqual({ name: 'Renamed by its admin' });
		expect(screen.queryByText(/from the edit button on its card/)).toBeNull();
	});

	it('refuses a description over 2000 characters before sending it', () => {
		fakeApi('superadmin');
		renderPage('/settings/federations/1');
		fireEvent.click(screen.getByRole('button', { name: 'Edit' }));
		fireEvent.change(within(dialog()).getByRole('textbox', { name: /Description/ }), { target: { value: 'x'.repeat(2001) } });
		expect((within(dialog()).getByRole('button', { name: 'Save changes' }) as HTMLButtonElement).disabled).toBe(true);
		expect(within(dialog()).getByText(/Too long/)).toBeTruthy();
	});

	it('shows the server\'s refusal', async () => {
		fakeApi('superadmin', { 'PATCH /api/federations/:id': () => json(400, { error: 'bad_request', message: 'A description takes no control characters.', field: 'description' }) });
		renderPage('/settings/federations/1');
		fireEvent.click(screen.getByRole('button', { name: 'Edit' }));
		fireEvent.change(within(dialog()).getByRole('textbox', { name: /Description/ }), { target: { value: 'something' } });
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save changes' }));
		expect((await within(dialog()).findByRole('alert')).textContent).toContain('no control characters');
	});

	it('deletes a federation only after confirming, then goes back to the list', async () => {
		const calls = fakeApi('superadmin', { 'DELETE /api/federations/:id': () => undefined });
		renderPage('/settings/federations/2');
		fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
		const d = dialog();
		expect(within(d).getByText(/cannot be undone/)).toBeTruthy();
		expect(within(d).getByText(/returns its \d+ groups to Default/)).toBeTruthy();
		fireEvent.click(within(d).getByRole('button', { name: 'Cancel' }));
		expect(callsTo(calls, 'DELETE', /federations/)).toHaveLength(0);
		fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Delete federation' }));
		await waitFor(() => expect(callsTo(calls, 'DELETE', '/api/federations/2')).toHaveLength(1));
		await waitFor(() => expect(callsTo(calls, 'GET', '/api/page/settings/federations')).toHaveLength(1));
	});

	it('offers no delete for Default, nor to an admin', () => {
		const r = renderPage('/settings/federations/0');
		expect(screen.queryByRole('button', { name: 'Delete' })).toBeNull();
		r.unmount();
		const id = [1, 2, 3].find((i) => federationDetail(i, 'admin'))!;
		renderPage(`/settings/federations/${id}`, 'admin');
		expect(screen.queryByRole('button', { name: 'Delete' })).toBeNull();
	});

	it('does not exist for a member', () => {
		const { page } = renderPage('/settings/federations/1', 'member');
		expect(page.status).toBe(404);
	});
});
