// A rich message's page blocks (RichDetail, web/docs/message-kinds.md,
// "Rich messages"): headings, paragraphs, lists, quotes, tables, media
// and the rest, drawn as React nodes, never as HTML. Their text goes
// through the text entities renderer; every link goes through its
// scheme check and opens in a new tab with rel=noopener. Embeds that
// need a third party's script are drawn as a link, and a block type
// this view does not know as a line saying so.
import { useState, type ReactNode } from 'react';
import { Square, SquareCheck } from 'lucide-react';
import { RichText, safeHref, type RichChat } from './richtext.tsx';
import { Animation, DocFile, MediaOf, Picture, VideoBox, Voice, kindOf } from '../routes/chat-media.tsx';
import { dateTime } from '../format.ts';
import type { Viewer } from '../page.ts';
import type { S } from '../api/schema.ts';

type Block = S['RichBlock'];
type Text = S['RichText'] | undefined;
type Caption = S['RichCaption'] | undefined;

// Blocks the chat shows before "Show all".
export const CHAT_BLOCKS = 4;

const EXT = { target: '_blank', rel: 'noopener noreferrer nofollow' } as const;

// edit: an earlier version, whose page comes without media.
interface Ctx { chat?: RichChat; v: Viewer; click: boolean; edit: boolean }

const has = (t: Text): t is S['RichText'] => !!t && !!t.text;
function T({ t, c }: { t: Text; c: Ctx }) {
	return has(t) ? <RichText text={t.text} entities={t.entities} chat={c.chat} /> : null;
}

// A link to a stored URL when its scheme is safe, else its children as
// text.
function Ext({ url, className, children }: { url?: string; className?: string; children: ReactNode }) {
	const href = url ? safeHref(url) : null;
	return href ? <a className={className} href={href} {...EXT}>{children}</a> : <span className={className}>{children}</span>;
}

function Cap({ cap, c }: { cap: Caption; c: Ctx }) {
	if (!cap || (!has(cap.text) && !has(cap.credit))) return null;
	return (
		<figcaption className="rb-caption">
			<T t={cap.text} c={c} />
			{has(cap.credit) && <cite className="rb-credit"><T t={cap.credit} c={c} /></cite>}
		</figcaption>
	);
}

// A media block marked as a spoiler shows nothing of its media (and
// fetches nothing) until asked.
function Hidden({ on, children }: { on?: boolean; children: ReactNode }) {
	const [shown, setShown] = useState(!on);
	if (shown) return <>{children}</>;
	return <button type="button" className="c-media c-missing rb-spoiler" onClick={() => setShown(true)}>Spoiler, show the media</button>;
}

const MEDIA_NAME: Partial<Record<Block['type'], string>> = { photo: 'photo', video: 'video', animation: 'animation', audio: 'audio file', document: 'file', voice_note: 'voice message' };

function MediaBlock({ b, c }: { b: Block; c: Ctx }) {
	const m = b.media;
	let body: ReactNode;
	if (!m) body = c.edit ? null : <div className="c-sub rb-nomedia">A {MEDIA_NAME[b.type]} that was not logged</div>;
	else {
		switch (b.type) {
			case 'photo': body = <Picture m={m} single onClick={c.click} />; break;
			case 'video': body = <VideoBox m={m} single onClick={c.click} />; break;
			case 'animation': body = <Animation m={m} onClick={c.click} />; break;
			case 'voice_note': body = <Voice m={m} />; break;
			case 'audio': body = <DocFile m={m} audio />; break;
			// A document draws by its render, as a message's would: a
			// picture or a video with its download, text, a PDF, a file.
			default: body = <MediaOf m={m} kind={kindOf('document', m) ?? 'document'} single onClick={c.click} doc />;
		}
	}
	return (
		<figure className="rb-figure">
			<Hidden on={b.spoiler && !!m}>{body}</Hidden>
			<Cap cap={b.caption} c={c} />
			{b.type === 'photo' && b.url && <Ext url={b.url} className="rt-link rb-sub">{b.url}</Ext>}
		</figure>
	);
}

// A list: numbered when its labels are numbers or letters, else
// bulleted; the labels are the page's own.
function List({ b, c }: { b: Block; c: Ctx }) {
	const items = b.items ?? [];
	const ordered = items.length > 0 && items.every((i) => /^[\dA-Za-z]{1,4}[.)]?$/.test(i.label));
	const body = items.map((it, i) => (
		<li key={i} className={it.checkbox && it.checked ? 'done' : undefined}>
			{it.checkbox ? (it.checked ? <SquareCheck size={14} className="rb-label" aria-label="checked" /> : <Square size={14} className="rb-label" aria-label="not checked" />)
				: it.label && <span className="rb-label" aria-hidden={ordered ? undefined : true}>{it.label}</span>}
			<div className="rb-item"><Blocks list={it.blocks} c={c} /></div>
		</li>
	));
	return ordered ? <ol className="rb-list">{body}</ol> : <ul className="rb-list">{body}</ul>;
}

