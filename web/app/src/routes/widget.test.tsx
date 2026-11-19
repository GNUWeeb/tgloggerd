// @vitest-environment happy-dom
// A widget message (chat-widget.tsx): its box at its size in the chat
// and on the message page, and its HTML only ever as text on the
// message page, never rendered (decision 2026-11-20-msg-widgets-html).
import { readFileSync } from 'node:fs';
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, within } from '@testing-library/react';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { ALL, KINDS_GROUP, sampleOf } from '../../standin/kinds.ts';
import { chatStream } from '../../../mockup/src/data/messages.js';
import type { ChatProps } from './chat.tsx';
import type { MessageProps } from './message.tsx';
import type { S } from '../api/schema.ts';

beforeEach(setup);

type W = S['WidgetDetail'];
type Raw = { id: number; message_id: number; chat: { id: number; kind: string } };
// The stand-in's widget samples, as the message page sends them.
const widgetOf = (m: Raw) => {
	const d = sampleOf(m)?.detail?.(ALL) as W | undefined;
	return d?.kind === 'widget' ? d : null;
};
const rawWidget = (pick: (d: W) => boolean) => (chatStream(KINDS_GROUP) as Raw[]).find((m) => { const d = widgetOf(m); return !!d && pick(d); })!;
const sized = (d: W) => d.width > 0 && d.height > 0 && d.html !== null;
const unsized = (d: W) => d.width === 0 && d.height === 0;
const notKept = (d: W) => d.html === null;

// No element of the page came from the widget's HTML.
const noneFromHtml = () => {
	expect(document.querySelector('script, iframe, object, embed, img[onerror]')).toBeNull();
	expect(document.title).not.toBe('widget ran');
};

describe('a widget in the chat', () => {
	const show = () => {
		fakeApi('superadmin');
		const url = `/groups/${KINDS_GROUP.id}/chat`;
		renderPage(url, 'superadmin');
		return fixture<ChatProps>(url, 'superadmin').props.messages;
	};
	const bubble = (id: number) => document.getElementById(`m${id}`)!;

	it('draws the box at its size, or the bubble\'s width and a default height', () => {
		show();
		const box = bubble(rawWidget(sized).message_id).querySelector<HTMLElement>('.k-widgetbox')!;
		expect(box.style.width).toBe('320px');
		expect(box.style.maxWidth).toBe('100%');
		expect(box.style.aspectRatio.replace(/\s/g, '')).toBe('320/180');
		const free = bubble(rawWidget(unsized).message_id).querySelector<HTMLElement>('.k-widgetbox')!;
		expect(free.style.width).toBe('100%');
		expect(free.style.height).toBe('160px');
	});

	it('says the archive does not run it and links the message page, without its source', () => {
		const msgs = show();
		const raw = rawWidget(sized);
		const m = msgs.find((x) => x.message_id === raw.message_id)!;
		// The chat window never gets the HTML.
		expect(m.detail).toMatchObject({ kind: 'widget', html: null });
		const b = within(bubble(raw.message_id));
		expect(b.getByText('This archive does not run widgets.')).toBeTruthy();
		expect(b.getByRole('link', { name: 'Its source is on the message page' }).getAttribute('href')).toBe(`/messages/group/${raw.id}`);
		expect(screen.queryByText('Show source')).toBeNull();
		expect(document.querySelector('.k-widgetsrc')).toBeNull();
		expect(document.body.textContent).not.toContain('<script>');
		noneFromHtml();
	});
});

describe('a widget on its message page', () => {
	const open = (id: number) => {
		fakeApi('superadmin');
		renderPage(`/messages/group/${id}`, 'superadmin');
		return fixture<MessageProps>(`/messages/group/${id}`, 'superadmin').props.message;
	};

	it('shows the box, the shared id and the HTML as literal text behind Show source', () => {
		const m = open(rawWidget(sized).id);
		expect((m.detail as W).html).toContain('<script>');
		const box = document.querySelector<HTMLElement>('.a-msgkind .k-widgetbox')!;
		expect(box.style.width).toBe('320px');
		expect(screen.getByText('5012345678901234567')).toBeTruthy();
		const src = document.querySelector('details.k-widgetsrc')!;
		expect(src.hasAttribute('open')).toBe(false);
		fireEvent.click(within(src as HTMLElement).getByText('Show source'));
		const code = src.querySelector('pre code')!;
		expect(code.textContent).toBe((m.detail as W).html);
		expect(code.textContent).toContain('<script>');
		expect(code.textContent).toContain('onerror=');
		// The code block holds one text node: nothing was parsed as markup.
		expect(code.children).toHaveLength(0);
		noneFromHtml();
	});

	it('says when the HTML was not kept', () => {
		open(rawWidget(notKept).id);
		expect(screen.getByText('Its HTML was not kept.')).toBeTruthy();
		expect(screen.queryByText('Show source')).toBeNull();
		expect(document.querySelector<HTMLElement>('.a-msgkind .k-widgetbox')!.style.aspectRatio.replace(/\s/g, '')).toBe('400/300');
	});
});

describe('the widget renderer', () => {
	it('has no way to hand the HTML to the DOM as markup', () => {
		const src = readFileSync('src/routes/chat-widget.tsx', 'utf8');
		for (const banned of [/dangerouslySetInnerHTML/, /innerHTML/i, /outerHTML/i, /<iframe/i, /srcdoc/i, /<object/i, /<embed/i, /createObjectURL/, /DOMParser/, /createContextualFragment/, /insertAdjacentHTML/, /document\.write/, /data:text\/html/i]) {
			expect(src).not.toMatch(banned);
		}
	});
});
