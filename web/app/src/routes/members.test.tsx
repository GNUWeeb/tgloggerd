// @vitest-environment happy-dom
// A group's Telegram members: the member count in the chat's head and
// on the group page, and the group page's Members tab.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { MEMBER_SAMPLES as M, membersApi } from '../../standin/members.ts';
import { compact, fmt } from '../format.ts';
import type { GroupMembersProps, GroupProps } from './group.tsx';
import type { GroupsProps } from './groups.tsx';

beforeEach(setup);
const sub = () => document.querySelector('.c-head .c-sub')!.textContent;
const statLabels = () => [...document.querySelectorAll('.a-stat > .a-muted:first-child')].map((e) => e.textContent);

describe('member count', () => {
	it('shows Telegram\'s count in a group chat\'s head, before the members seen', () => {
		const g = fixture<GroupProps>(`/groups/${M.LONG}`).props.group;
		fakeApi('superadmin');
		renderPage(`/groups/${M.LONG}/chat`);
		expect(g.member_count).toBeGreaterThan(0);
		expect(sub()).toBe(`supergroup . ${fmt(g.member_count!)} members . ${fmt(g.members)} members seen . ${fmt(g.msg_count)} messages`);
	});

	it('leaves the count out of the head while it is not known', () => {
		const g = fixture<GroupProps>(`/groups/${M.NO_COUNT}`).props.group;
		fakeApi('superadmin');
		renderPage(`/groups/${M.NO_COUNT}/chat`);
		expect(sub()).toBe(`supergroup . ${fmt(g.members)} members seen . ${fmt(g.msg_count)} messages`);
	});

	it('shows the count on the group page beside the members seen', () => {
		const g = fixture<GroupProps>(`/groups/${M.LONG}`).props.group;
		fakeApi('superadmin');
		renderPage(`/groups/${M.LONG}`);
		expect(document.querySelector('.a-ph-meta')!.textContent).toContain(`${fmt(g.member_count!)} members`);
		expect(statLabels()).toEqual(expect.arrayContaining(['Members', 'Members seen']));
		const value = (label: string) => [...document.querySelectorAll('.a-stat')].find((e) => e.firstElementChild?.textContent === label)!.querySelector('b')!.textContent;
		expect(value('Members')).toBe(fmt(g.member_count!));
		expect(value('Members seen')).toBe(fmt(g.members));
	});

	it('shows only the members seen on the group page while the count is not known', () => {
		fakeApi('superadmin');
		renderPage(`/groups/${M.NO_COUNT}`);
		expect(statLabels()).toContain('Members seen');
		expect(statLabels()).not.toContain('Members');
		expect(document.querySelector('.a-ph-meta')!.textContent).not.toMatch(/members/);
		expect(screen.getByRole('heading', { level: 1 })).toBeTruthy();
	});
});

const tab = () => screen.queryByRole('link', { name: /^Members/ });
const table = () => document.querySelector<HTMLTableElement>('.a-table')!;
const rows = () => [...table().querySelectorAll('tbody tr')];
// The stand-in's answer to the members endpoint, as the service sends it.
const answer = (id: number) => ({ query }: { query: URLSearchParams }) => membersApi(String(id), query, 'superadmin', () => true)[1];

