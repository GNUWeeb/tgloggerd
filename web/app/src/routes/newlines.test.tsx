// @vitest-environment happy-dom
// Descriptions and bios keep their line breaks, as written on Telegram.
import { beforeEach, describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { fixture, renderPage, setup } from '../test/page.tsx';
import type { GroupProps } from './group.tsx';
import type { GroupsProps } from './groups.tsx';
import type { UserProps } from './user.tsx';
import type { UsersProps } from './users.tsx';

beforeEach(setup);
const text = 'PM @someone to ask for approval to join.\n\nWe build community through open-source technology.\n\nhttps://www.gnuweeb.org';
const shown = (t: string) => [...document.querySelectorAll('.a-multiline')].find((e) => e.textContent === t) as HTMLElement | undefined;

describe('line breaks', () => {
	it('keeps a group\'s description as written', () => {
		const id = fixture<GroupsProps>('/groups').props.rows[0]!.id;
		const p = fixture<GroupProps>(`/groups/${id}`).props;
		renderPage(`/groups/${id}`, 'superadmin', { props: { ...p, group: { ...p.group, description: text } } });
		expect(shown(text)).toBeTruthy();
	});

	it('keeps a user\'s bio as written', () => {
		const id = fixture<UsersProps>('/users').props.rows[0]!.id;
		const p = fixture<Extract<UserProps, { limited?: false }>>(`/users/${id}`).props;
		renderPage(`/users/${id}`, 'superadmin', { props: { ...p, user: { ...p.user, bio: text } } });
		expect(shown(text)).toBeTruthy();
	});

	it('keeps them in the history of description changes', () => {
		const id = fixture<GroupsProps>('/groups').props.rows[0]!.id;
		const p = fixture<GroupProps>(`/groups/${id}/history`).props;
		renderPage(`/groups/${id}/history`, 'superadmin', { props: { ...p, history: [{ kind: 'description', at: '2026-11-10T10:00:00Z', detail: text }], history_next: null } });
		expect(shown(text)).toBeTruthy();
	});

	it('keeps them in the groups list, the first lines shown', () => {
		const p = fixture<GroupsProps>('/groups').props;
		renderPage('/groups', 'superadmin', { props: { ...p, rows: [{ ...p.rows[0]!, description: text }, ...p.rows.slice(1)] } });
		const cell = [...document.querySelectorAll('.a-longtext')].find((e) => e.textContent === text);
		expect(cell).toBeTruthy();
		expect(screen.getAllByRole('button', { name: /^Show all of Description/ }).length).toBeGreaterThan(0);
	});
});
