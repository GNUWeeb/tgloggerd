// @vitest-environment happy-dom
// "Look up real name" on a contact's user page.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import { sampleContacts } from '../../standin/fixtures.ts';
import type { UsersProps } from './users.tsx';

beforeEach(setup);
const [contact, ownUnknown, pending] = sampleContacts() as [number, number, number];
const button = () => screen.queryByRole('button', { name: 'Look up real name' });
const NAME = { user_id: contact, first_name: 'Dewi', last_name: 'Kusuma', contact_first_name: 'Dewi', contact_last_name: '(kernel)', checked_at: 1763450000 };

describe('look up real name', () => {
	it('is offered to a superadmin on a contact\'s page only', () => {
		const r = renderPage(`/users/${contact}`);
		expect(button()).toBeTruthy();
		expect(screen.getByText('mutual contact')).toBeTruthy();
		r.unmount();
		const other = fixture<UsersProps>('/users').props.rows.find((u) => !sampleContacts().includes(u.id))!;
		const r2 = renderPage(`/users/${other.id}`);
		expect(button()).toBeNull();
		r2.unmount();
		renderPage(`/users/${contact}`, 'admin');
		expect(button()).toBeNull();
	});

	it('asks first, says it is waiting, then shows the own name and refreshes the page', async () => {
		let answer: (v: unknown) => void = () => {};
		const calls = fakeApi('superadmin', { 'POST /api/users/:id/lookup-name': () => new Promise((r) => { answer = r; }) });
		renderPage(`/users/${contact}`);
		fireEvent.click(button()!);
		const d = screen.getByRole('dialog', { name: 'Look up real name' });
		expect(within(d).getByText(/adds the contact back exactly as it was/)).toBeTruthy();
		expect(within(d).getByText(/custom photo set for the contact, are not restored/)).toBeTruthy();
		expect(callsTo(calls, 'POST', /lookup-name/)).toHaveLength(0);
		fireEvent.click(within(d).getByRole('button', { name: 'Look up' }));
		await within(d).findByText(/can take up to two minutes/);
		const post = callsTo(calls, 'POST', `/api/users/${contact}/lookup-name`);
		expect(post).toHaveLength(1);
		expect(post[0]!.headers.get('x-csrf-token')).toBeTruthy();
		answer(NAME);
		await within(d).findByText('Dewi Kusuma');
		await waitFor(() => expect(callsTo(calls, 'GET', `/api/page/users/${contact}`)).toHaveLength(1));
	});

	it('shows the refusal with when to try again', async () => {
		fakeApi('superadmin', { 'POST /api/users/:id/lookup-name': () => new Response(JSON.stringify({ error: 'rate_limited', message: 'Another lookup ran a moment ago' }), { status: 429, headers: { 'content-type': 'application/json', 'retry-after': '240' } }) });
		renderPage(`/users/${contact}`);
		fireEvent.click(button()!);
		fireEvent.click(screen.getByRole('button', { name: 'Look up' }));
		expect((await screen.findByRole('alert')).textContent).toBe('Another lookup ran a moment ago. Try again in 4 minutes.');
	});

	it('says when the server cannot be reached', async () => {
		fakeApi('superadmin', { 'POST /api/users/:id/lookup-name': () => json(502, { error: 'daemon', message: 'The daemon did not answer.' }) });
		renderPage(`/users/${contact}`);
		fireEvent.click(button()!);
		fireEvent.click(screen.getByRole('button', { name: 'Look up' }));
		expect((await screen.findByRole('alert')).textContent).toBe('The daemon did not answer.');
	});

	it('says when a lookup left the contact removed, and when no own name is known', () => {
		const r = renderPage(`/users/${pending}`);
		expect(screen.getByText(/removed this contact and has not added it back yet/)).toBeTruthy();
		r.unmount();
		renderPage(`/users/${ownUnknown}`);
		expect(screen.getByText(/no name they set has been seen yet/)).toBeTruthy();
	});
});
