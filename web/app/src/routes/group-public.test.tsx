// @vitest-environment happy-dom
// An exposed group's public page: for a signed-out visitor, and for a
// signed-in viewer whose scope does not see the group.
import { beforeEach, describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { fmt } from '../format.ts';
import type { GroupsProps } from './groups.tsx';
import type { GroupProps, GroupPublicProps } from './group.tsx';
import type { PageData } from '../page.ts';

beforeEach(setup);

const exposed = () => fixture<GroupsProps>('/groups').props.rows.find((g) => g.exposed)!;
// The public answer, cut from the full page data as the server cuts it.
function publicData(persona: 'public' | 'member'): Partial<PageData> {
	const g = fixture<GroupProps>(`/groups/${exposed().id}`).props.group;
	const props: GroupPublicProps = { public: true, group: { id: g.id, title: g.title, username: g.username, usernames: g.usernames, type: g.type, photo: g.photo, description: g.description, members: g.members, msg_count: g.msg_count, created_at: g.created_at, exposed: true } };
	return { route: 'group', status: 200, props, viewer: fixture('/', persona === 'public' ? 'public' : 'member').viewer };
}

describe('public group page', () => {
	it('shows a signed-out visitor the group, its description and counts, and its chat', () => {
		fakeApi('public');
		const d = publicData('public');
		const g = (d.props as GroupPublicProps).group;
		renderPage(`/groups/${g.id}`, 'public', d);
		expect(screen.getByRole('heading', { level: 1 }).textContent).toBe(g.title);
		expect(screen.getByText(fmt(g.msg_count))).toBeTruthy();
		expect(screen.getByText(fmt(g.members))).toBeTruthy();
		if (g.description) expect(screen.getByText(g.description)).toBeTruthy();
		expect(screen.getByRole('link', { name: /Open chat/ }).getAttribute('href')).toBe(`/groups/${g.id}/chat`);
		expect(screen.getByRole('link', { name: /Sign in/ }).getAttribute('href')).toBe(`/login?next=${encodeURIComponent(`/groups/${g.id}`)}`);
		// Nothing of the full page: no tabs, admins, history or pin.
		expect(screen.queryByRole('link', { name: /Admins/ })).toBeNull();
		expect(screen.queryByText(/Recent changes|Most active members/)).toBeNull();
		expect(document.querySelector('.a-sidebar, nav.a-nav')).toBeNull();
	});

	it('shows a member without the group the same page inside the usual shell', () => {
		fakeApi('member');
		const d = publicData('member');
		const g = (d.props as GroupPublicProps).group;
		renderPage(`/groups/${g.id}`, 'member', d);
		expect(screen.getByRole('heading', { level: 1 }).textContent).toBe(g.title);
		expect(screen.queryByRole('link', { name: /Sign in/ })).toBeNull();
		expect(within(document.querySelector('.a-publicgroup')!).getByRole('link', { name: /Open chat/ })).toBeTruthy();
	});

	it('links the public chat\'s top bar to the group\'s public page', () => {
		const g = exposed();
		fakeApi('public');
		renderPage(`/groups/${g.id}/chat`, 'public');
		const a = document.querySelector<HTMLAnchorElement>('.c-head a.c-head-link')!;
		expect(a.getAttribute('href')).toBe(`/groups/${g.id}`);
	});
});
