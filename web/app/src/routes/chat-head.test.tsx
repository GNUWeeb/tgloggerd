// @vitest-environment happy-dom
// A chat's top bar opens the chat's details: the group's page, or the
// profile of the user a private chat is with.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import type { GroupsProps } from './groups.tsx';
import type { MessagesProps } from './messages.tsx';

beforeEach(setup);
const head = () => document.querySelector<HTMLAnchorElement>('.c-head a.c-head-link');

describe('chat top bar', () => {
	it('links a group chat to the group\'s page', async () => {
		const g = fixture<GroupsProps>('/groups').props.rows[0]!;
		const calls = fakeApi('superadmin');
		renderPage(`/groups/${g.id}/chat`);
		const a = head()!;
		expect(a.getAttribute('href')).toBe(`/groups/${g.id}`);
		expect(a.querySelector('h1')!.textContent).toBe(g.title);
		expect(a.title).toBe('Group details');
		fireEvent.click(a);
		await waitFor(() => expect(callsTo(calls, 'GET', `/api/page/groups/${g.id}`)).toHaveLength(1));
	});

	it('links a private chat to the profile of the user it is with', () => {
		const id = fixture<MessagesProps>('/private_messages').props.rows[0]!.chat.id;
		fakeApi('superadmin');
		renderPage(`/users/${id}/chat`);
		expect(head()!.getAttribute('href')).toBe(`/users/${id}`);
		expect(head()!.title).toBe('Profile');
		expect(screen.getByRole('heading', { level: 1 }).closest('a')).toBe(head());
	});
});
