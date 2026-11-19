// @vitest-environment happy-dom
// The administration and integration pages: accounts, federations,
// forwarding, Join chat and MCP.
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import type { AccountsProps } from './accounts.tsx';
import type { FederationsProps } from './federations.tsx';
import type { D2TProps, T2DProps } from './forwarding.tsx';
import type { McpGroupsProps } from './mcp.tsx';
import type { PickItem } from '../types.ts';
import { adminApi } from '../../standin/fixtures.ts';

beforeEach(setup);
const pick = (id: number, title: string, kind: 'group' | 'user' = 'group'): PickItem => ({ id, title, kind });
const dialog = () => screen.getByRole('dialog');
// The open chat picker's list of chats.
const popup = () => waitFor(() => dialog().querySelector('.a-picker-pop') as HTMLElement);
// The table row of an exposed group (its title also shows elsewhere).
const rowOf = (title: string) => screen.getAllByText(title).map((x) => x.closest('tbody tr')).find(Boolean) as HTMLElement;
const fill = (root: HTMLElement, sel: string, value: string) => fireEvent.change(root.querySelector(sel)!, { target: { value } });
// A new route's chat in one federation: no pick, Save once it loads.
const oneFed = { 'GET /api/forwarding/federations': () => ({ federations: [{ id: 0, name: 'Default' }] }) };
const saveWhenReady = async () => {
	const save = within(dialog()).getByRole('button', { name: 'Save' }) as HTMLButtonElement;
	await waitFor(() => expect(save.disabled).toBe(false));
	fireEvent.click(save);
};

