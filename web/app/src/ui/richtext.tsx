// A message text with Telegram's text entities: links, mentions,
// hashtags, bold, italic, code, spoilers, quotes and the rest, drawn as
// React nodes (never as HTML). Offsets and lengths are UTF-16 code
// units, which are JavaScript's string indexes. Entities may nest and
// may overlap; one that overlaps another is split where it must be. An
// entity past the text's end is clipped, a malformed one ignored, an
// unknown type drawn as plain text.
import { useState, type KeyboardEvent, type MouseEvent, type ReactNode } from 'react';
import { Link } from '../nav.tsx';
import type { TextEntity } from '../api/entities.ts';
import { CustomEmoji } from './customemoji.tsx';

// The chat a text is in: hashtags search its messages.
export interface RichChat { scope: 'group' | 'private'; id: number }

// The chat's message search for a text, as the chat's search box does.
export const chatSearchHref = (c: RichChat, q: string) => `/${c.scope}_messages?q=${encodeURIComponent(q)}&f=${encodeURIComponent(JSON.stringify([{ field: 'chat_id', op: '=', value: String(c.id) }]))}`;

// Only these schemes become links; javascript:, data: and the others
// stay text. Browsers ignore control characters and spaces in a URL's
// scheme, so they are dropped before the check.
const SAFE = new Set(['http', 'https', 'mailto', 'tel', 'tg']);
export function safeHref(raw: string): string | null {
	const u = raw.replace(/[\u0000- \u007f]/g, '');
	if (!u) return null;
	const s = /^([a-z][a-z0-9+.-]*):/i.exec(u);
	if (!s) return /^[\w.-]+\.[a-z]{2,}([/?#:]|$)/i.test(u) ? `https://${u}` : null;
	return SAFE.has(s[1]!.toLowerCase()) ? u : null;
}

const EXT = { target: '_blank', rel: 'noopener noreferrer nofollow' } as const;

interface Node { e: TextEntity | null; start: number; end: number; kids: (Node | string)[] }

// The entities that can be drawn: whole numbers inside the text, of a
// string type; clipped to the text's end.
function clean(text: string, list: readonly unknown[] | null | undefined): TextEntity[] {
	const out: TextEntity[] = [];
	for (const x of list ?? []) {
		if (!x || typeof x !== 'object') continue;
		const e = x as TextEntity;
		if (typeof e.type !== 'string' || !Number.isInteger(e.offset) || !Number.isInteger(e.length) || e.offset < 0 || e.length <= 0 || e.offset >= text.length) continue;
		out.push({ ...e, length: Math.min(e.length, text.length - e.offset) });
	}
	return out;
}

// The entities as a tree. The text is cut where any entity starts or
// ends; each piece is inside the entities that cover it. Walking the
// pieces, entities still open stay open; one that ends inside another
// closes that one too, which reopens after it. Among those opening at
// one place, the one that lasts longest goes outside, so properly
// nested entities are never split.
function tree(text: string, ents: TextEntity[]): Node {
	const root: Node = { e: null, start: 0, end: text.length, kids: [] };
	const cuts = [...new Set([0, text.length, ...ents.flatMap((e) => [e.offset, e.offset + e.length])])].sort((a, b) => a - b);
	const stack: Node[] = [root];
	for (let i = 0; i + 1 < cuts.length; i++) {
		const a = cuts[i]!, b = cuts[i + 1]!;
		const on = ents.filter((e) => e.offset <= a && e.offset + e.length >= b);
		// Keep the open entities that still cover this piece, from the
		// outside in, up to the first one that does not.
		let keep = 1;
		while (keep < stack.length && on.includes(stack[keep]!.e!)) keep++;
		stack.length = keep;
		const open = new Set(stack.map((n) => n.e));
		const add = on.filter((e) => !open.has(e)).sort((x, y) => (y.offset + y.length) - (x.offset + x.length) || ents.indexOf(x) - ents.indexOf(y));
		for (const e of add) {
			const n: Node = { e, start: a, end: e.offset + e.length, kids: [] };
			stack[stack.length - 1]!.kids.push(n);
			stack.push(n);
		}
		stack[stack.length - 1]!.kids.push(text.slice(a, b));
	}
	return root;
}

// A spoiler: hidden until clicked or activated by keyboard, then shown
// for good. While hidden, a link inside does not open.
function Spoiler({ children }: { children: ReactNode }) {
	const [shown, setShown] = useState(false);
	if (shown) return <span className="rt-spoiler shown">{children}</span>;
	const show = (e: MouseEvent | KeyboardEvent) => { e.preventDefault(); e.stopPropagation(); setShown(true); };
	return (
		<span className="rt-spoiler" role="button" tabIndex={0} aria-label="Spoiler, show the hidden text" onClickCapture={show}
			onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') show(e); }}>
			<span aria-hidden="true">{children}</span>
		</span>
	);
}

// A quote that Telegram shows collapsed until opened.
function Expandable({ children }: { children: ReactNode }) {
	const [open, setOpen] = useState(false);
	return (
		<blockquote className={`rt-quote rt-expandable ${open ? 'open' : ''}`}>
			<div className="rt-quotebody">{children}</div>
			<button type="button" className="rt-more" aria-expanded={open} onClick={() => setOpen((x) => !x)}>{open ? 'Show less' : 'Show more'}</button>
		</blockquote>
	);
}

const LINKS = new Set(['url', 'text_url', 'email', 'phone_number', 'mention', 'mention_name', 'hashtag', 'cashtag']);

function draw(n: Node | string, text: string, chat: RichChat | undefined, inLink: boolean, key: number): ReactNode {
	if (typeof n === 'string') return n;
	const e = n.e!;
	// A link inside a link would nest anchors: the inner one is styled
	// only.
	const linked = inLink || LINKS.has(e.type);
	const kids = n.kids.map((k, i) => draw(k, text, chat, linked, i));
	const own = text.slice(e.offset, e.offset + e.length);
	const ext = (href: string | null, cls = 'rt-link') => (href && !inLink ? <a key={key} className={cls} href={href} {...EXT}>{kids}</a> : <span key={key} className={cls}>{kids}</span>);
	const app = (to: string | null, cls: string) => (to && !inLink ? <Link key={key} to={to} className={cls}>{kids}</Link> : <span key={key} className={cls}>{kids}</span>);
	switch (e.type) {
		case 'url': return ext(safeHref(own));
		case 'text_url': return ext(typeof e.url === 'string' ? safeHref(e.url) : null);
		case 'email': return ext(/^[^\s@:]+@[^\s@]+$/.test(own) ? `mailto:${own}` : null);
		case 'phone_number': {
			const tel = own.replace(/[^\d+]/g, '');
			return ext(tel ? `tel:${tel}` : null);
		}
		case 'mention': {
			if (typeof e.user_id === 'number') return app(`/users/${e.user_id}`, 'rt-mention');
			const name = /^@([A-Za-z0-9_]{1,64})$/.exec(own)?.[1];
			return ext(name ? `https://t.me/${name}` : null, 'rt-mention');
		}
		case 'mention_name': return app(typeof e.user_id === 'number' ? `/users/${e.user_id}` : null, 'rt-mention');
		case 'hashtag': case 'cashtag': return app(chat ? chatSearchHref(chat, own) : null, 'rt-tag');
		case 'bot_command': return <span key={key} className="rt-cmd">{kids}</span>;
		case 'bold': return <strong key={key}>{kids}</strong>;
		case 'italic': return <em key={key}>{kids}</em>;
		case 'underline': return <u key={key}>{kids}</u>;
		case 'strikethrough': return <s key={key}>{kids}</s>;
		case 'spoiler': return <Spoiler key={key}>{kids}</Spoiler>;
		case 'code': return <code key={key} className="rt-code">{kids}</code>;
		case 'pre': case 'pre_code': {
			const lang = e.type === 'pre_code' && typeof e.language === 'string' ? e.language.replace(/[^\w+#-]/g, '') : '';
			return <pre key={key} className="rt-pre"><code className={lang ? `language-${lang}` : undefined}>{kids}</code></pre>;
		}
		case 'block_quote': return <blockquote key={key} className="rt-quote">{kids}</blockquote>;
		case 'expandable_block_quote': return <Expandable key={key}>{kids}</Expandable>;
		// The sticker, its text (the emoji's own characters) as alt text;
		// the text itself while the sticker is not known.
		case 'custom_emoji': return <CustomEmoji key={key} e={e.custom_emoji} alt={own}><span className="rt-emoji">{kids}</span></CustomEmoji>;
		// media_timestamp (the time is in the text; the service sends no
		// second), bank_card_number, unknown and types this view does not
		// know: the text as it is.
		default: return <span key={key}>{kids}</span>;
	}
}

export function RichText({ text, entities, chat }: { text: string; entities?: readonly unknown[] | null; chat?: RichChat }) {
	const ents = clean(text, entities);
	if (!ents.length) return <>{text}</>;
	return <>{tree(text, ents).kids.map((k, i) => draw(k, text, chat, false, i))}</>;
}
