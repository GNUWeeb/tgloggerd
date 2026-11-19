// @vitest-environment happy-dom
// Text entities where the chat shows texts: bubbles, the edit history,
// polls and checklists, from the stand-in's formatted samples
// (standin/kinds.ts).
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { KINDS_GROUP } from '../../standin/kinds.ts';
import type { ChatProps } from './chat.tsx';
import type { ChatMessage } from '../types.ts';
import type { HasEntities, TextEntity } from '../api/entities.ts';

beforeEach(setup);

type Persona = 'superadmin' | 'member' | 'public';
const url = `/groups/${KINDS_GROUP.id}/chat`;
const msgs = (p: Persona = 'superadmin') => fixture<ChatProps>(url, p).props.messages;
const openChat = (p: Persona = 'superadmin') => {
	const calls = fakeApi(p);
	renderPage(url, p);
	return calls;
};
const bubble = (m: ChatMessage) => document.getElementById(`m${m.message_id}`)!;
const formatted = (p: Persona = 'superadmin') => msgs(p).find((m) => m.text.startsWith('Release notes:'))!;
const blocks = () => msgs().find((m) => m.text.includes('fn main()'))!;
const editedOne = () => msgs().find((m) => m.edits.some((e) => (e as HasEntities).entities?.length))!;
const ent = (m: ChatMessage, type: string, i = 0) => (m as HasEntities).entities!.filter((e) => e.type === type)[i]!;
const textOf = (m: { text: string }, e: TextEntity) => m.text.slice(e.offset, e.offset + e.length);
const hashtagHref = (q: string) => `/group_messages?q=${encodeURIComponent(q)}&f=${encodeURIComponent(JSON.stringify([{ field: 'chat_id', op: '=', value: String(KINDS_GROUP.id) }]))}`;

