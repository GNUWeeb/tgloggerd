// @vitest-environment happy-dom
// The administration and integration pages: accounts, federations,
// forwarding, Join chat and MCP.
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import type { AccountsProps } from './accounts.tsx';
import type { FederationsProps } from './federations.tsx';
import type { T2DProps } from './forwarding.tsx';
import type { McpGroupsProps } from './mcp.tsx';
import type { PickItem } from '../types.ts';

beforeEach(setup);
const pick = (id: number, title: string, kind: 'group' | 'user' = 'group'): PickItem => ({ id, title, kind });
const dialog = () => screen.getByRole('dialog');
// The open chat picker's list of chats.
const popup = () => waitFor(() => dialog().querySelector('.a-picker-pop') as HTMLElement);
const fill = (root: HTMLElement, sel: string, value: string) => fireEvent.change(root.querySelector(sel)!, { target: { value } });

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
		const props = { ...p, editable: { federations: false, group_grants: false, restrictions: false, abilities: false } };
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

	it('sends an admin\'s restrictions with hidden groups as ids', async () => {
		const p = fixture<AccountsProps>('/settings/accounts').props;
		const admin = p.accounts.find((a) => a.role === 'admin' && (a.restrict?.hide_groups.length ?? 0) > 0)!;
		const calls = fakeApi('superadmin', { 'PATCH /api/accounts/:id': (c) => ({ ...admin, ...(c.body as object) }) });
		renderPage('/settings/accounts');
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
			'GET /api/chats/search': () => ({ items: [pick(-100777, 'Rust Programming')], next_cursor: null }),
			'POST /api/forwarding/telegram-discord': () => json(201, { id: 50 }),
		});
		renderPage('/platform-fwd/telegram-discord');
		fireEvent.click(screen.getByRole('button', { name: /Add webhook/ }));
		fireEvent.click(dialog().querySelector('.a-picker-btn')!);
		fireEvent.click(await within(await popup()).findByRole('button', { name: /Rust Programming/ }));
		fill(dialog(), 'input[type=password]', 'https://discord.com/api/webhooks/1/abc');
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save' }));
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/forwarding/telegram-discord')).toHaveLength(1));
		expect(callsTo(calls, 'POST', '/api/forwarding/telegram-discord')[0]!.body).toMatchObject({ chat_id: -100777, chat_kind: 'group', webhook_url: 'https://discord.com/api/webhooks/1/abc', enabled: true });
	});

	it('shows why a webhook URL is refused', async () => {
		fakeApi('superadmin', {
			'GET /api/chats/search': () => ({ items: [pick(-100777, 'Rust Programming')], next_cursor: null }),
			'POST /api/forwarding/telegram-discord': () => json(400, { error: 'bad_webhook', message: 'That is not a Discord webhook URL.' }),
		});
		renderPage('/platform-fwd/telegram-discord');
		fireEvent.click(screen.getByRole('button', { name: /Add webhook/ }));
		fireEvent.click(dialog().querySelector('.a-picker-btn')!);
		fireEvent.click(await within(await popup()).findByRole('button', { name: /Rust Programming/ }));
		fill(dialog(), 'input[type=password]', 'https://evil.example/x');
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save' }));
		await within(dialog()).findByText('That is not a Discord webhook URL.');
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

describe('Join chat', () => {
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

	it('shows a new token once', async () => {
		fakeApi('superadmin', { 'POST /api/mcp/tokens': () => json(201, { id: 7, token: 'tgmcp_test_once' }) });
		renderPage('/mcp-admin/tokens');
		fireEvent.click(screen.getByRole('button', { name: /Mint a token/ }));
		fill(dialog(), 'input', 'ci');
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Mint' }));
		await screen.findByText('tgmcp_test_once');
		expect(screen.getByText(/Copy this token now/)).toBeTruthy();
	});
});
