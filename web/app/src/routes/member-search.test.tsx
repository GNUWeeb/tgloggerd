// @vitest-environment happy-dom
// The Members tab's search: a box above the list, kept in the URL as
// ?q=, whose results come with the page data and page with Load more.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { MEMBER_SAMPLES as M, membersApi } from '../../standin/members.ts';
import type { GroupMember } from '../api/members.ts';
import type { GroupMembersProps } from './group.tsx';

beforeEach(setup);

const BASE = `/groups/${M.LONG}/members`;
const PAGE = `/api/page${BASE}`;
const box = () => screen.queryByRole('searchbox', { name: 'Search members by name, @username or ID' }) as HTMLInputElement | null;
const names = () => [...document.querySelectorAll('.a-table tbody tr .a-uname')].map((e) => e.textContent);
const answer = ({ query }: { query: URLSearchParams }) => membersApi(String(M.LONG), query, 'superadmin', () => true)[1];
const pause = (ms: number) => new Promise((r) => setTimeout(r, ms));
// Every member of the long sample list, to pick search words from.
const every = (): GroupMember[] => [
	...fixture<GroupMembersProps>(BASE).props.members,
	...(membersApi(String(M.LONG), new URLSearchParams('offset=50&limit=200'), 'superadmin', () => true)[1] as { members: GroupMember[] }).members,
];

describe('member search', () => {
	it('fetches one page of results for a burst of typing, keeping q in the URL in place', async () => {
		const u = every().find((m) => m.user.username)!.user;
		const calls = fakeApi('superadmin');
		renderPage(BASE);
		const before = history.length;
		for (let i = 1; i <= u.username.length; i++) fireEvent.change(box()!, { target: { value: '@' + u.username.slice(0, i) } });
		await waitFor(() => expect(callsTo(calls, 'GET', PAGE)).toHaveLength(1));
		expect(callsTo(calls, 'GET', PAGE)[0]!.query.get('q')).toBe('@' + u.username);
		await pause(400);
		expect(callsTo(calls, 'GET', PAGE)).toHaveLength(1);
		await waitFor(() => expect(location.search).toBe(`?q=${encodeURIComponent('@' + u.username)}`));
		// The entry was replaced: Back does not step through the typing.
		expect(history.length).toBe(before);
		// The results replace the list.
		await waitFor(() => expect(names()).toContain(u.name));
		expect(names().length).toBeLessThan(50);
		expect(box()!.value).toBe('@' + u.username);
	});

	it('opens with the search of the URL, and pages within it with q and offset', async () => {
		const calls = fakeApi('superadmin', { 'GET /api/groups/:id/members': answer });
		const p = fixture<GroupMembersProps>(`${BASE}?q=a`).props;
		expect(p.total).toBeGreaterThan(50);
		renderPage(`${BASE}?q=a`);
		expect(box()!.value).toBe('a');
		expect(names()).toHaveLength(50);
		fireEvent.click(screen.getByRole('button', { name: 'Load more' }));
		await waitFor(() => expect(callsTo(calls, 'GET', `/api/groups/${M.LONG}/members`)).toHaveLength(1));
		const asked = callsTo(calls, 'GET', `/api/groups/${M.LONG}/members`)[0]!.query;
		expect(asked.get('q')).toBe('a');
		expect(asked.get('offset')).toBe('50');
		await waitFor(() => expect(names().length).toBe(Math.min(p.total!, 100)));
		await pause(400);
		expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(0);
	});

	it('pages after the search matches, not the id match that comes in addition to limit', async () => {
		const [a, b, c, d, e] = every();
		const tag = (m: GroupMember, by: 'id' | 'search') => ({ ...m, matched_by: by });
		const p = fixture<GroupMembersProps>(BASE).props;
		const calls = fakeApi('superadmin', { 'GET /api/groups/:id/members': () => ({ ...p, total: 5, offset: 2, limit: 2, members: [tag(d!, 'search'), tag(e!, 'search')] }) });
		renderPage(`${BASE}?q=7`, 'superadmin', { props: { ...p, total: 5, offset: 0, limit: 2, members: [tag(a!, 'id'), tag(b!, 'search'), tag(c!, 'search')] } as GroupMembersProps });
		expect(names()).toHaveLength(3);
		fireEvent.click(screen.getByRole('button', { name: 'Load more' }));
		await waitFor(() => expect(names()).toHaveLength(5));
		expect(callsTo(calls, 'GET', `/api/groups/${M.LONG}/members`)[0]!.query.get('offset')).toBe('2');
		// All 5 matches shown: nothing more to load.
		expect(screen.queryByRole('button', { name: 'Load more' })).toBeNull();
	});

	it('says when nothing matches, and clears the search', async () => {
		const calls = fakeApi('superadmin');
		renderPage(`${BASE}?q=zzzqqqxxx`);
		expect(document.querySelector('.a-empty')!.textContent).toContain('No member matches zzzqqqxxx.');
		expect(document.querySelector('.a-table')).toBeNull();
		fireEvent.click(within(document.querySelector<HTMLElement>('.a-empty')!).getByRole('button', { name: 'Clear search' }));
		await waitFor(() => expect(callsTo(calls, 'GET', PAGE)).toHaveLength(1));
		expect(callsTo(calls, 'GET', PAGE)[0]!.query.get('q')).toBeNull();
		await waitFor(() => expect(names()).toHaveLength(50));
		expect(box()!.value).toBe('');
		expect(location.search).toBe('');
	});

	it('marks a member found by its id', async () => {
		const last = every().at(-1)!.user;
		fakeApi('superadmin');
		renderPage(`${BASE}?q=${last.id}`);
		const row = document.querySelector('.a-table tbody tr')!;
		expect(row.textContent).toContain('matched by ID');
		expect(row.querySelector('.a-uname')!.textContent).toBe(`User ${last.id}`);
		// Members found by the name search carry no mark.
		const r2 = fixture<GroupMembersProps>(`${BASE}?q=a`);
		expect(r2.props.members.every((m) => m.matched_by === 'search')).toBe(true);
	});

	it('clears the box with Escape', async () => {
		const calls = fakeApi('superadmin');
		renderPage(`${BASE}?q=a`);
		fireEvent.keyDown(box()!, { key: 'Escape' });
		expect(box()!.value).toBe('');
		await waitFor(() => expect(callsTo(calls, 'GET', PAGE)).toHaveLength(1));
		expect(callsTo(calls, 'GET', PAGE)[0]!.query.get('q')).toBeNull();
		await pause(400);
		expect(callsTo(calls, 'GET', PAGE)).toHaveLength(1);
	});

	it('has no box when the list is not available', () => {
		fakeApi('admin');
		const r = renderPage(BASE, 'admin');
		expect(box()).toBeTruthy();
		r.unmount();
		renderPage(`/groups/${M.NOT_ALLOWED}/members`, 'admin');
		expect(document.querySelector('.a-membersoff')).toBeTruthy();
		expect(box()).toBeNull();
	});
});