describe('text entities in the chat', () => {
	it('draws a bubble\'s links, mentions, tags and styles', () => {
		openChat();
		const m = formatted();
		const b = bubble(m).querySelector('.c-text')!;
		const w = within(b as HTMLElement);
		expect(w.getByText('https://example.org/notes').getAttribute('href')).toBe('https://example.org/notes');
		const cl = w.getByText('the changelog');
		expect(cl.getAttribute('href')).toBe('https://example.org/changelog');
		expect(cl.getAttribute('rel')).toBe('noopener noreferrer nofollow');
		expect(cl.getAttribute('target')).toBe('_blank');
		expect(w.getByText('#release').getAttribute('href')).toBe(hashtagHref('#release'));
		const bold = b.querySelector('strong')!;
		expect(bold.textContent).toBe('Bold with italic inside');
		expect(bold.querySelector('em')!.textContent).toBe('italic');
		expect(b.querySelector('u')!.textContent).toBe('underlined');
		expect(b.querySelector('s')!.textContent).toBe('struck');
		expect(w.getByText('inline code').tagName).toBe('CODE');
		expect(w.getByText('/status@tgd_bot').className).toBe('rt-cmd');
		// javascript: never becomes a link.
		expect(w.getByText('a sneaky link').tagName).toBe('SPAN');
		expect([...b.querySelectorAll('a')].every((a) => /^(https?:|mailto:|tel:|\/)/.test(a.getAttribute('href')!))).toBe(true);
		// The text is all there, the entity past the end clipped.
		expect(b.textContent).toBe(m.text);
		expect(b.querySelector('strong:last-of-type')!.textContent).toBe('bold to the end');
	});

	it('links a mention resolved to a user to the user page, else to t.me', () => {
		openChat();
		const m = formatted();
		const known = ent(m, 'mention', 0);
		expect(typeof known.user_id).toBe('number');
		const w = within(bubble(m));
		const a = w.getByText(textOf(m, known));
		expect(a.getAttribute('href')).toBe(`/users/${known.user_id}`);
		expect(a.getAttribute('target')).toBeNull();
		expect(w.getByText('@nobody_here_x').getAttribute('href')).toBe('https://t.me/nobody_here_x');
		const name = ent(m, 'mention_name', 0);
		expect(w.getByText(textOf(m, name)).getAttribute('href')).toBe(`/users/${name.user_id}`);
	});

	it('opens the user page of a resolved mention in place', async () => {
		const calls = openChat();
		const m = formatted();
		const known = ent(m, 'mention', 0);
		fireEvent.click(within(bubble(m)).getByText(textOf(m, known)));
		await waitFor(() => expect(callsTo(calls, 'GET', new RegExp(`^/api/page/users/${known.user_id}$`))).toHaveLength(1));
	});

	it('shows a member the mentions it may not resolve as t.me links and plain names', () => {
		openChat('member');
		const m = formatted('member');
		const hidden = ent(m, 'mention', 1);
		expect(hidden.user_id).toBeUndefined();
		const w = within(bubble(m));
		const handle = textOf(m, hidden);
		expect(w.getByText(handle).getAttribute('href')).toBe(`https://t.me/${handle.slice(1)}`);
		const name = w.getByText(textOf(m, ent(m, 'mention_name', 1)));
		expect(name.tagName).toBe('SPAN');
	});

	it('hides a spoiler until clicked and folds a long quote', () => {
		openChat();
		const sp = within(bubble(formatted())).getByRole('button', { name: /spoiler/i });
		expect(sp.textContent).toBe('a hidden ending');
		fireEvent.click(sp);
		expect(within(bubble(formatted())).queryByRole('button', { name: /spoiler/i })).toBeNull();
		const b = bubble(blocks());
		expect(b.querySelector('pre.rt-pre code.language-rust')!.textContent).toContain('println!');
		expect(b.querySelector('blockquote.rt-quote:not(.rt-expandable)')!.textContent).toBe('A quote from the docs.');
		const more = within(b).getByRole('button', { name: 'Show more' });
		fireEvent.click(more);
		expect(b.querySelector('blockquote.rt-expandable')!.classList.contains('open')).toBe(true);
	});

	it('places entities after emoji by UTF-16 offsets', () => {
		openChat();
		const b = bubble(blocks());
		expect(b.querySelector('.c-text strong')!.textContent).toBe('Family');
		expect(b.querySelector('.c-text em')!.textContent).toBe('𝕏');
	});

	it('draws an earlier version\'s entities in the edit list', () => {
		openChat();
		const m = editedOne();
		fireEvent.click(within(bubble(m)).getByRole('button', { name: /edited/ }));
		const list = bubble(m).querySelector('.c-edits')!;
		expect(within(list as HTMLElement).getByText('#meetup').getAttribute('href')).toBe(hashtagHref('#meetup'));
		expect(within(list as HTMLElement).getByText('Thursday').tagName).toBe('EM');
		// The current version, with its own.
		expect(within(list as HTMLElement).getByText('Friday').tagName).toBe('STRONG');
	});

	it('draws a poll\'s question and explanation and checklist tasks with entities', () => {
		openChat();
		const quiz = msgs().find((m) => m.detail?.kind === 'poll' && m.detail.type === 'quiz')!;
		const q = bubble(quiz).querySelector('.k-question')!;
		expect(q.textContent).toBe('Who wrote the first Linux kernel?');
		expect(q.querySelector('em')!.textContent).toBe('Linux kernel');
		expect(within(bubble(quiz).querySelector('.k-explain') as HTMLElement).getByText('the first version').getAttribute('href')).toBe('https://example.org/linux-0.01');
		const list = msgs().find((m) => m.detail?.kind === 'checklist')!;
		expect(within(bubble(list)).getByText('make dist').tagName).toBe('CODE');
		expect(within(bubble(list)).getByText('#news').getAttribute('href')).toBe(hashtagHref('#news'));
	});

	it('styles tags without a search for a signed-out visitor', () => {
		const exposed = fixture<{ rows: { id: number; exposed?: boolean }[] }>('/groups').props.rows.find((g) => g.exposed)!;
		fakeApi('public');
		const purl = `/groups/${exposed.id}/chat`;
		const m = fixture<ChatProps>(purl, 'public').props.messages.at(-1)!;
		const text = 'see #tag here';
		renderPage(purl, 'public', { props: { ...fixture<ChatProps>(purl, 'public').props, messages: [{ ...m, text, entities: [{ offset: 4, length: 4, type: 'hashtag' }] } as ChatMessage] } });
		expect(screen.getByText('#tag').tagName).toBe('SPAN');
	});
});
