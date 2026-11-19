// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import type { JoinChatProps } from './joinchat.tsx';

beforeEach(setup);

const chat = { chat_id: -1001999000001, title: 'Compiler Nerds ID', type: 'supergroup', member_count: 1843, username: 'compilernerds_id', is_member: false, needs_approval: false };
const card = (title: string) => screen.getByRole('heading', { name: title }).closest('.a-card') as HTMLElement;

// Preview, tick the first `n` federations, join.
async function joinWith(n: number) {
	fireEvent.change(screen.getByPlaceholderText('@groupname or https://t.me/+...'), { target: { value: '@compilernerds_id' } });
	fireEvent.click(screen.getByRole('button', { name: 'Preview' }));
	await screen.findByText(/Join this chat\?/);
	const chips = [...document.querySelectorAll<HTMLInputElement>('label.a-checkchip input')];
	for (const c of chips.slice(0, n)) fireEvent.click(c);
	fireEvent.click(screen.getByRole('button', { name: 'Join' }));
}

describe('Join chat federations', () => {
	it('shows the federations the answer filed the group in, and reloads the lists', async () => {
		const feds = fixture<JoinChatProps>('/join-chat').props.federations;
		const calls = fakeApi('superadmin', {
			'POST /api/join-chat/preview': () => ({ outcome: 'ok', message: '', chat }),
			'POST /api/join-chat/join': (c) => ({ outcome: 'joined', message: 'Joined.', chat: { ...chat, is_member: true }, federation_ids: (c.body as { federation_ids: number[] }).federation_ids, filing_pending: false }),
		});
		renderPage('/join-chat');
		await joinWith(2);
		await screen.findByText('Joined.');
		const filed = screen.getByRole('list', { name: 'Filed in' });
		expect(within(filed).getAllByRole('listitem').map((li) => li.textContent)).toEqual([feds[0]!.name, feds[1]!.name]);
		expect(screen.queryByText(/The filing waits/)).toBeNull();
		// The choice is made: no chips to change it after the join.
		expect(document.querySelector('label.a-checkchip')).toBeNull();
		await waitFor(() => expect(callsTo(calls, 'GET', '/api/page/join-chat')).toHaveLength(1));
	});

	it('says a filing waits when the join only sent a request', async () => {
		const feds = fixture<JoinChatProps>('/join-chat').props.federations;
		fakeApi('superadmin', {
			'POST /api/join-chat/preview': () => ({ outcome: 'ok', message: '', chat: { ...chat, needs_approval: true } }),
			'POST /api/join-chat/join': () => ({ outcome: 'request_sent', message: 'Join request sent.', chat, federation_ids: [feds[0]!.id], filing_pending: true }),
		});
		renderPage('/join-chat');
		await joinWith(1);
		await screen.findByText('Join request sent.');
		expect(within(screen.getByRole('list', { name: 'To be filed in' })).getByText(feds[0]!.name)).toBeTruthy();
		expect(screen.getByText(/The filing waits/)).toBeTruthy();
	});

	it('shows no federations when none was chosen', async () => {
		fakeApi('superadmin', {
			'POST /api/join-chat/preview': () => ({ outcome: 'ok', message: '', chat }),
			'POST /api/join-chat/join': () => ({ outcome: 'joined', message: 'Joined.', chat: { ...chat, is_member: true }, federation_ids: [], filing_pending: false }),
		});
		renderPage('/join-chat');
		await joinWith(0);
		await screen.findByText('Joined.');
		expect(screen.queryByRole('list', { name: /filed in/i })).toBeNull();
	});
});

