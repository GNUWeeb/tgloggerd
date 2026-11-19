// @vitest-environment happy-dom
// The text entity renderer: every type, nesting and overlaps, UTF-16
// offsets around emoji, unsafe link schemes and malformed entities.
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { NavProvider } from '../nav.tsx';
import { RichText, safeHref, type RichChat } from './richtext.tsx';
import { fixture } from '../test/page.tsx';
import type { TextEntity } from '../api/entities.ts';

afterEach(cleanup);

const at = (text: string, part: string, type: string, extra: Partial<TextEntity> = {}, from = 0): TextEntity => ({ offset: text.indexOf(part, from), length: part.length, type, ...extra });

function show(text: string, entities: unknown[], chat?: RichChat) {
	const { container } = render(<NavProvider initial={fixture('/')}><div data-testid="t"><RichText text={text} entities={entities} chat={chat} /></div></NavProvider>);
	return container.querySelector('[data-testid="t"]')!;
}

describe('RichText', () => {
	it('draws plain text as it is', () => {
		const el = show('hello\nworld', []);
		expect(el.innerHTML).toBe('hello\nworld');
	});

	it('draws the inline styles', () => {
		const t = 'bold italic under strike code';
		const el = show(t, [at(t, 'bold', 'bold'), at(t, 'italic', 'italic'), at(t, 'under', 'underline'), at(t, 'strike', 'strikethrough'), at(t, 'code', 'code')]);
		expect(el.querySelector('strong')!.textContent).toBe('bold');
		expect(el.querySelector('em')!.textContent).toBe('italic');
		expect(el.querySelector('u')!.textContent).toBe('under');
		expect(el.querySelector('s')!.textContent).toBe('strike');
		expect(el.querySelector('code.rt-code')!.textContent).toBe('code');
		expect(el.textContent).toBe(t);
	});

	it('draws a rich page\'s subscript, superscript and marked text', () => {
		const t = 'H2O, x2 and a marked word';
		const el = show(t, [at(t, '2', 'subscript'), at(t, '2', 'superscript', {}, 5), at(t, 'marked', 'marked')]);
		expect(el.querySelector('sub')!.textContent).toBe('2');
		expect(el.querySelector('sup')!.textContent).toBe('2');
		expect(el.querySelector('mark.rt-mark')!.textContent).toBe('marked');
		expect(el.textContent).toBe(t);
	});

	it('links urls, text urls, emails and phone numbers to new tabs', () => {
		const t = 'see example.org/x or here, mail a@b.co, call +62 812 0000';
		const el = show(t, [at(t, 'example.org/x', 'url'), at(t, 'here', 'text_url', { url: 'https://kernel.org/' }), at(t, 'a@b.co', 'email'), at(t, '+62 812 0000', 'phone_number')]);
		const links = [...el.querySelectorAll('a')];
		expect(links.map((a) => a.getAttribute('href'))).toEqual(['https://example.org/x', 'https://kernel.org/', 'mailto:a@b.co', 'tel:+628120000']);
		for (const a of links) {
			expect(a.getAttribute('target')).toBe('_blank');
			expect(a.getAttribute('rel')).toBe('noopener noreferrer nofollow');
		}
	});

	it('never links unsafe schemes', () => {
		const t = 'click me or javascript:alert(1) or this';
		const el = show(t, [at(t, 'click me', 'text_url', { url: 'javascript:alert(1)' }), at(t, 'javascript:alert(1)', 'url'), at(t, 'this', 'text_url', { url: ' java\tscript:alert(2)' })]);
		expect(el.querySelectorAll('a')).toHaveLength(0);
		expect(el.textContent).toBe(t);
		expect(safeHref('data:text/html,x')).toBeNull();
		expect(safeHref('tg://resolve?domain=x')).toBe('tg://resolve?domain=x');
		expect(safeHref('HTTPS://x.org')).toBe('HTTPS://x.org');
	});

	it('links mentions: a known user to the user page, else to t.me', () => {
		const t = 'hi @alice and @bob_x and Carol and Dave';
		const el = show(t, [at(t, '@alice', 'mention', { user_id: 101 }), at(t, '@bob_x', 'mention'), at(t, 'Carol', 'mention_name', { user_id: 202 }), at(t, 'Dave', 'mention_name', { user_id: null })]);
		expect(screen.getByText('@alice').getAttribute('href')).toBe('/users/101');
		expect(screen.getByText('@alice').getAttribute('target')).toBeNull();
		expect(screen.getByText('@bob_x').getAttribute('href')).toBe('https://t.me/bob_x');
		expect(screen.getByText('Carol').getAttribute('href')).toBe('/users/202');
		const dave = screen.getByText('Dave');
		expect(dave.tagName).toBe('SPAN');
		expect(dave.className).toBe('rt-mention');
		expect(el.querySelectorAll('a')).toHaveLength(3);
	});

	it('links hashtags and cashtags to the chat search, plain without a chat', () => {
		const t = '#rust and $BTC';
		const chat = { scope: 'group' as const, id: -100123 };
		show(t, [at(t, '#rust', 'hashtag'), at(t, '$BTC', 'cashtag')], chat);
		const href = new URL(screen.getByText('#rust').getAttribute('href')!, 'http://x');
		expect(href.pathname).toBe('/group_messages');
		expect(href.searchParams.get('q')).toBe('#rust');
		expect(JSON.parse(href.searchParams.get('f')!)).toEqual([{ field: 'chat_id', op: '=', value: '-100123' }]);
		expect(screen.getByText('$BTC').getAttribute('href')).toContain('q=%24BTC');
		cleanup();
		show(t, [at(t, '#rust', 'hashtag')]);
		expect(screen.getByText('#rust').tagName).toBe('SPAN');
	});

	it('styles bot commands, custom emoji and leaves the others plain', () => {
		const t = '/start 😀 at 1:05 card 4111 and ??';
		const el = show(t, [at(t, '/start', 'bot_command'), at(t, '😀', 'custom_emoji', { custom_emoji_id: '5368324170671202286' }), at(t, '1:05', 'media_timestamp'), at(t, '4111', 'bank_card_number'), at(t, '??', 'some_future_type'), at(t, '4111', 'unknown')]);
		expect(el.querySelector('.rt-cmd')!.textContent).toBe('/start');
		expect(el.querySelector('.rt-emoji')!.textContent).toBe('😀');
		expect(el.querySelectorAll('a')).toHaveLength(0);
		expect(el.textContent).toBe(t);
	});

	it('draws pre and pre_code blocks with the language as a class', () => {
		const t = 'look:\nfn main() {}\nand\nx = 1';
		const el = show(t, [at(t, 'fn main() {}', 'pre_code', { language: 'rust' }), at(t, 'x = 1', 'pre'), { offset: 0, length: 4, type: 'pre_code', language: '"><script>' }]);
		const pres = el.querySelectorAll('pre.rt-pre');
		expect(pres).toHaveLength(3);
		expect(pres[1]!.querySelector('code')!.className).toBe('language-rust');
		expect(pres[1]!.textContent).toBe('fn main() {}');
		expect(pres[2]!.querySelector('code')!.className).toBe('');
		expect(pres[0]!.querySelector('code')!.className).toBe('language-script');
	});

	it('nests entities: bold inside a link, italic inside bold', () => {
		const t = 'read the fine manual now';
		const el = show(t, [at(t, 'the fine manual', 'text_url', { url: 'https://man.example/' }), at(t, 'fine manual', 'bold'), at(t, 'manual', 'italic')]);
		const a = el.querySelector('a')!;
		expect(a.textContent).toBe('the fine manual');
		expect(a.querySelector('strong')!.textContent).toBe('fine manual');
		expect(a.querySelector('strong em')!.textContent).toBe('manual');
		expect(el.querySelectorAll('a')).toHaveLength(1);
		expect(el.textContent).toBe(t);
	});

	it('splits an overlapping entity where it must, keeping the text', () => {
		// bold [0,8), italic [4,12): italic is split at bold's end.
		const t = 'aaaabbbbcccc';
		const el = show(t, [{ offset: 0, length: 8, type: 'bold' }, { offset: 4, length: 8, type: 'italic' }]);
		expect(el.querySelector('strong')!.textContent).toBe('aaaabbbb');
		expect([...el.querySelectorAll('em')].map((e) => e.textContent)).toEqual(['bbbb', 'cccc']);
		expect(el.textContent).toBe(t);
	});

	it('never nests a link in a link', () => {
		const t = 'go @someone now';
		const el = show(t, [{ offset: 0, length: t.length, type: 'text_url', url: 'https://a.example/' }, at(t, '@someone', 'mention')]);
		expect(el.querySelectorAll('a')).toHaveLength(1);
		expect(el.querySelector('a .rt-mention')!.tagName).toBe('SPAN');
	});

	it('counts offsets in UTF-16 units, past emoji and surrogate pairs', () => {
		// 👨‍👩‍👧 is 8 UTF-16 units, 𝕏 2.
		const t = '👨‍👩‍👧 𝕏 bold';
		expect(t.length).toBe(16);
		const el = show(t, [{ offset: 12, length: 4, type: 'bold' }, { offset: 9, length: 2, type: 'italic' }, { offset: 0, length: 8, type: 'underline' }]);
		expect(el.querySelector('strong')!.textContent).toBe('bold');
		expect(el.querySelector('em')!.textContent).toBe('𝕏');
		expect(el.querySelector('u')!.textContent).toBe('👨‍👩‍👧');
		expect(el.textContent).toBe(t);
	});

	it('clips an entity past the end and ignores malformed ones', () => {
		const t = 'short text';
		const el = show(t, [{ offset: 6, length: 99, type: 'bold' }, { offset: 50, length: 3, type: 'italic' }, { offset: -1, length: 3, type: 'italic' }, { offset: 1, length: 0, type: 'italic' },
			{ offset: 1.5, length: 2, type: 'italic' }, { offset: 0, length: 2 }, null, 'x', { offset: '0', length: 2, type: 'italic' }]);
		expect(el.querySelector('strong')!.textContent).toBe('text');
		expect(el.querySelector('em')).toBeNull();
		expect(el.textContent).toBe(t);
		cleanup();
		expect(show(t, null as never).textContent).toBe(t);
	});

	it('hides a spoiler until clicked', () => {
		const t = 'the end: he was dead';
		const el = show(t, [at(t, 'he was dead', 'spoiler')]);
		const sp = screen.getByRole('button', { name: /spoiler/i });
		expect(sp.className).toBe('rt-spoiler');
		fireEvent.click(sp);
		expect(screen.queryByRole('button', { name: /spoiler/i })).toBeNull();
		expect(el.querySelector('.rt-spoiler.shown')!.textContent).toBe('he was dead');
	});

	it('reveals a spoiler from the keyboard, and a link in it opens only after', () => {
		const t = 'secret link';
		const el = show(t, [{ offset: 0, length: t.length, type: 'spoiler' }, at(t, 'link', 'text_url', { url: 'https://s.example/' })]);
		const a = el.querySelector('a')!;
		const ev = new MouseEvent('click', { bubbles: true, cancelable: true });
		a.dispatchEvent(ev);
		expect(ev.defaultPrevented).toBe(true);
		cleanup();
		show(t, [{ offset: 0, length: t.length, type: 'spoiler' }]);
		const sp = screen.getByRole('button', { name: /spoiler/i });
		expect(sp.tabIndex).toBe(0);
		fireEvent.keyDown(sp, { key: 'Enter' });
		expect(screen.queryByRole('button', { name: /spoiler/i })).toBeNull();
	});

	it('draws quotes; an expandable one opens with Show more', () => {
		const t = 'said:\nquoted\nlong quote';
		const el = show(t, [at(t, 'quoted', 'block_quote'), at(t, 'long quote', 'expandable_block_quote')]);
		expect(el.querySelector('blockquote.rt-quote:not(.rt-expandable)')!.textContent).toBe('quoted');
		const q = el.querySelector('blockquote.rt-expandable')!;
		const b = screen.getByRole('button', { name: 'Show more' });
		expect(b.getAttribute('aria-expanded')).toBe('false');
		expect(q.classList.contains('open')).toBe(false);
		fireEvent.click(b);
		expect(q.classList.contains('open')).toBe(true);
		expect(screen.getByRole('button', { name: 'Show less' }).getAttribute('aria-expanded')).toBe('true');
	});

	it('never writes markup from the text', () => {
		const t = '<img src=x onerror=alert(1)> bold';
		const el = show(t, [at(t, 'bold', 'bold')]);
		expect(el.querySelector('img')).toBeNull();
		expect(el.textContent).toBe(t);
	});
});