describe('accounts', () => {
	it('lets an admin create a member only with a federation granted, and sends the grant', async () => {
		const calls = fakeApi('admin', { 'POST /api/accounts': () => json(201, { id: 99, invited: false, temporary_password: 'tmp-Zx81' }) });
		renderPage('/settings/accounts', 'admin');
		fireEvent.click(screen.getByRole('button', { name: /New member/ }));
		const d = dialog();
		const inputs = d.querySelectorAll('input:not([type=checkbox]):not([type=radio])');
		fireEvent.change(inputs[0]!, { target: { value: 'newmember' } });
		fireEvent.change(inputs[1]!, { target: { value: 'New member' } });
		fill(d, 'input[type=email]', 'n@example.org');
		const create = within(d).getByRole('button', { name: 'Create account' }) as HTMLButtonElement;
		expect(create.disabled).toBe(true);
		fireEvent.click(d.querySelector('.a-fedgrant input.a-switch')!);
		expect(create.disabled).toBe(false);
		fireEvent.click(create);
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/accounts')).toHaveLength(1));
		const body = callsTo(calls, 'POST', '/api/accounts')[0]!.body as { role: string; grants: { fed: number; group_ids: number[] }[] };
		expect(body).toMatchObject({ username: 'newmember', name: 'New member', email: 'n@example.org', role: 'member' });
		expect(body.grants).toHaveLength(1);
		// The temporary password, shown once.
		expect(await screen.findByLabelText('Temporary password')).toHaveProperty('textContent', 'tmp-Zx81');
		expect(screen.getByText(/Shown only now/)).toBeTruthy();
	});

	it('copies the temporary password, and forgets it once the dialog closes', async () => {
		const write = vi.fn(async () => {});
		vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText: write } });
		fakeApi('superadmin', { 'POST /api/accounts': () => json(201, { id: 99, invited: false, temporary_password: 'tmp-Q2' }) });
		renderPage('/settings/accounts');
		fireEvent.click(screen.getByRole('button', { name: /New account/ }));
		const d = dialog();
		const inputs = d.querySelectorAll('input:not([type=checkbox]):not([type=radio])');
		fireEvent.change(inputs[0]!, { target: { value: 'x1' } });
		fireEvent.change(inputs[1]!, { target: { value: 'X' } });
		fill(d, 'input[type=email]', 'x1@example.org');
		fireEvent.click(within(d).getByRole('button', { name: 'Create account' }));
		fireEvent.click(await screen.findByRole('button', { name: 'Copy' }));
		await screen.findByRole('button', { name: 'Copied' });
		expect(write).toHaveBeenCalledWith('tmp-Q2');
		fireEvent.click(screen.getByRole('button', { name: 'Done' }));
		expect(screen.queryByText('tmp-Q2')).toBeNull();
		expect(document.body.innerHTML).not.toContain('tmp-Q2');
	});

	it('resets an account to a temporary password, only after confirming', async () => {
		const p = fixture<AccountsProps>('/settings/accounts').props;
		const a = p.accounts.find((x) => x.role === 'member' && x.active)!;
		const ask = vi.fn().mockReturnValueOnce(false).mockReturnValueOnce(true);
		vi.stubGlobal('confirm', ask);
		const calls = fakeApi('superadmin', { 'POST /api/accounts/:id/password-reset': () => ({ link_sent: false, temporary_password: 'tmp-R3' }) });
		renderPage('/settings/accounts');
		fireEvent.click(screen.getByRole('button', { name: `Edit ${a.username}` }));
		fireEvent.click(within(dialog()).getByRole('button', { name: /Reset password/ }));
		expect(callsTo(calls, 'POST', `/api/accounts/${a.id}/password-reset`)).toHaveLength(0);
		fireEvent.click(within(dialog()).getByRole('button', { name: /Reset password/ }));
		expect(await screen.findByLabelText('Temporary password')).toHaveProperty('textContent', 'tmp-R3');
		expect(callsTo(calls, 'POST', `/api/accounts/${a.id}/password-reset`)).toHaveLength(1);
	});

	describe('a new account\'s first password', () => {
		const open = (name: string, email: string) => {
			renderPage('/settings/accounts');
			fireEvent.click(screen.getByRole('button', { name: /New account/ }));
			const d = dialog();
			const inputs = d.querySelectorAll('input:not([type=checkbox]):not([type=radio]):not([type=password])');
			fireEvent.change(inputs[0]!, { target: { value: name } });
			fireEvent.change(inputs[1]!, { target: { value: name } });
			fill(d, 'input[type=email]', email);
			return d;
		};
		const created = (c: ReturnType<typeof fakeApi>) => callsTo(c, 'POST', '/api/accounts')[0]!.body as Record<string, unknown>;

		it('sends a typed one, and says it must be changed', async () => {
			const calls = fakeApi('superadmin', { 'POST /api/accounts': () => json(201, { id: 9, invited: false }) });
			const d = open('typed1', '');
			fill(d, 'input[type=password]', 'a typed passphrase');
			fireEvent.click(within(d).getByRole('button', { name: 'Create account' }));
			await screen.findByText(/has the password you typed/);
			expect(created(calls)).toMatchObject({ username: 'typed1', email: null, password: 'a typed passphrase' });
			expect(created(calls)).not.toHaveProperty('invite');
		});

		it('refuses a short one before asking the server', () => {
			const calls = fakeApi('superadmin');
			const d = open('short1', '');
			fill(d, 'input[type=password]', 'short');
			fireEvent.click(within(d).getByRole('button', { name: 'Create account' }));
			expect(within(d).getByRole('alert').textContent).toMatch(/at least 8 characters/);
			expect(callsTo(calls, 'POST', '/api/accounts')).toHaveLength(0);
		});

		it('asks for none when left empty, and shows the generated one', async () => {
			const calls = fakeApi('superadmin', { 'POST /api/accounts': () => json(201, { id: 9, invited: false, temporary_password: 'gen-77' }) });
			const d = open('gen1', 'gen1@example.org');
			fireEvent.click(within(d).getByRole('button', { name: 'Create account' }));
			expect(await screen.findByLabelText('Temporary password')).toHaveProperty('textContent', 'gen-77');
			expect(created(calls)).toMatchObject({ email: 'gen1@example.org', password: null });
		});

		it('sends a set-password link instead, which needs an address', async () => {
			const calls = fakeApi('superadmin', { 'POST /api/accounts': () => json(201, { id: 9, invited: true }) });
			const d = open('inv1', '');
			const sw = within(d).getByRole('checkbox', { name: /Email a set-password link instead/ }) as HTMLInputElement;
			expect(sw.disabled).toBe(true);
			fill(d, 'input[type=email]', 'inv1@example.org');
			fireEvent.click(sw);
			expect(d.querySelector('input[type=password]')).toBeNull();
			fireEvent.click(within(d).getByRole('button', { name: 'Create account' }));
			await screen.findByText(/went to inv1@example.org/);
			expect(created(calls)).toMatchObject({ email: 'inv1@example.org', invite: true });
			expect(created(calls)).not.toHaveProperty('password');
		});
	});

	it('offers no set-password links when mail is not set up', () => {
		const page = fixture<AccountsProps>('/settings/accounts');
		const props = { ...page.props, mail: false };
		const invited = props.accounts.find((x) => x.invited && x.email)!;
		renderPage('/settings/accounts', 'superadmin', { props });
		fireEvent.click(screen.getByRole('button', { name: /New account/ }));
		fill(dialog(), 'input[type=email]', 'someone@example.org');
		expect(within(dialog()).queryByRole('checkbox', { name: /Email a set-password link instead/ })).toBeNull();
		expect(dialog().querySelector('input[type=password]')).not.toBeNull();
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Cancel' }));
		fireEvent.click(screen.getByRole('button', { name: `Edit ${invited.username}` }));
		expect(within(dialog()).queryByRole('button', { name: /Resend set-password link/ })).toBeNull();
		expect(within(dialog()).getByRole('button', { name: /Reset password/ })).toBeTruthy();
	});

	it('says a reset link went out when the server mailed one', async () => {
		const p = fixture<AccountsProps>('/settings/accounts').props;
		const a = p.accounts.find((x) => x.role === 'member' && x.active)!;
		vi.stubGlobal('confirm', () => true);
		fakeApi('superadmin', { 'POST /api/accounts/:id/password-reset': () => ({ link_sent: true }) });
		renderPage('/settings/accounts');
		fireEvent.click(screen.getByRole('button', { name: `Edit ${a.username}` }));
		fireEvent.click(within(dialog()).getByRole('button', { name: /Reset password/ }));
		await within(dialog()).findByText(/went to the account's address/);
		expect(screen.queryByLabelText('Temporary password')).toBeNull();
	});

	it('resends an invitation, and shows why one cannot be', async () => {
		const p = fixture<AccountsProps>('/settings/accounts').props;
		const a = p.accounts.find((x) => x.invited && x.email)!;
		fakeApi('superadmin', { 'POST /api/accounts/:id/resend-invite': () => json(409, { error: 'mail_off', message: 'Mail is not set up on this server.' }) });
		renderPage('/settings/accounts');
		fireEvent.click(screen.getByRole('button', { name: `Edit ${a.username}` }));
		fireEvent.click(within(dialog()).getByRole('button', { name: /Resend set-password link/ }));
		await within(dialog()).findByText('Mail is not set up on this server.');
	});

	it('shows and sends only what the server lets it change yet', async () => {
		const p = fixture<AccountsProps>('/settings/accounts').props;
		const props = { ...p, editable: { federations: false, group_grants: false, restrictions: false, restrict_private: false, abilities: false } };
		const member = p.accounts.find((x) => x.role === 'member' && x.active)!;
		const admin = p.accounts.find((x) => x.role === 'admin')!;
		// Saving fetches the page data again: keep answering with these flags.
		const calls = fakeApi('superadmin', { 'PATCH /api/accounts/:id': () => ({}), 'GET /api/page/settings/accounts': () => ({ ...fixture('/settings/accounts'), props }) });
		renderPage('/settings/accounts', 'superadmin', { props });
		fireEvent.click(screen.getByRole('button', { name: `Edit ${member.username}` }));
		fireEvent.click(within(dialog()).getByRole('tab', { name: 'Access' }));
		const note = 'Federations, group grants, restrictions and abilities are not available yet.';
		expect(within(dialog()).getByRole('note').textContent).toBe(note);
		expect(within(dialog()).queryByText('Abilities')).toBeNull();
		expect(dialog().querySelectorAll('.a-fedgrant')).toHaveLength(1); // Default only
		expect(dialog().querySelector('.a-multi')).toBeNull(); // no group pickers
		expect(within(dialog()).queryByText(/sees only the groups/)).toBeNull();
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save changes' }));
		await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/accounts/${member.id}`)).toHaveLength(1));
		const mb = callsTo(calls, 'PATCH', `/api/accounts/${member.id}`)[0]!.body as Record<string, unknown>;
		expect(mb).not.toHaveProperty('abilities');
		expect((mb.grants as { group_ids: number[] }[]).every((g) => g.group_ids.length === 0)).toBe(true);
		fireEvent.click(screen.getByRole('button', { name: `Edit ${admin.username}` }));
		fireEvent.click(within(dialog()).getByRole('tab', { name: 'Access' }));
		expect(within(dialog()).queryByText('Administers')).toBeNull();
		expect(within(dialog()).queryByText('Restrictions')).toBeNull();
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save changes' }));
		await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/accounts/${admin.id}`)).toHaveLength(1));
		const ab = callsTo(calls, 'PATCH', `/api/accounts/${admin.id}`)[0]!.body as Record<string, unknown>;
		expect(ab).not.toHaveProperty('admin_of');
		expect(ab).not.toHaveProperty('restrict');
	});

	describe('hiding private messages alone', () => {
		const flags = { federations: true, group_grants: true, restrictions: false, restrict_private: true, abilities: true };
		const setup2 = () => {
			const p = fixture<AccountsProps>('/settings/accounts').props;
			const props = { ...p, editable: flags };
			const admin = p.accounts.find((x) => x.role === 'admin' && x.active)!;
			const member = p.accounts.find((x) => x.role === 'member' && x.active)!;
			const calls = fakeApi('superadmin', { 'PATCH /api/accounts/:id': () => ({}), 'GET /api/page/settings/accounts': () => ({ ...fixture('/settings/accounts'), props }) });
			renderPage('/settings/accounts', 'superadmin', { props });
			return { admin, member, calls };
		};
		const access = (username: string) => {
			fireEvent.click(screen.getByRole('button', { name: `Edit ${username}` }));
			fireEvent.click(within(dialog()).getByRole('tab', { name: 'Access' }));
		};

		it('offers an admin only the private-messages restriction', () => {
			const { admin } = setup2();
			access(admin.username);
			expect(within(dialog()).getByText('Restrictions')).toBeTruthy();
			expect(within(dialog()).getByRole('checkbox', { name: /Hide private messages/ })).toBeTruthy();
			expect(within(dialog()).queryByText('Hide groups')).toBeNull();
			expect(within(dialog()).queryByText('Allow Join chat')).toBeNull();
			expect(within(dialog()).getByRole('note').textContent).toBe('Hidden groups and Join chat are not available yet.');
		});

		it('sends the private-messages restriction alone', async () => {
			const { admin, calls } = setup2();
			access(admin.username);
			const box = within(dialog()).getByRole('checkbox', { name: /Hide private messages/ }) as HTMLInputElement;
			const was = box.checked;
			fireEvent.click(box);
			fireEvent.click(within(dialog()).getByRole('button', { name: 'Save changes' }));
			await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/accounts/${admin.id}`)).toHaveLength(1));
			const body = callsTo(calls, 'PATCH', `/api/accounts/${admin.id}`)[0]!.body as { restrict: unknown };
			expect(body.restrict).toEqual({ hide_private: !was, hide_groups: [], join_chat: false });
		});

		it('sends it for a member made admin in the same save', async () => {
			const { member, calls } = setup2();
			fireEvent.click(screen.getByRole('button', { name: `Edit ${member.username}` }));
			fireEvent.click(within(dialog()).getByRole('radio', { name: /^Admin/ }));
			fireEvent.click(within(dialog()).getByRole('tab', { name: 'Access' }));
			fireEvent.click(within(dialog()).getByRole('checkbox', { name: /Hide private messages/ }));
			fireEvent.click(within(dialog()).getByRole('button', { name: 'Save changes' }));
			await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/accounts/${member.id}`)).toHaveLength(1));
			expect(callsTo(calls, 'PATCH', `/api/accounts/${member.id}`)[0]!.body).toMatchObject({ role: 'admin', restrict: { hide_private: true, hide_groups: [], join_chat: false } });
		});

		it('offers no restriction to a member', () => {
			const { member } = setup2();
			access(member.username);
			expect(within(dialog()).queryByRole('checkbox', { name: /Hide private messages/ })).toBeNull();
		});

		it('offers nothing without the flag', () => {
			const p = fixture<AccountsProps>('/settings/accounts').props;
			const admin = p.accounts.find((x) => x.role === 'admin' && x.active)!;
			renderPage('/settings/accounts', 'superadmin', { props: { ...p, editable: { ...flags, restrict_private: false } } });
			access(admin.username);
			expect(within(dialog()).queryByText('Restrictions')).toBeNull();
		});
	});

	describe('the Join chat grant', () => {
		it('lets a superadmin allow an admin Join chat, sent with the restrictions', async () => {
			const p = fixture<AccountsProps>('/settings/accounts').props;
			expect(p.editable.restrictions).toBe(true);
			const admin = p.accounts.find((a) => a.role === 'admin' && !a.restrict?.join_chat)!;
			const calls = fakeApi('superadmin', { 'PATCH /api/accounts/:id': () => ({}) });
			renderPage('/settings/accounts');
			fireEvent.click(screen.getByRole('button', { name: `Edit ${admin.username}` }));
			fireEvent.click(within(dialog()).getByRole('tab', { name: 'Access' }));
			const box = within(dialog()).getByRole('checkbox', { name: /Allow Join chat/ }) as HTMLInputElement;
			expect(box.checked).toBe(false);
			fireEvent.click(box);
			fireEvent.click(within(dialog()).getByRole('button', { name: 'Save changes' }));
			await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/accounts/${admin.id}`)).toHaveLength(1));
			expect((callsTo(calls, 'PATCH', `/api/accounts/${admin.id}`)[0]!.body as { restrict: { join_chat: boolean } }).restrict.join_chat).toBe(true);
		});
	});

	describe('hiding private messages and groups', () => {
		const flags = { federations: true, group_grants: true, restrictions: false, restrict_private: true, restrict_groups: true, abilities: true };
		const withGroups = () => {
			const p = fixture<AccountsProps>('/settings/accounts').props;
			const admin = p.accounts.find((a) => a.role === 'admin' && (a.restrict?.hide_groups.length ?? 0) > 0)!;
			const props = { ...p, editable: flags };
			const calls = fakeApi('superadmin', { 'PATCH /api/accounts/:id': () => ({}), 'GET /api/page/settings/accounts': () => ({ ...fixture('/settings/accounts'), props }) });
			renderPage('/settings/accounts', 'superadmin', { props });
			return { admin, calls };
		};

		it('offers a superadmin both, and says only Join chat is missing', () => {
			const { admin } = withGroups();
			fireEvent.click(screen.getByRole('button', { name: `Edit ${admin.username}` }));
			fireEvent.click(within(dialog()).getByRole('tab', { name: 'Access' }));
			expect(within(dialog()).getByRole('checkbox', { name: /Hide private messages/ })).toBeTruthy();
			expect(within(dialog()).getByText('Hide groups')).toBeTruthy();
			expect(dialog().querySelector('.a-multi')).toBeTruthy();
			expect(within(dialog()).queryByText('Allow Join chat')).toBeNull();
			expect(within(dialog()).getByRole('note').textContent).toBe('Join chat is not available yet.');
		});

		it('sends the hidden groups as ids, with Join chat off', async () => {
			const { admin, calls } = withGroups();
			fireEvent.click(screen.getByRole('button', { name: `Edit ${admin.username}` }));
			fireEvent.click(within(dialog()).getByRole('button', { name: 'Save changes' }));
			await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/accounts/${admin.id}`)).toHaveLength(1));
			expect(callsTo(calls, 'PATCH', `/api/accounts/${admin.id}`)[0]!.body).toMatchObject({
				restrict: { hide_private: admin.restrict!.hide_private, hide_groups: admin.restrict!.hide_groups.map((g) => g.id), join_chat: false },
			});
		});

		it('sums up the hidden groups in the account row', () => {
			const { admin } = withGroups();
			const n = admin.restrict!.hide_groups.length;
			const row = screen.getByRole('button', { name: `Edit ${admin.username}` }).closest('tr')!;
			expect(row.textContent).toContain(`${n} hidden group${n > 1 ? 's' : ''}`);
		});

		it('offers hiding groups only to a superadmin', () => {
			const p = fixture<AccountsProps>('/settings/accounts', 'admin').props;
			renderPage('/settings/accounts', 'admin', { props: { ...p, editable: flags } });
			// An admin manages members only: no admin account to restrict.
			const edits = screen.queryAllByRole('button', { name: /^Edit / });
			for (const b of edits) {
				fireEvent.click(b);
				const access = within(dialog()).queryByRole('tab', { name: 'Access' });
				if (access) fireEvent.click(access);
				expect(within(dialog()).queryByText('Hide groups')).toBeNull();
				fireEvent.click(within(dialog()).getByRole('button', { name: 'Close' }));
			}
		});
	});

	it('sends an admin\'s restrictions with hidden groups as ids', async () => {
		const p = fixture<AccountsProps>('/settings/accounts').props;
		const admin = p.accounts.find((a) => a.role === 'admin' && (a.restrict?.hide_groups.length ?? 0) > 0)!;
		const calls = fakeApi('superadmin', { 'PATCH /api/accounts/:id': (c) => ({ ...admin, ...(c.body as object) }) });
		// Once every restriction is built (editable.restrictions).
		renderPage('/settings/accounts', 'superadmin', { props: { ...p, editable: { ...p.editable, restrictions: true } } });
		fireEvent.click(screen.getByRole('button', { name: `Edit ${admin.username}` }));
		fireEvent.click(within(dialog()).getByRole('tab', { name: 'Access' }));
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save changes' }));
		await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/accounts/${admin.id}`)).toHaveLength(1));
		const body = callsTo(calls, 'PATCH', `/api/accounts/${admin.id}`)[0]!.body as { restrict: { hide_groups: unknown[] } };
		expect(body.restrict.hide_groups).toEqual(admin.restrict!.hide_groups.map((g) => g.id));
	});

	it('shows the server\'s message when an action is refused', async () => {
		const p = fixture<AccountsProps>('/settings/accounts').props;
		const a = p.accounts.find((x) => x.role === 'member' && x.active)!;
		fakeApi('superadmin', { 'POST /api/accounts/:id/disable': () => json(409, { error: 'conflict', message: 'Refused for a test.' }) });
		renderPage('/settings/accounts');
		fireEvent.click(screen.getByRole('button', { name: `Edit ${a.username}` }));
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Disable account' }));
		await within(dialog()).findByText('Refused for a test.');
	});

	it('cannot disable or demote the last active superadmin', () => {
		const p = fixture<AccountsProps>('/settings/accounts').props;
		const last = { ...p, accounts: p.accounts.map((a) => (a.role === 'superadmin' ? { ...a, last_superadmin: a.id === 1, active: a.id === 1 } : a)) };
		renderPage('/settings/accounts', 'superadmin', { props: last });
		fireEvent.click(screen.getByRole('button', { name: 'Edit admin' }));
		expect(within(dialog()).queryByRole('button', { name: 'Disable account' })).toBeNull();
		expect(within(dialog()).getByText(/last active superadmin/)).toBeTruthy();
	});
});

describe('federations', () => {
	it('loads every group of a federation before saving, and sends the complete list', async () => {
		const fed = fixture<FederationsProps>('/settings/federations').props.federations.find((f) => !f.default)!;
		const all = Array.from({ length: 250 }, (_, i) => pick(-1000 - i, `Group ${i}`));
		const calls = fakeApi('superadmin', {
			'GET /api/chats/search': (c) => {
				const from = Number(c.query.get('cursor') ?? 0);
				return { items: all.slice(from, from + 200), next_cursor: from + 200 < all.length ? String(from + 200) : null };
			},
			'PATCH /api/federations/:id': () => ({}),
		});
		renderPage('/settings/federations');
		fireEvent.click(screen.getByRole('button', { name: `Edit ${fed.name}` }));
		const save = within(dialog()).getByRole('button', { name: 'Save changes' }) as HTMLButtonElement;
		expect(save.disabled).toBe(true); // until every group is there
		await waitFor(() => expect(save.disabled).toBe(false));
		expect(callsTo(calls, 'GET', '/api/chats/search').every((c) => c.query.get('federation') === String(fed.id))).toBe(true);
		fireEvent.click(save);
		await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/federations/${fed.id}`)).toHaveLength(1));
		expect((callsTo(calls, 'PATCH', `/api/federations/${fed.id}`)[0]!.body as { group_ids: number[] }).group_ids).toHaveLength(250);
	});
});

