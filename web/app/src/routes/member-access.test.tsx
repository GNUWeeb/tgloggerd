// @vitest-environment happy-dom
// A member's access on the accounts page: groups granted per federation,
// chosen from that federation's own groups, and its abilities.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup, type Call } from '../test/page.tsx';
import type { AccountsProps } from './accounts.tsx';

beforeEach(setup);

type Body = { grants: { fed: number; group_ids: number[] }[]; abilities?: string[] };
const dialog = () => screen.getByRole('dialog');
const props = (persona: 'superadmin' | 'admin') => fixture<AccountsProps>('/settings/accounts', persona).props;
const researcher = (persona: 'superadmin' | 'admin') => props(persona).accounts.find((a) => a.username === 'researcher')!;
const fedGroups = (call: Call) => ({ items: [{ id: -1009001, title: `Group of ${call.path.split('/')[3]} ${call.query.get('q') ?? ''}`.trim(), username: 'fedgroup', photo: null, msg_count: 10 }], total: 1, next_cursor: null });

function openAccess(persona: 'superadmin' | 'admin', handlers: Record<string, (c: Call) => unknown> = {}) {
	const calls = fakeApi(persona, { 'PATCH /api/accounts/:id': () => ({}), 'GET /api/federations/:id/groups': fedGroups, ...handlers });
	renderPage('/settings/accounts', persona);
	fireEvent.click(screen.getByRole('button', { name: 'Edit researcher' }));
	fireEvent.click(within(dialog()).getByRole('tab', { name: 'Access' }));
	return calls;
}
const section = (fed: string) => [...dialog().querySelectorAll<HTMLElement>('.a-fedgrant')].find((s) => s.textContent!.includes(fed))!;
const save = async (calls: Call[], id: number) => {
	fireEvent.click(within(dialog()).getByRole('button', { name: 'Save changes' }));
	await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/accounts/${id}`)).toHaveLength(1));
	return callsTo(calls, 'PATCH', `/api/accounts/${id}`)[0]!.body as Body;
};

describe('member access', () => {
	it('offers only the federation\'s own groups, searched in that federation', async () => {
		const calls = openAccess('superadmin');
		const fed = props('superadmin').federations.find((f) => f.id === 1)!;
		const box = within(section(fed.name)).getByRole('textbox');
		fireEvent.focus(box);
		fireEvent.change(box, { target: { value: 'ker' } });
		await within(section(fed.name)).findByRole('button', { name: 'Add Group of 1 ker' });
		const asked = callsTo(calls, 'GET', '/api/federations/1/groups').at(-1)!;
		expect(asked.query.get('q')).toBe('ker');
		// Never the archive-wide chat search.
		expect(callsTo(calls, 'GET', '/api/chats/search')).toHaveLength(0);
		fireEvent.click(within(section(fed.name)).getByRole('button', { name: 'Add Group of 1 ker' }));
		const r = researcher('superadmin');
		const body = await save(calls, r.id);
		const was = r.grants.find((g) => g.fed === 1)!.groups.map((g) => g.id);
		expect(body.grants.find((g) => g.fed === 1)!.group_ids).toEqual([...was, -1009001]);
		expect(body.grants.find((g) => g.fed === 2)!.group_ids).toEqual(r.grants.find((g) => g.fed === 2)!.groups.map((g) => g.id));
	});

	it('removes a granted group, and unfiles the member from a federation switched off', async () => {
		const calls = openAccess('superadmin');
		const r = researcher('superadmin');
		const first = r.grants.find((g) => g.fed === 1)!.groups[0]!;
		fireEvent.click(within(section('GNU/Weeb network')).getByRole('button', { name: `Remove ${first.title}` }));
		fireEvent.click(section('Rust and systems').querySelector('input.a-switch')!);
		const body = await save(calls, r.id);
		expect(body.grants.map((g) => g.fed)).toEqual([1]);
		expect(body.grants[0]!.group_ids).not.toContain(first.id);
	});

	it('says when a federation grants no group yet', () => {
		openAccess('superadmin');
		fireEvent.click(section('Security').querySelector('input.a-switch')!);
		expect(within(section('Security')).getByText(/No group granted yet: the member sees nothing of Security/)).toBeTruthy();
	});

	it('lets an admin change only the federations it administers, and names the others', async () => {
		const calls = openAccess('admin');
		const r = researcher('admin');
		const mine = props('admin').federations.map((f) => f.id);
		expect(mine).not.toContain(2);
		expect(section('Rust and systems')).toBeUndefined();
		const note = within(dialog()).getByRole('note', { name: '' });
		expect(note.textContent).toMatch(/Also filed in Rust and systems, which you do not administer/);
		// Its groups there are not the admin's to see: no count of them.
		expect(note.textContent).not.toMatch(/group/);
		const body = await save(calls, r.id);
		// Naming a federation it does not administer would be refused.
		expect(body.grants.every((g) => mine.includes(g.fed))).toBe(true);
		expect(body.grants.map((g) => g.fed)).toEqual([1]);
	});

	it('sends the abilities as chosen', async () => {
		const calls = openAccess('superadmin');
		const r = researcher('superadmin');
		expect(r.abilities).toEqual(['forward', 'fetch_files']);
		fireEvent.click(within(dialog()).getByRole('checkbox', { name: /Fetch files from Telegram/ }));
		const body = await save(calls, r.id);
		expect(body.abilities).toEqual(['forward']);
	});

	it('shows the server\'s reason when a grant is refused', async () => {
		openAccess('superadmin', { 'PATCH /api/accounts/:id': () => json(400, { error: 'bad_request', message: 'Group -1009001 is not in GNU/Weeb network.', field: 'grants' }) });
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save changes' }));
		expect((await within(dialog()).findByRole('alert')).textContent).toBe('Group -1009001 is not in GNU/Weeb network.');
	});

	it('gives an admin only the granted groups it may see', () => {
		const asAdmin = researcher('admin');
		const asSuper = researcher('superadmin');
		expect(asSuper.grants.find((g) => g.fed === 2)!.groups.length).toBeGreaterThan(0);
		expect(asAdmin.grants.find((g) => g.fed === 2)!.groups).toEqual([]);
		expect(asAdmin.grants.find((g) => g.fed === 1)!.groups).toEqual(asSuper.grants.find((g) => g.fed === 1)!.groups);
	});
});
