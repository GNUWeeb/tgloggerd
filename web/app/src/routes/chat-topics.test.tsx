// @vitest-environment happy-dom
// A forum group's chat: its topics above the messages, each message's
// topic while all are shown, and one topic at a time through ?topic=.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { FORUM_GROUP, KINDS_GROUP, TOPICS } from '../../standin/kinds.ts';
import type { ChatProps } from './chat.tsx';

beforeEach(setup);
const url = `/groups/${FORUM_GROUP.id}/chat`;
const strip = () => screen.getByRole('navigation', { name: 'Forum topics' });

describe('forum topics', () => {
	it('lists the topics, and names each message\'s topic while all are shown', () => {
		fakeApi('superadmin');
		renderPage(url);
		const links = within(strip()).getAllByRole('link');
		expect(links.map((a) => a.textContent)).toEqual(['All topics', ...TOPICS.map((t) => t.name)]);
		expect(links[0]!.getAttribute('aria-current')).toBe('page');
		expect(within(strip()).getByLabelText('closed').closest('a')!.textContent).toBe('Off-topic');
		const p = fixture<ChatProps>(url).props;
		const m = p.messages.find((x) => x.content_type !== 'service' && x.topic_id !== null)!;
		const tag = document.getElementById(`m${m.message_id}`)!.querySelector('.k-topictag')!;
		expect(tag.textContent).toBe(TOPICS.find((t) => t.id === m.topic_id)!.name);
		expect(tag.getAttribute('href')).toBe(`${url}?topic=${m.topic_id}`);
	});

	it('opens one topic, and keeps loading older messages within it', async () => {
		const t = TOPICS[1]!;
		const calls = fakeApi('superadmin', { 'GET /api/chats/group/:id/messages': () => ({ messages: [], has_older: false, has_newer: false }) });
		const page = fixture<ChatProps>(`${url}?topic=${t.id}`);
		expect(page.props.topic).toBe(t.id);
		expect(page.props.messages.every((m) => m.topic_id === t.id)).toBe(true);
		renderPage(`${url}?topic=${t.id}`, 'superadmin', { viewer: { ...page.viewer, chat: { ...page.viewer.chat, auto_load: false } } });
		expect(within(strip()).getByRole('link', { name: t.name }).getAttribute('aria-current')).toBe('page');
		// One topic shown: no message needs a tag.
		expect(document.querySelector('.k-topictag')).toBeNull();
		fireEvent.click(screen.getByRole('button', { name: 'Load older messages' }));
		await waitFor(() => expect(callsTo(calls, 'GET', /\/api\/chats\/group\/-?\d+\/messages/)).toHaveLength(1));
		expect(callsTo(calls, 'GET', /\/api\/chats\/group\/-?\d+\/messages/)[0]!.query.get('topic')).toBe(String(t.id));
	});

	it('shows no topics for a chat that is not a forum', () => {
		fakeApi('superadmin');
		renderPage(`/groups/${KINDS_GROUP.id}/chat`);
		expect(screen.queryByRole('navigation', { name: 'Forum topics' })).toBeNull();
	});
});