describe('Members tab', () => {
	it('is there, with the count, for viewers who see users in full', async () => {
		const g = fixture<GroupProps>(`/groups/${M.LONG}`).props.group;
		for (const who of ['superadmin', 'admin'] as const) {
			const calls = fakeApi(who);
			const r = renderPage(`/groups/${M.LONG}`, who);
			expect(tab()!.getAttribute('href')).toBe(`/groups/${M.LONG}/members`);
			expect(tab()!.querySelector('.a-tabcount')!.textContent).toBe(compact(g.member_count!));
			fireEvent.click(tab()!);
			await waitFor(() => expect(callsTo(calls, 'GET', `/api/page/groups/${M.LONG}/members`)).toHaveLength(1));
			r.unmount();
		}
	});

	it('is not there for a member, who gets no list', () => {
		const id = fixture<GroupsProps>('/groups', 'member').props.rows[0]!.id;
		fakeApi('member');
		renderPage(`/groups/${id}`, 'member');
		expect(screen.getByRole('link', { name: /^Admins/ })).toBeTruthy();
		expect(tab()).toBeNull();
	});

	it('lists the members with their status and links to their pages', () => {
		const p = fixture<GroupMembersProps>(`/groups/${M.LONG}/members`).props;
		fakeApi('superadmin');
		renderPage(`/groups/${M.LONG}/members`);
		expect(tab()!.getAttribute('aria-current')).toBe('page');
		expect(rows()).toHaveLength(p.members.length);
		const first = p.members[0]!;
		const link = within(rows()[0] as HTMLElement).getByRole('link');
		expect(link.getAttribute('href')).toBe(`/users/${first.user.id}`);
		expect(link.textContent).toContain(first.user.name);
		const chip = (s: string) => rows().filter((r, i) => p.members[i]!.status === s).map((r) => r.children[1]!.textContent);
		expect(new Set(chip('creator'))).toEqual(new Set(['owner']));
		expect(new Set(chip('administrator'))).toEqual(new Set(['admin']));
		expect(new Set(chip('restricted'))).toEqual(new Set(['restricted']));
		expect(new Set(chip('member'))).toEqual(new Set(['']));
	});

	it('pages the list by offset with Load more, to its end', async () => {
		const calls = fakeApi('superadmin', { 'GET /api/groups/:id/members': answer(M.LONG) });
		renderPage(`/groups/${M.LONG}/members`);
		expect(rows()).toHaveLength(50);
		fireEvent.click(screen.getByRole('button', { name: 'Load more' }));
		await waitFor(() => expect(rows()).toHaveLength(100));
		const asked = callsTo(calls, 'GET', `/api/groups/${M.LONG}/members`);
		expect(asked).toHaveLength(1);
		expect(asked[0]!.query.get('offset')).toBe('50');
		expect(asked[0]!.query.get('limit')).toBe('50');
		fireEvent.click(screen.getByRole('button', { name: 'Load more' }));
		await waitFor(() => expect(callsTo(calls, 'GET', `/api/groups/${M.LONG}/members`)).toHaveLength(2));
		expect(callsTo(calls, 'GET', `/api/groups/${M.LONG}/members`)[1]!.query.get('offset')).toBe('100');
		// The last page is short: no more to load. Its last member is not
		// stored yet and is named by its id.
		await waitFor(() => expect(screen.queryByRole('button', { name: 'Load more' })).toBeNull());
		const last = rows().at(-1)!;
		expect(last.querySelector('.a-uname')!.textContent).toMatch(/^User 77\d+$/);
		expect(screen.getByText(/Telegram lists no more|End of the list/)).toBeTruthy();
		// Nothing asked for the page data again.
		expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(0);
	});

	it.each([
		[M.NOT_ALLOWED, "Telegram doesn't show this group's members to the archive account"],
		[M.NOT_MEMBER, "Telegram doesn't show this group's members to the archive account"],
		[M.FLOOD, 'Telegram asked to wait; try again later'],
	])('says why a list is not available (%s), with the count', (id, why) => {
		const p = fixture<GroupMembersProps>(`/groups/${id}/members`, 'admin').props;
		fakeApi('admin');
		renderPage(`/groups/${id}/members`, 'admin');
		const box = document.querySelector('.a-membersoff')!;
		expect(box.textContent).toContain(`${fmt(p.total!)} members`);
		expect(box.textContent).toContain('Member list not available');
		expect(box.textContent).toContain(why);
		expect(document.querySelector('.a-table')).toBeNull();
	});

	it('says a superadmin hid the list, to an admin', () => {
		fakeApi('admin');
		renderPage(`/groups/${M.HIDDEN}/members`, 'admin');
		expect(document.querySelector('.a-membersoff')!.textContent).toContain('Hidden by a superadmin');
	});

	it('says without a count when neither the list nor the group has one', () => {
		const p = fixture<GroupMembersProps>(`/groups/${M.FLOOD}/members`, 'admin');
		fakeApi('admin');
		renderPage(`/groups/${M.FLOOD}/members`, 'admin', { props: { ...p.props, total: null, group: { ...p.props.group, member_count: null } } });
		expect(document.querySelector('.a-membersoff b')!.textContent).toBe('Member list not available');
	});
});