function Table({ b, c }: { b: Block; c: Ctx }) {
	return (
		<div className="rb-tablewrap">
			<table className={`rb-table ${b.bordered ? 'bordered' : ''} ${b.striped ? 'striped' : ''}`}>
				{has(b.text) && <caption><T t={b.text} c={c} /></caption>}
				<tbody>{(b.rows ?? []).map((row, r) => (
					<tr key={r}>{row.map((cell, i) => {
						const Cell = cell.header ? 'th' : 'td';
						return (
							<Cell key={i} colSpan={cell.colspan > 1 ? cell.colspan : undefined} rowSpan={cell.rowspan > 1 ? cell.rowspan : undefined}
								style={{ textAlign: cell.align, verticalAlign: cell.valign }}><T t={cell.text} c={c} /></Cell>
						);
					})}</tr>
				))}</tbody>
			</table>
		</div>
	);
}

// A quote that the page shows folded until opened.
function Folded({ b, c }: { b: Block; c: Ctx }) {
	const [open, setOpen] = useState(false);
	return (
		<blockquote className={`rt-quote rt-expandable ${open ? 'open' : ''}`}>
			<div className="rt-quotebody"><T t={b.text} c={c} /></div>
			{has(b.credit) && <cite className="rb-credit"><T t={b.credit} c={c} /></cite>}
			<button type="button" className="rt-more" aria-expanded={open} onClick={() => setOpen((x) => !x)}>{open ? 'Show less' : 'Show more'}</button>
		</blockquote>
	);
}

const osm = (lat: number, lon: number, zoom: number) => `https://www.openstreetmap.org/?mlat=${lat}&mlon=${lon}#map=${zoom}/${lat}/${lon}`;
const HEADING: Partial<Record<Block['type'], 'h2' | 'h3' | 'h4'>> = { title: 'h2', subtitle: 'h3', header: 'h3', subheader: 'h4' };