describe('Join chat pending filings', () => {
	it('lists every held filing to a superadmin: title or ref, federation, who', () => {
		const { pending } = fixture<JoinChatProps>('/join-chat').props;
		expect(new Set(pending.map((f) => f.by)).size).toBeGreaterThan(1);
		renderPage('/join-chat');
		const rows = within(card('Pending filings')).getAllByRole('listitem');
		expect(rows).toHaveLength(pending.length);
		for (const [i, f] of pending.entries()) {
			expect(rows[i]!.textContent).toContain(f.title || f.ref);
			expect(rows[i]!.textContent).toContain(f.federation.name);
			expect(rows[i]!.textContent).toContain(`by ${f.by}`);
			expect(within(rows[i]!).getByRole('button', { name: `Cancel filing ${f.title || f.ref} into ${f.federation.name}` })).toBeTruthy();
		}
	});

	it('lists only an admin\'s own', () => {
		const { pending } = fixture<JoinChatProps>('/join-chat', 'admin').props;
		expect(pending.length).toBeGreaterThan(0);
		renderPage('/join-chat', 'admin');
		const rows = within(card('Pending filings')).getAllByRole('listitem');
		expect(rows).toHaveLength(pending.length);
		expect(rows.every((r) => r.textContent!.includes('by gw_admin'))).toBe(true);
	});

	it('shows no list when nothing is held', () => {
		const p = fixture<JoinChatProps>('/join-chat').props;
		renderPage('/join-chat', 'superadmin', { props: { ...p, pending: [] } });
		expect(screen.queryByRole('heading', { name: 'Pending filings' })).toBeNull();
	});

	it('cancels one only after confirming, then reloads the list', async () => {
		const f = fixture<JoinChatProps>('/join-chat').props.pending[0]!;
		vi.stubGlobal('confirm', vi.fn().mockReturnValueOnce(false).mockReturnValueOnce(true));
		const calls = fakeApi('superadmin', { 'DELETE /api/join-chat/pending/:id': () => undefined });
		renderPage('/join-chat');
		const cancel = () => fireEvent.click(screen.getByRole('button', { name: `Cancel filing ${f.title || f.ref} into ${f.federation.name}` }));
		cancel();
		expect(callsTo(calls, 'DELETE', /^\/api\/join-chat\/pending\//)).toHaveLength(0);
		cancel();
		await waitFor(() => expect(callsTo(calls, 'DELETE', `/api/join-chat/pending/${f.id}`)).toHaveLength(1));
		expect(callsTo(calls, 'DELETE', `/api/join-chat/pending/${f.id}`)[0]!.headers.get('x-csrf-token')).toBe('test-csrf');
		await waitFor(() => expect(callsTo(calls, 'GET', '/api/page/join-chat')).toHaveLength(1));
	});

	it('says why a cancel failed', async () => {
		const f = fixture<JoinChatProps>('/join-chat').props.pending[0]!;
		vi.stubGlobal('confirm', vi.fn().mockReturnValue(true));
		fakeApi('superadmin', { 'DELETE /api/join-chat/pending/:id': () => json(404, { error: 'not_found', message: 'No such held filing.' }) });
		renderPage('/join-chat');
		fireEvent.click(screen.getByRole('button', { name: `Cancel filing ${f.title || f.ref} into ${f.federation.name}` }));
		expect((await within(card('Pending filings')).findByRole('alert')).textContent).toBe('No such held filing.');
	});
});

describe('Join chat recent attempts', () => {
	// Every outcome a join can end in, in words: the daemon's, and the
	// audit's own when the daemon gave none.
	const WORDS: [string, string][] = [
		['joined', 'joined'], ['already_member', 'already a member'], ['request_sent', 'request sent'],
		['approval_required', 'approval required'], ['declined', 'declined'], ['not_a_group', 'not a group'],
		['not_found', 'not found'], ['link_invalid', 'invalid link'], ['flood_wait', 'rate limited'],
		['banned', 'banned'], ['too_many_chats', 'too many chats'], ['error', 'error'],
		['api_off', 'API not set up'], ['no_answer', 'no answer'], ['http_502', 'API error 502'],
	];
	it('names each outcome in words', () => {
		const p = fixture<JoinChatProps>('/join-chat').props;
		const recent = WORDS.map(([outcome], i) => ({ at: '2026-11-17T10:00:00Z', by: 'admin', ref: `@chat_${i}`, title: `Chat ${i}`, outcome }));
		renderPage('/join-chat', 'superadmin', { props: { ...p, recent } });
		const rows = within(card('Recent join attempts')).getAllByRole('listitem');
		expect(rows.map((r) => r.querySelector('.a-pill')!.textContent)).toEqual(WORDS.map(([, w]) => w));
	});
});