describe('forwarding', () => {
	it('adds a webhook for a searched chat', async () => {
		const calls = fakeApi('superadmin', {
			...oneFed,
			'GET /api/chats/search': () => ({ items: [pick(-100777, 'Rust Programming')], next_cursor: null }),
			'POST /api/forwarding/telegram-discord': () => json(201, { id: 50 }),
		});
		renderPage('/platform-fwd/telegram-discord');
		fireEvent.click(screen.getByRole('button', { name: /Add webhook/ }));
		fireEvent.click(dialog().querySelector('.a-picker-btn')!);
		fireEvent.click(await within(await popup()).findByRole('button', { name: /Rust Programming/ }));
		fill(dialog(), '#webhook-url', 'https://discord.com/api/webhooks/1/abc');
		await saveWhenReady();
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/forwarding/telegram-discord')).toHaveLength(1));
		expect(callsTo(calls, 'POST', '/api/forwarding/telegram-discord')[0]!.body).toMatchObject({ chat_id: -100777, chat_kind: 'group', webhook_url: 'https://discord.com/api/webhooks/1/abc', enabled: true });
	});

	it('shows why a webhook URL is refused', async () => {
		fakeApi('superadmin', {
			...oneFed,
			'GET /api/chats/search': () => ({ items: [pick(-100777, 'Rust Programming')], next_cursor: null }),
			'POST /api/forwarding/telegram-discord': () => json(400, { error: 'bad_webhook', message: 'That is not a Discord webhook URL.' }),
		});
		renderPage('/platform-fwd/telegram-discord');
		fireEvent.click(screen.getByRole('button', { name: /Add webhook/ }));
		fireEvent.click(dialog().querySelector('.a-picker-btn')!);
		fireEvent.click(await within(await popup()).findByRole('button', { name: /Rust Programming/ }));
		fill(dialog(), '#webhook-url', 'https://evil.example/x');
		await saveWhenReady();
		await within(dialog()).findByText('That is not a Discord webhook URL.');
	});

	// A masked password field in a form looks like a login form to
	// browsers, which filled the saved site password into it.
	it('keeps password managers out of the webhook URL', async () => {
		fakeApi('superadmin', { 'GET /api/chats/search': () => ({ items: [pick(-100777, 'Rust Programming')], next_cursor: null }) });
		renderPage('/platform-fwd/telegram-discord');
		fireEvent.click(screen.getByRole('button', { name: /Add webhook/ }));
		expect(dialog().querySelector('input[type=password]')).toBeNull();
		const url = dialog().querySelector<HTMLInputElement>('#webhook-url')!;
		expect(url.type).toBe('text');
		expect(url.name).toBe('webhook-url');
		expect(url.className).toContain('a-secret');
		for (const [a, v] of [['autocomplete', 'off'], ['autocorrect', 'off'], ['autocapitalize', 'off'], ['spellcheck', 'false'], ['data-lpignore', 'true'], ['data-form-type', 'other']]) expect(url.getAttribute(a!)).toBe(v);
		expect(url.getAttribute('data-1p-ignore')).not.toBeNull();
		const eye = within(dialog()).getByRole('button', { name: 'Show webhook URL' });
		fireEvent.click(eye);
		expect(eye.getAttribute('aria-pressed')).toBe('true');
		expect(url.className).not.toContain('a-secret');
		fireEvent.click(dialog().querySelector('.a-picker-btn')!);
		const search = (await popup()).querySelector<HTMLInputElement>('input')!;
		expect(search.getAttribute('autocomplete')).toBe('off');
		expect(search.getAttribute('data-form-type')).toBe('other');
		// Shown again masked after closing and reopening.
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Cancel' }));
		fireEvent.click(screen.getByRole('button', { name: /Add webhook/ }));
		expect(dialog().querySelector('#webhook-url')!.className).toContain('a-secret');
	});

	it('tests a saved webhook by its id, and switches a route off', async () => {
		const r = fixture<T2DProps>('/platform-fwd/telegram-discord').props.routes[0]!;
		const calls = fakeApi('superadmin', { 'POST /api/forwarding/telegram-discord/test': () => undefined, 'PATCH /api/forwarding/telegram-discord/:id': () => undefined });
		renderPage('/platform-fwd/telegram-discord');
		const row = screen.getAllByRole('button', { name: 'Edit' })[0]!.closest('tr')!;
		fireEvent.click(row.querySelector('input.a-switch')!);
		await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/forwarding/telegram-discord/${r.id}`)).toHaveLength(1));
		expect(callsTo(calls, 'PATCH', `/api/forwarding/telegram-discord/${r.id}`)[0]!.body).toEqual({ enabled: !r.enabled });
		fireEvent.click(within(row).getByRole('button', { name: 'Edit' }));
		fireEvent.click(within(dialog()).getByRole('button', { name: /Send test message/ }));
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/forwarding/telegram-discord/test')).toHaveLength(1));
		expect(callsTo(calls, 'POST', '/api/forwarding/telegram-discord/test')[0]!.body).toEqual({ id: r.id });
	});

	it('deletes a route only after confirming', async () => {
		const r = fixture<T2DProps>('/platform-fwd/telegram-discord').props.routes[0]!;
		const calls = fakeApi('superadmin', { 'DELETE /api/forwarding/telegram-discord/:id': () => undefined });
		const ask = vi.fn().mockReturnValueOnce(false).mockReturnValueOnce(true);
		vi.stubGlobal('confirm', ask);
		renderPage('/platform-fwd/telegram-discord');
		const del = screen.getAllByRole('button', { name: 'Delete' })[0]!;
		fireEvent.click(del);
		expect(callsTo(calls, 'DELETE', /^\/api\/forwarding\//)).toHaveLength(0);
		fireEvent.click(del);
		await waitFor(() => expect(callsTo(calls, 'DELETE', `/api/forwarding/telegram-discord/${r.id}`)).toHaveLength(1));
		expect(ask).toHaveBeenCalledTimes(2);
	});
});

describe('people with access', () => {
	it('shows the owner and offers no transfer while the service has none', () => {
		renderPage('/platform-fwd/telegram-discord');
		fireEvent.click(screen.getAllByRole('button', { name: 'People with access' })[0]!);
		expect(within(dialog()).getByText('Owner')).toBeTruthy();
		expect(within(dialog()).queryByRole('button', { name: 'Transfer ownership' })).toBeNull();
		expect(within(dialog()).queryByText('Transfer ownership to')).toBeNull();
	});
});

describe('Discord to Telegram', () => {
	const ref = (id: number, kind: 'discord' | 'telegram', name: string, enabled = true) => ({ id, kind, name, identity: null, status: enabled ? 'connected' as const : 'disabled' as const, enabled });
	const BOTS = [ref(2, 'discord', 'Discord reader'), ref(1, 'telegram', 'Logger relay'), ref(4, 'telegram', 'Research bot', false)];
	const bots = { 'GET /api/bots': (c: { query: URLSearchParams }) => ({ bots: BOTS.filter((b) => b.kind === c.query.get('kind')) }) };

	it('adds a route read by a Discord bot and sent by a Telegram bot, into a private chat too', async () => {
		const calls = fakeApi('superadmin', {
			...bots, ...oneFed,
			'GET /api/chats/search': () => ({ items: [pick(4242, 'Dewi Kusuma', 'user')], next_cursor: null }),
			'POST /api/forwarding/discord-telegram': () => json(201, { id: 9 }),
		});
		renderPage('/platform-fwd/discord-telegram');
		fireEvent.click(screen.getByRole('button', { name: /Add route/ }));
		const save = within(dialog()).getByRole('button', { name: 'Save' }) as HTMLButtonElement;
		fill(dialog(), 'input[inputmode=numeric]', '1093447120571433021');
		const discord = await within(dialog()).findByRole('combobox', { name: 'Discord bot' });
		fireEvent.change(discord, { target: { value: '2' } });
		fireEvent.click(dialog().querySelector('.a-picker-btn')!);
		fireEvent.click(await within(await popup()).findByRole('button', { name: /Dewi Kusuma/ }));
		expect(save.disabled).toBe(true); // until the Telegram bot is chosen
		const telegram = within(dialog()).getByRole('combobox', { name: 'Telegram bot' });
		expect([...(telegram as HTMLSelectElement).options].map((o) => o.textContent)).toEqual(['Choose a bot...', 'Logger relay - connected', 'Research bot - disabled']);
		fireEvent.change(telegram, { target: { value: '1' } });
		await saveWhenReady();
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/forwarding/discord-telegram')).toHaveLength(1));
		const body = callsTo(calls, 'POST', '/api/forwarding/discord-telegram')[0]!.body as Record<string, unknown>;
		expect(body).toMatchObject({ discord_channel_id: '1093447120571433021', chat_id: 4242, discord_bot_id: 2, telegram_bot_id: 1, enabled: true });
		expect(body).not.toHaveProperty('bot_token');
		expect(callsTo(calls, 'GET', '/api/bots').map((c) => c.query.get('kind')).sort()).toEqual(['discord', 'telegram']);
		// Groups, channels and users alike: no kind restriction.
		expect(callsTo(calls, 'GET', '/api/chats/search')[0]!.query.get('kind')).toBeNull();
	});

	it('shows each route\'s bots with their state, and why a route forwards nothing', () => {
		const rs = fixture<D2TProps>('/platform-fwd/discord-telegram').props.routes;
		renderPage('/platform-fwd/discord-telegram');
		const rows = screen.getAllByRole('button', { name: 'Edit' }).map((b) => b.closest('tr')!);
		const unread = rs.findIndex((r) => !r.discord_bot);
		expect(within(rows[unread]!).getByText('no Discord bot')).toBeTruthy();
		expect(within(rows[unread]!).getByText('bot disabled')).toBeTruthy();
		expect(within(rows[unread]!).getByText('Stopped: no Discord bot')).toBeTruthy();
		const read = rs.findIndex((r) => r.discord_bot);
		expect(within(rows[read]!).getByText(rs[read]!.discord_bot!.name)).toBeTruthy();
		expect(within(rows[read]!).getByText('intent missing')).toBeTruthy();
		expect(within(rows[read]!).getByText('On')).toBeTruthy();
	});

	it('picks a Discord bot for a route that has none', async () => {
		const rs = fixture<D2TProps>('/platform-fwd/discord-telegram').props.routes;
		const r = rs.find((x) => !x.discord_bot)!;
		const calls = fakeApi('superadmin', { ...bots, 'PATCH /api/forwarding/discord-telegram/:id': () => undefined });
		renderPage('/platform-fwd/discord-telegram');
		fireEvent.click(screen.getByRole('button', { name: /no Discord bot/ }));
		const save = within(dialog()).getByRole('button', { name: 'Save' }) as HTMLButtonElement;
		expect(save.disabled).toBe(true);
		fireEvent.change(await within(dialog()).findByRole('combobox', { name: 'Discord bot' }), { target: { value: '2' } });
		fireEvent.click(save);
		await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/forwarding/discord-telegram/${r.id}`)).toHaveLength(1));
		expect(callsTo(calls, 'PATCH', `/api/forwarding/discord-telegram/${r.id}`)[0]!.body).toEqual({ types: r.types, enabled: r.enabled, discord_bot_id: 2 });
	});

	it('edits a route\'s channel and bots, sending only what changed', async () => {
		const r = fixture<D2TProps>('/platform-fwd/discord-telegram').props.routes.find((x) => x.discord_bot)!;
		const calls = fakeApi('superadmin', { ...bots, 'PATCH /api/forwarding/discord-telegram/:id': () => undefined });
		renderPage('/platform-fwd/discord-telegram');
		const row = screen.getByText(r.channel_name).closest('tr')!;
		fireEvent.click(within(row).getByRole('button', { name: 'Edit' }));
		fill(dialog(), 'input[inputmode=numeric]', '1111111111111111111');
		fireEvent.change(await within(dialog()).findByRole('combobox', { name: 'Telegram bot' }), { target: { value: '4' } });
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save' }));
		await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/forwarding/discord-telegram/${r.id}`)).toHaveLength(1));
		expect(callsTo(calls, 'PATCH', `/api/forwarding/discord-telegram/${r.id}`)[0]!.body).toEqual({ types: r.types, enabled: r.enabled, discord_channel_id: '1111111111111111111', telegram_bot_id: 4 });
	});

	it('points to the Bots page when there is no bot of a kind yet', async () => {
		fakeApi('superadmin', { 'GET /api/bots': () => ({ bots: [] }) });
		renderPage('/platform-fwd/discord-telegram');
		fireEvent.click(screen.getByRole('button', { name: /Add route/ }));
		const links = await within(dialog()).findAllByRole('link', { name: 'Add one on the Bots page' });
		expect(links).toHaveLength(2);
		expect(links[0]!.getAttribute('href')).toBe('/platform-fwd/bots');
	});
});

describe('Telegram to Discord edits', () => {
	it('moves a webhook to another chat, sending the chat only when it changed', async () => {
		const r = fixture<T2DProps>('/platform-fwd/telegram-discord').props.routes[0]!;
		const calls = fakeApi('superadmin', {
			'GET /api/chats/search': () => ({ items: [pick(4242, 'Dewi Kusuma', 'user')], next_cursor: null }),
			'PATCH /api/forwarding/telegram-discord/:id': () => undefined,
		});
		renderPage('/platform-fwd/telegram-discord');
		const edit = () => fireEvent.click(within(screen.getAllByRole('button', { name: 'Edit' })[0]!.closest('tr')!).getByRole('button', { name: 'Edit' }));
		edit();
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save' }));
		await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/forwarding/telegram-discord/${r.id}`)).toHaveLength(1));
		expect(callsTo(calls, 'PATCH', `/api/forwarding/telegram-discord/${r.id}`)[0]!.body).not.toHaveProperty('chat_id');
		edit();
		fireEvent.click(dialog().querySelector('.a-picker-btn')!);
		fireEvent.click(await within(await popup()).findByRole('button', { name: /Dewi Kusuma/ }));
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save' }));
		await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/forwarding/telegram-discord/${r.id}`)).toHaveLength(2));
		expect(callsTo(calls, 'PATCH', `/api/forwarding/telegram-discord/${r.id}`)[1]!.body).toMatchObject({ chat_id: 4242, chat_kind: 'user' });
	});
});

describe('Bots links', () => {
	it('links the Bots page from the menu and from Platform forwarding', () => {
		renderPage('/platform-fwd');
		const nav = screen.getAllByRole('link', { name: 'Bots' });
		expect(nav.map((a) => a.getAttribute('href'))).toEqual(expect.arrayContaining(['/platform-fwd/bots']));
		expect(screen.getAllByRole('link').some((a) => a.getAttribute('href') === '/platform-fwd/bots' && a.classList.contains('a-dircard'))).toBe(true);
	});
});

describe('Join chat', () => {
	it('says up front when the logger\'s API is not set up, and offers nothing to try', () => {
		const p = fixture<{ api_configured: boolean }>('/join-chat').props;
		const calls = fakeApi('superadmin');
		renderPage('/join-chat', 'superadmin', { props: { ...p, api_configured: false } });
		expect(screen.getByRole('status').textContent).toContain('not connected to the logger\'s API');
		expect((screen.getByRole('textbox', { name: /Invite link or username/ }) as HTMLInputElement).disabled).toBe(true);
		const preview = screen.getByRole('button', { name: 'Preview' }) as HTMLButtonElement;
		expect(preview.disabled).toBe(true);
		fireEvent.click(preview);
		expect(callsTo(calls, 'POST', /join-chat/)).toHaveLength(0);
	});

	it('shows no such notice when the API is set up', () => {
		renderPage('/join-chat');
		expect(screen.queryByText(/not connected to the logger/)).toBeNull();
		expect((screen.getByRole('textbox', { name: /Invite link or username/ }) as HTMLInputElement).disabled).toBe(false);
	});

	it('previews, then joins into the chosen federations', async () => {
		const chat = { chat_id: -1001999000001, title: 'Compiler Nerds ID', type: 'supergroup', member_count: 1843, username: 'compilernerds_id', is_member: false, needs_approval: false };
		const calls = fakeApi('superadmin', {
			'POST /api/join-chat/preview': () => ({ outcome: 'ok', message: '', chat }),
			'POST /api/join-chat/join': () => ({ outcome: 'joined', message: 'Joined.', chat: { ...chat, is_member: true } }),
		});
		renderPage('/join-chat');
		fireEvent.change(screen.getByPlaceholderText('@groupname or https://t.me/+...'), { target: { value: '@compilernerds_id' } });
		fireEvent.click(screen.getByRole('button', { name: 'Preview' }));
		await screen.findByText(/Join this chat\?/);
		fireEvent.click(document.querySelector('label.a-checkchip input')!);
		fireEvent.click(screen.getByRole('button', { name: 'Join' }));
		await screen.findByText('Joined.');
		const body = callsTo(calls, 'POST', '/api/join-chat/join')[0]!.body as { ref: string; federation_ids: number[] };
		expect(body.ref).toBe('@compilernerds_id');
		expect(body.federation_ids).toHaveLength(1);
	});
});

describe('MCP', () => {
	it('exposes a group, offering none of the ones already exposed', async () => {
		const p = fixture<McpGroupsProps>('/mcp-admin/groups').props;
		const exposed = p.groups[0]!.group;
		const calls = fakeApi('superadmin', {
			'GET /api/chats/search': () => ({ items: [pick(exposed.id, exposed.title), pick(-100555, 'Not exposed yet')], next_cursor: null }),
			'POST /api/mcp/groups': () => json(201, { group_id: -100555 }),
		});
		renderPage('/mcp-admin/groups');
		fireEvent.click(screen.getByRole('button', { name: /Expose a group/ }));
		fireEvent.click(dialog().querySelector('.a-picker-btn')!);
		const list = await popup();
		fireEvent.click(await within(list).findByRole('button', { name: /Not exposed yet/ }));
		expect(within(list).queryByText(exposed.title)).toBeNull();
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Expose' }));
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/mcp/groups')).toHaveLength(1));
		expect(callsTo(calls, 'POST', '/api/mcp/groups')[0]!.body).toMatchObject({ group_id: -100555 });
	});

	it('edits a group\'s note in place, keeping when it was added', async () => {
		const row = fixture<McpGroupsProps>('/mcp-admin/groups').props.groups[0]!;
		const calls = fakeApi('superadmin', { 'PATCH /api/mcp/groups/:id': (c) => { const [st, out] = adminApi('PATCH', c.path.slice('/api/'.length), c.body, 'superadmin'); return st === 204 ? undefined : json(st, out); } });
		renderPage('/mcp-admin/groups');
		const tr = () => rowOf(row.group.title);
		const added = tr().querySelector(`td[title="${row.added}"]`)!.textContent;
		fireEvent.click(within(tr()).getByRole('button', { name: /Edit note/ }));
		const input = within(dialog()).getByLabelText(/Note/) as HTMLInputElement;
		expect(input.value).toBe(row.note);
		expect(input.maxLength).toBe(255);
		fireEvent.change(input, { target: { value: 'Reviewed in November' } });
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save' }));
		await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
		expect(callsTo(calls, 'PATCH', `/api/mcp/groups/${row.group.id}`)[0]!.body).toEqual({ note: 'Reviewed in November' });
		await waitFor(() => expect(within(tr()).getByText('Reviewed in November')).toBeTruthy());
		expect(tr().querySelector(`td[title="${row.added}"]`)!.textContent).toBe(added);
	});

	it('leaves the note alone on Cancel, and sends an empty note to clear it', async () => {
		const row = fixture<McpGroupsProps>('/mcp-admin/groups').props.groups[1]!;
		const calls = fakeApi('superadmin', { 'PATCH /api/mcp/groups/:id': () => undefined });
		renderPage('/mcp-admin/groups');
		const tr = () => rowOf(row.group.title);
		fireEvent.click(within(tr()).getByRole('button', { name: /Edit note/ }));
		fireEvent.change(within(dialog()).getByLabelText(/Note/), { target: { value: 'Not this' } });
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Cancel' }));
		expect(screen.queryByRole('dialog')).toBeNull();
		expect(within(tr()).getByText(row.note)).toBeTruthy();
		expect(callsTo(calls, 'PATCH', `/api/mcp/groups/${row.group.id}`)).toHaveLength(0);
		fireEvent.click(within(tr()).getByRole('button', { name: /Edit note/ }));
		expect((within(dialog()).getByLabelText(/Note/) as HTMLInputElement).value).toBe(row.note);
		fireEvent.change(within(dialog()).getByLabelText(/Note/), { target: { value: '' } });
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save' }));
		await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/mcp/groups/${row.group.id}`)).toHaveLength(1));
		expect(callsTo(calls, 'PATCH', `/api/mcp/groups/${row.group.id}`)[0]!.body).toEqual({ note: '' });
	});

	it('says so when the group is no longer exposed', async () => {
		const row = fixture<McpGroupsProps>('/mcp-admin/groups').props.groups[0]!;
		fakeApi('superadmin', { 'PATCH /api/mcp/groups/:id': () => json(404, { error: 'not_found' }) });
		renderPage('/mcp-admin/groups');
		fireEvent.click(within(rowOf(row.group.title)).getByRole('button', { name: /Edit note/ }));
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save' }));
		expect((await within(dialog()).findByRole('alert')).textContent).toMatch(/no longer exposed/);
	});

	it('shows a new token once', async () => {
		fakeApi('superadmin', { 'POST /api/mcp/tokens': () => json(201, { id: 7, token: 'tgmcp_test_once' }) });
		renderPage('/mcp-admin/tokens');
		fireEvent.click(screen.getByRole('button', { name: /Mint a token/ }));
		fill(dialog(), 'input', 'ci');
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Mint' }));
		await screen.findByText('tgmcp_test_once');
		expect(screen.getByText(/Copy this token now/)).toBeTruthy();
	});

	it('keeps password managers out of the token name', () => {
		renderPage('/mcp-admin/tokens');
		fireEvent.click(screen.getByRole('button', { name: /Mint a token/ }));
		const i = dialog().querySelector('input')!;
		expect(i.name).toBe('mcp-token-label');
		expect(i.getAttribute('autocomplete')).toBe('off');
		expect(i.getAttribute('data-1p-ignore')).not.toBeNull();
		expect(i.getAttribute('data-lpignore')).toBe('true');
		expect(i.getAttribute('data-form-type')).toBe('other');
	});
});