function BlockOf({ b, c }: { b: Block; c: Ctx }): ReactNode {
	switch (b.type) {
		case 'title': case 'subtitle': case 'header': case 'subheader': {
			const H = HEADING[b.type]!;
			return <H className={`rb-${b.type}`}><T t={b.text} c={c} /></H>;
		}
		case 'section_heading': {
			const H = (b.size ?? 1) <= 1 ? 'h3' : b.size === 2 ? 'h4' : 'h5';
			return <H className="rb-heading"><T t={b.text} c={c} /></H>;
		}
		case 'kicker': return <div className="rb-kicker"><T t={b.text} c={c} /></div>;
		case 'paragraph': return <p className="rb-p"><T t={b.text} c={c} /></p>;
		case 'footer': return <div className="rb-footer c-sub"><T t={b.text} c={c} /></div>;
		case 'thinking': return <div className="rb-thinking c-sub"><T t={b.text} c={c} /></div>;
		case 'author_date': return <div className="rb-byline c-sub"><T t={b.text} c={c} />{has(b.text) && b.date ? ' . ' : ''}{b.date ? dateTime(b.date, c.v) : ''}</div>;
		case 'preformatted': {
			const lang = (b.language ?? '').replace(/[^\w+#-]/g, '');
			return <pre className="rt-pre"><code className={lang ? `language-${lang}` : undefined}><T t={b.text} c={c} /></code></pre>;
		}
		case 'mathematical_expression': return <div className="rb-math"><code className="rt-code">{b.expression ?? ''}</code></div>;
		case 'divider': return <hr className="rb-hr" />;
		// A link target inside the page; it draws nothing.
		case 'anchor': return null;
		case 'list': return <List b={b} c={c} />;
		case 'block_quote':
			return (
				<blockquote className="rt-quote rb-quote">
					<Blocks list={b.blocks} c={c} />
					{has(b.credit) && <cite className="rb-credit"><T t={b.credit} c={c} /></cite>}
				</blockquote>
			);
		case 'expandable_block_quote': return <Folded b={b} c={c} />;
		case 'pull_quote':
			return (
				<blockquote className="rb-pull">
					<T t={b.text} c={c} />
					{has(b.credit) && <cite className="rb-credit"><T t={b.credit} c={c} /></cite>}
				</blockquote>
			);
		case 'photo': case 'video': case 'animation': case 'audio': case 'document': case 'voice_note': return <MediaBlock b={b} c={c} />;
		case 'cover': return <Blocks list={b.blocks} c={c} />;
		case 'collage': case 'slideshow':
			return (
				<figure className="rb-figure">
					<div className={`rb-${b.type}`}><Blocks list={b.blocks} c={c} /></div>
					<Cap cap={b.caption} c={c} />
				</figure>
			);
		case 'embedded':
			return (
				<figure className="rb-figure rb-embed">
					{b.media && <Picture m={b.media} single onClick={c.click} />}
					{b.url ? <Ext url={b.url} className="rt-link">Embedded content: {b.url}</Ext> : <span className="c-sub">Embedded content</span>}
					<Cap cap={b.caption} c={c} />
				</figure>
			);
		case 'embedded_post':
			return (
				<blockquote className="rt-quote rb-post">
					<div className="rb-byline">{b.url ? <Ext url={b.url} className="rt-link">{b.author || b.url}</Ext> : <b>{b.author}</b>}{b.date ? <span className="c-sub"> . {dateTime(b.date, c.v)}</span> : null}</div>
					<Blocks list={b.blocks} c={c} />
					<Cap cap={b.caption} c={c} />
				</blockquote>
			);
		case 'chat_link':
			return <div className="rb-chatlink">{b.username ? <Ext url={`https://t.me/${encodeURIComponent(b.username)}`} className="rt-link">{b.title || `@${b.username}`}</Ext> : <b>{b.title}</b>}</div>;
		case 'table': return <Table b={b} c={c} />;
		case 'details':
			return (
				<details className="rb-details" open={b.open || undefined}>
					<summary><T t={b.text} c={c} /></summary>
					<Blocks list={b.blocks} c={c} />
				</details>
			);
		case 'related_articles':
			return (
				<section className="rb-related">
					{has(b.text) && <h4 className="rb-subheader"><T t={b.text} c={c} /></h4>}
					<ul>{(b.articles ?? []).map((a, i) => (
						<li key={i}>
							<Ext url={a.url} className="rt-link">{a.title || a.url}</Ext>
							{a.description && <div className="c-sub">{a.description}</div>}
							{(a.author || a.date) && <div className="c-sub">{[a.author, a.date ? dateTime(a.date, c.v) : ''].filter(Boolean).join(' . ')}</div>}
						</li>
					))}</ul>
				</section>
			);
		case 'map': {
			const ok = typeof b.lat === 'number' && typeof b.lon === 'number';
			return (
				<figure className="rb-figure">
					{ok ? <a className="rt-link" href={osm(b.lat!, b.lon!, Math.min(19, Math.max(1, b.zoom ?? 15)))} {...EXT}>Map at {b.lat!.toFixed(5)}, {b.lon!.toFixed(5)}</a> : <span className="c-sub">A map</span>}
					<Cap cap={b.caption} c={c} />
				</figure>
			);
		}
		case 'button_row':
			return (
				<div className={`rb-buttons ${b.align ?? 'left'}`}>
					{(b.buttons ?? []).map((x, i) => (x.url && safeHref(x.url)
						? <Ext key={i} url={x.url} className="a-btn rb-button"><T t={x.text} c={c} /></Ext>
						: <span key={i} className="rb-button off" title="A button an archive cannot press"><T t={x.text} c={c} /></span>))}
				</div>
			);
		default: {
			const name = b.type === 'unknown' ? b.name || 'unknown' : b.type;
			return <div className="c-unknown c-sub rb-unknown">A block this view does not show ({name})</div>;
		}
	}
}

function Blocks({ list, c }: { list?: Block[]; c: Ctx }) {
	return <>{(list ?? []).map((b, i) => <BlockOf key={i} b={b} c={c} />)}</>;
}

// A rich message's blocks: in the chat (collapse) the first few with
// "Show all", on the message page all of them. An edit's version has
// no media, so its media blocks show their captions only.
export function RichBlocks({ d, chat, v, collapse = false, edit = false }: { d: S['RichDetail'] & { blocks: Block[] }; chat?: RichChat; v: Viewer; collapse?: boolean; edit?: boolean }) {
	const [all, setAll] = useState(!collapse);
	const c: Ctx = { chat, v, click: v.chat?.media === 'click', edit };
	const hidden = all ? 0 : Math.max(0, d.blocks.length - CHAT_BLOCKS);
	return (
		<div className="rb-page" dir={d.is_rtl ? 'rtl' : undefined}>
			<Blocks list={hidden ? d.blocks.slice(0, CHAT_BLOCKS) : d.blocks} c={c} />
			{hidden > 0 && <button type="button" className="rt-more rb-all" onClick={() => setAll(true)}>Show all ({hidden} more block{hidden === 1 ? '' : 's'})</button>}
			{!d.is_full && <div className="c-sub rb-partial">Only the start of this message was logged.</div>}
		</div>
	);
}
