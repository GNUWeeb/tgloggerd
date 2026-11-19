// @vitest-environment happy-dom
// A group's Telegram members: the member count in the chat's head and
// on the group page.
import { beforeEach, describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { MEMBER_SAMPLES as M } from '../../standin/members.ts';
import { fmt } from '../format.ts';
import type { GroupProps } from './group.tsx';

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
