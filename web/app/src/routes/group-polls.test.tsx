// @vitest-environment happy-dom
// A group's Polls tab: the chat's poll card for each poll, newest
// first, with its sender, date and both message links, and older pages
// appended from GET /api/groups/{id}/polls.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { FILES_GROUP, SOME_KINDS_GROUP } from '../../standin/groupfiles.ts';
import type { GroupPollsPage } from '../api/groupfiles.ts';

beforeEach(setup);
const pollsUrl = (id: number) => `/groups/${id}/polls`;
const cards = () => [...document.querySelectorAll<HTMLElement>('.a-pollcard')];

describe('group polls tab', () => {
	it('is a tab of the group page', () => {
		renderPage(`/groups/${FILES_GROUP.id}`);
		expect(within(document.querySelector<HTMLElement>('.a-tabs')!).getByRole('link', { name: /^Polls/ }).getAttribute('href')).toBe(pollsUrl(FILES_GROUP.id));
	});

	it('draws each poll with the chat\'s card, its sender, date and links', () => {
		const p = fixture<GroupPollsPage>(pollsUrl(FILES_GROUP.id)).props;
		renderPage(pollsUrl(FILES_GROUP.id));
		expect(cards()).toHaveLength(p.rows.length);
		expect(cards().map((c) => c.querySelector('.k-question')!.textContent)).toEqual(p.rows.map((r) => r.poll.question));
		cards().forEach((c, i) => {
			const r = p.rows[i]!;
			expect(c.querySelector('.k-poll')).not.toBeNull();
			expect(within(c).getByRole('link', { name: 'In the chat' }).getAttribute('href')).toBe(r.chat_link);
			expect(within(c).getByRole('link', { name: 'Message' }).getAttribute('href')).toBe(`/messages/group/${r.message_row_id}`);
			expect(c.querySelector('time')!.getAttribute('datetime')).toBe(r.date);
			if (r.sender) expect(within(c).getByRole('link', { name: r.sender.name }).getAttribute('href')).toBe(`/users/${r.sender.id}`);
		});
		// The closed quiz marks its correct option; the long poll says how
		// many options it leaves out.
		const quiz = cards()[p.rows.findIndex((r) => r.poll.type === 'quiz')]!;
		expect(within(quiz).getByText(/Quiz . closed/)).toBeTruthy();
		expect(within(quiz).getByLabelText('correct answer')).toBeTruthy();
		const long = cards()[p.rows.findIndex((r) => r.poll.options_total > 20)]!;
		expect(within(long).getByText(/5 more options not shown/)).toBeTruthy();
		// One page: nothing more to load.
		expect(screen.queryByRole('button', { name: 'Load more' })).toBeNull();
		expect(screen.getByText(`All ${p.rows.length} polls shown`)).toBeTruthy();
	});

	it('appends the next page from next_before', async () => {
		const p = fixture<GroupPollsPage>(pollsUrl(FILES_GROUP.id)).props;
		const [last, ...rest] = [...p.rows].reverse();
		const firstPage = { ...p, rows: rest.reverse(), next_before: rest[0]!.message_id };
		const calls = fakeApi('superadmin', { 'GET /api/groups/:id/polls': () => ({ rows: [last], next_before: null }) });
		renderPage(pollsUrl(FILES_GROUP.id), 'superadmin', { props: firstPage });
		expect(cards()).toHaveLength(p.rows.length - 1);
		expect(within(document.querySelector<HTMLElement>('.a-tabs')!).getByRole('link', { name: /^Polls/ }).textContent).toBe(`Polls${p.rows.length - 1}+`);
		fireEvent.click(screen.getByRole('button', { name: 'Load more' }));
		await waitFor(() => expect(cards()).toHaveLength(p.rows.length));
		expect(cards().at(-1)!.querySelector('.k-question')!.textContent).toBe(last!.poll.question);
		const call = callsTo(calls, 'GET', `/api/groups/${FILES_GROUP.id}/polls`)[0]!;
		expect(call.query.get('before')).toBe(String(firstPage.next_before));
		expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(0);
	});

	it('says when a group has no polls', () => {
		renderPage(pollsUrl(SOME_KINDS_GROUP.id));
		expect(screen.getByText('No polls in this group.')).toBeTruthy();
	});
});
