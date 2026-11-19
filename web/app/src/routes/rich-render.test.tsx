// @vitest-environment happy-dom
// A rich message in the chat: its page blocks, cut to the first few
// with "Show all", instead of its text; an earlier version's page in
// the edit list; and a message whose page was not kept, shown as its
// text.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, within } from '@testing-library/react';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { KINDS_GROUP } from '../../standin/kinds.ts';
import { RICH_PLAIN_TEXT } from '../../standin/rich.ts';
import { privateMessages } from '../../../mockup/src/data/messages.js';
import { CHAT_BLOCKS } from '../ui/richblocks.tsx';
import type { ChatProps } from './chat.tsx';
import type { ChatMessage } from '../types.ts';

beforeEach(setup);

const groupChat = `/groups/${KINDS_GROUP.id}/chat`;
const privateChat = `/users/${privateMessages()[0]!.chat.id}/chat`;
const show = (url: string) => {
	fakeApi('superadmin');
	renderPage(url, 'superadmin');
	return fixture<ChatProps>(url, 'superadmin').props.messages;
};
const bubbleOf = (m: ChatMessage) => document.getElementById(`m${m.message_id}`)!;
const rich = (msgs: ChatMessage[]) => msgs.find((m) => m.detail?.kind === 'rich')!;

describe('a rich message in the chat', () => {
	it('draws its first blocks instead of its text, and the rest on Show all', () => {
		const m = rich(show(groupChat));
		const b = bubbleOf(m);
		expect(b.querySelector('.c-text')).toBeNull();
		expect(b.querySelector('.k-placeholder')).toBeNull();
		expect(b.querySelector('h2.rb-title')!.textContent).toBe('Release 2.0 of the sample tool');
		expect(b.querySelector('table')).toBeNull();
		const d = m.detail as { blocks: unknown[] };
		fireEvent.click(within(b).getByRole('button', { name: `Show all (${d.blocks.length - CHAT_BLOCKS} more blocks)` }));
		expect(b.querySelector('table.rb-table')).toBeTruthy();
		expect(within(b).getByText('A block this view does not show (future_block)')).toBeTruthy();
	});

	it('draws an earlier version\'s page, without media, in the edit list', () => {
		const m = rich(show(groupChat));
		const b = bubbleOf(m);
		fireEvent.click(within(b).getByRole('button', { name: 'edited' }));
		const list = b.querySelector('.c-edits') as HTMLElement;
		const page = list.querySelector('.rb-page')!;
		expect(page.textContent).toContain('Draft notes');
		expect(page.querySelector('em')!.textContent).toBe('Draft');
		expect(page.querySelector('.rb-nomedia')).toBeNull();
		// The version's text is its page's; it is not drawn twice.
		expect(list.textContent!.split('Draft notes')).toHaveLength(2);
	});

	it('shows the text of one whose page was not kept', () => {
		const m = rich(show(privateChat));
		const b = bubbleOf(m);
		expect(b.querySelector('.rb-page')).toBeNull();
		expect(b.querySelector('.c-text')!.textContent).toBe(RICH_PLAIN_TEXT);
		expect(b.querySelector('.k-placeholder')).toBeNull();
	});
});

describe('a rich message on its message page', () => {
	const open = (scope: 'group' | 'private', id: number) => {
		fakeApi('superadmin');
		renderPage(`/messages/${scope}/${id}`, 'superadmin');
		return document.querySelector('.a-msgkind');
	};

	it('draws its whole page, not its text', () => {
		const m = rich(fixture<ChatProps>(groupChat, 'superadmin').props.messages);
		const kind = open('group', m.id)!;
		expect(kind.querySelector('h2.rb-title')).toBeTruthy();
		expect(kind.querySelector('table.rb-table')).toBeTruthy();
		expect(within(kind as HTMLElement).getByText('A block this view does not show (future_block)')).toBeTruthy();
		expect(within(kind as HTMLElement).queryByRole('button', { name: /Show all/ })).toBeNull();
		expect(document.querySelector('.a-msgtext')).toBeNull();
	});

	it('draws an earlier version\'s page in the edit history, once', () => {
		const m = rich(fixture<ChatProps>(groupChat, 'superadmin').props.messages);
		open('group', m.id);
		const pages = [...document.querySelectorAll('.e-detail .rb-page')];
		expect(pages).toHaveLength(1);
		expect(pages[0]!.textContent).toContain('Draft notes');
		expect(document.body.textContent!.split('Draft notes')).toHaveLength(2);
	});

	it('shows the text of one whose page was not kept', () => {
		const m = rich(fixture<ChatProps>(privateChat, 'superadmin').props.messages);
		open('private', m.id);
		expect(document.querySelector('.rb-page')).toBeNull();
		expect(document.querySelector('.a-msgtext')!.textContent).toBe(RICH_PLAIN_TEXT);
		expect(document.querySelector('.k-placeholder')).toBeNull();
	});
});
