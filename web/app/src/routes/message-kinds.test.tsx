// @vitest-environment happy-dom
// A message's own page for the kinds: the same card as in the chat, the
// interaction it was last seen with, and its service action.
import { beforeEach, describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { FORUM_GROUP, KINDS_GROUP, sampleOf } from '../../standin/kinds.ts';
import { chatStream } from '../../../mockup/src/data/messages.js';
import type { MessageProps } from './message.tsx';

beforeEach(setup);

type Raw = { id: number; message_id: number; chat: { id: number; kind: string } };
const sampleId = (pick: (s: NonNullable<ReturnType<typeof sampleOf>>) => boolean) => (chatStream(KINDS_GROUP) as Raw[]).find((m) => { const s = sampleOf(m); return !!s && pick(s); })!.id;
const open = (id: number, persona: 'superadmin' | 'member' = 'superadmin') => {
	fakeApi(persona);
	renderPage(`/messages/group/${id}`, persona);
	return fixture<MessageProps>(`/messages/group/${id}`, persona).props.message;
};
const kv = (key: string) => screen.getAllByText(key).map((dt) => dt.nextElementSibling?.textContent).find(Boolean);

describe('the message page of a kind', () => {
	it('shows a poll with its reactions, counts and pin', () => {
		const m = open(sampleId((s) => s.content_type === 'poll' && !!s.kinds?.pinned));
		expect(m.detail?.kind).toBe('poll');
		expect(screen.getByText('Which editor do you use?')).toBeTruthy();
		expect(document.querySelectorAll('.a-msginteract .k-reaction')).toHaveLength(4);
		expect(screen.getByText('pinned')).toBeTruthy();
		expect(kv('Type')).toBe('poll');
		expect(kv('Views')).toBe('1,534');
		expect(kv('Forwards')).toBe('12');
		expect(kv('Replies')).toBe('4');
	});

	it('keeps a checklist\'s title in its card', () => {
		const m = open(sampleId((s) => s.content_type === 'checklist'));
		expect(within(document.querySelector('.a-msgkind')!).getByText(m.text)).toBeTruthy();
		expect(document.querySelector('.a-msgtext')).toBeNull();
	});

	it('links a pin to the message it pinned, and names its action', () => {
		const m = open(sampleId((s) => s.content_type === 'service' && s.text === undefined && !!s.service && s.service(({ fetch: false, full: true, user: () => true, group: () => true })).type === 'pin_message'));
		const about = m.service!.refs.message_id!;
		expect(screen.getByRole('link', { name: 'The message this is about' }).getAttribute('href')).toBe(`/groups/${KINDS_GROUP.id}/chat?at=${about}`);
		expect(kv('Service action')).toBe('pin_message');
	});

	it('gives a member a contact without its phone', () => {
		const m = open(sampleId((s) => s.content_type === 'contact'), 'member');
		expect(m.detail).toMatchObject({ kind: 'contact', phone: null });
		expect(screen.queryByText('+62 812 0000 0101')).toBeNull();
	});

	it('links a forum message to its topic', () => {
		const raw = (chatStream(FORUM_GROUP) as Raw[]).at(-1)!;
		const m = open(raw.id);
		expect(m.topic_id).not.toBeNull();
		expect(screen.getByRole('link', { name: `topic ${m.topic_id}` }).getAttribute('href')).toBe(`/groups/${FORUM_GROUP.id}/chat?topic=${m.topic_id}`);
	});
});
