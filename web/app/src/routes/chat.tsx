// The Telegram-like chat view (group or private chat), inside the shell,
// or on its own as the public page of a group exposed through MCP.
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactElement } from 'react';
import { ArrowLeft, CalendarDays, ChevronDown, CloudOff, Download, Forward, Globe, Info, LogIn, Pencil, Play, Radio, Search, Trash2, X } from 'lucide-react';
import Shell, { ThemeButton } from '../ui/Shell.tsx';
import { Link, setQuery, useNav } from '../nav.tsx';
import { Avatar } from '../ui/primitives.tsx';
import { duration } from '../ui/media.tsx';
import { fetchChatWindow } from '../api/chat.ts';
import { useLive, useLiveState, useRefetcher } from '../live/react.tsx';
import { bytes, fmt } from '../format.ts';
import { TYPE_LABEL } from './groups.tsx';
import type { PageData, Viewer } from '../page.ts';
import type { ChatMedia, ChatMessage, GroupSummary } from '../types.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

export type ChatProps = S['ChatProps'];

// Telegram's palette for sender names, picked per user id.
const NAME_COLORS = ['#e17076', '#eda86c', '#a695e7', '#7bc862', '#6ec9cb', '#65aadd', '#ee7aae'];
const nameColor = (id: number) => NAME_COLORS[Math.abs(id) % NAME_COLORS.length];

function dayKey(iso: string, v: Viewer): string {
	return new Intl.DateTimeFormat('en-CA', { timeZone: v.timezone || 'Asia/Jakarta' }).format(new Date(iso));
}
function dayLabel(iso: string, v: Viewer): string {
	return new Intl.DateTimeFormat('en-US', { weekday: 'short', month: 'long', day: 'numeric', year: 'numeric', timeZone: v.timezone || 'Asia/Jakarta' }).format(new Date(iso));
}
function hm(iso: string, v: Viewer): string {
	return new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit', hour12: v.date_format === '12h', timeZone: v.timezone || 'Asia/Jakarta' }).format(new Date(iso));
}

function Photo({ m, single }: { m: ChatMedia; single?: boolean }) {
	const ratio = m.w && m.h ? `${m.w} / ${m.h}` : '4 / 3';
	if (!m.stored) return <div className="c-media c-missing" style={{ aspectRatio: ratio }}><CloudOff size={22} /><span>Not stored</span></div>;
	return (
		<a className={`c-media ${single ? 'single' : ''}`} style={{ aspectRatio: ratio }} href={m.url ?? undefined} target="_blank" rel="noopener">
			{m.thumb ? <img src={m.thumb} alt="" loading="lazy" style={{ width: '100%', height: '100%', objectFit: 'cover', borderRadius: 'inherit' }} /> : null}
		</a>
	);
}

function Bubble({ m, v, isGroup, first, last, flash, onReply }: { m: ChatMessage; v: Viewer; isGroup: boolean; first: boolean; last: boolean; flash: boolean; onReply: (id: number) => void }) {
	const out = !m.sender;
	const ct = m.content_type;
	const sticker = ct === 'sticker';
	const mediaOnly = (ct === 'photo' || ct === 'video') && !m.text && !m.reply_to && !m.forward;
	const [edits, setEdits] = useState(false);
	const s = m.sender;
	return (
		<div id={`m${m.message_id}`} className={`c-row ${out ? 'out' : 'in'} ${last ? 'last' : ''} ${flash ? 'flash' : ''}`}>
			{isGroup && !out && <div className="c-avslot">{last && s && <Link to={`/users/${s.id}`} aria-label={s.name}><Avatar id={s.id} name={s.name} photo={s.photo} size={34} /></Link>}</div>}
			<div className={`c-bubble ${sticker ? 'sticker' : ''} ${mediaOnly ? 'mediaonly' : ''} ${m.deleted_at ? 'deleted' : ''} ${last ? 'tail' : ''}`}>
				{isGroup && s && first && !sticker && <Link to={`/users/${s.id}`} className="c-sender" style={{ color: nameColor(s.id) }}>{s.name}</Link>}
				{m.forward && <div className="c-fwd"><Forward size={13} />Forwarded from <b>{m.forward}</b></div>}
				{m.reply_to && (
					<a href={`?at=${m.reply_to.message_id}`} className="c-reply" style={{ ['--rc' as string]: nameColor(m.reply_to.sender_id) }}
						onClick={(e) => { e.preventDefault(); onReply(m.reply_to!.message_id); }}>
						<b>{m.reply_to.sender_name}</b><span>{m.reply_to.snippet}</span>
					</a>
				)}
				{ct === 'photo' && m.media && (m.album.length
					? <div className={`c-album n${Math.min(4, m.album.length + 1)}`}><Photo m={m.media} />{m.album.map((a, i) => <Photo key={i} m={a} />)}</div>
					: <Photo m={m.media} single />)}
				{ct === 'video' && m.media && (
					<div className="c-videowrap">
						<Photo m={m.media} single />
						{m.media.stored && <span className="c-playbig"><Play size={22} fill="currentColor" /></span>}
						{m.media.duration ? <span className="c-dur">{duration(m.media.duration)}{m.media.size ? ' . ' + bytes(m.media.size) : ''}</span> : null}
					</div>
				)}
				{(ct === 'document' || ct === 'audio') && m.media && (
					<a className="c-doc" href={m.media.url ?? undefined}>
						<span className="c-docicon">{m.media.stored ? <Download size={18} /> : <CloudOff size={18} />}</span>
						<span className="c-docmeta"><b>{m.media.name}</b><span className="c-sub">{bytes(m.media.size ?? 0)}{!m.media.stored && ' . not stored'}</span></span>
					</a>
				)}
				{sticker && m.media && <div className="c-sticker" role="img" aria-label="sticker">{m.media.emoji}</div>}
				{ct === 'voice' && m.media && (
					<div className="c-voice">
						<button type="button" className="c-play" aria-label="Play"><Play size={16} fill="currentColor" /></button>
						<div><div className="c-wave">{(m.media.wave ?? []).map((w, i) => <span key={i} style={{ height: 4 + w * 18 }} />)}</div><span className="c-sub">{duration(m.media.duration ?? 0)}</span></div>
					</div>
				)}
				{m.text && <div className="c-text">{m.text}</div>}
				{m.link_preview && <div className="c-lp"><b className="c-lp-site">{m.link_preview.site}</b><b>{m.link_preview.title}</b><span>{m.link_preview.desc}</span></div>}
				<div className="c-meta">
					{m.deleted_at && <span className="c-deltag" title={`Deleted ${m.deleted_at}`}><Trash2 size={11} />deleted</span>}
					{m.edits.length > 0 && <button type="button" className="c-edited" onClick={() => setEdits((x) => !x)} aria-expanded={edits}><Pencil size={10} />edited</button>}
					<Link to={`/messages/${m.scope}/${m.id}`} className="c-time" title={m.date}>{hm(m.date, v)}</Link>
				</div>
				{edits && (
					<div className="c-edits">
						{m.edits.map((e, i) => <div key={i}><span className="c-sub">{hm(e.at, v)}</span><div>{e.text}</div></div>)}
						<div><span className="c-sub">{m.edited_at ? hm(m.edited_at, v) : ''} (current)</span><div>{m.text}</div></div>
					</div>
				)}
			</div>
		</div>
	);
}

function Feed({ msgs, v, isGroup, flash, onReply }: { msgs: ChatMessage[]; v: Viewer; isGroup: boolean; flash: number | null; onReply: (id: number) => void }) {
	const out: ReactElement[] = [];
	let day = '';
	msgs.forEach((m, i) => {
		const d = dayKey(m.date, v);
		if (d !== day) {
			day = d;
			out.push(<div key={'d' + d} className="c-day"><span>{dayLabel(m.date, v)}</span></div>);
		}
		if (m.content_type === 'service') {
			out.push(<div key={m.id} id={`m${m.message_id}`} className="c-service"><span>{m.sender && <Link to={`/users/${m.sender.id}`}>{m.sender.name}</Link>} {m.text}</span></div>);
			return;
		}
		const same = (a?: ChatMessage) => !!a && a.content_type !== 'service' && dayKey(a.date, v) === d && (a.sender?.id ?? 0) === (m.sender?.id ?? 0);
		out.push(<Bubble key={m.id} m={m} v={v} isGroup={isGroup} first={!same(msgs[i - 1])} last={!same(msgs[i + 1])} flash={flash === m.message_id} onReply={onReply} />);
	});
	return <>{out}</>;
}

function Chat({ page }: { page: PageData<ChatProps> }) {
	const p = page.props;
	const v = page.viewer;
	const { navigate } = useNav();
	const publicView = !v.signed_in;
	const isGroup = p.chat.kind === 'group';
	const chatId = p.chat.kind === 'group' ? p.chat.group.id : p.chat.user.id;
	const scope = isGroup ? 'group' : 'private';
	const [msgs, setMsgs] = useState(p.messages);
	const [older, setOlder] = useState(p.has_older);
	const [newer, setNewer] = useState(p.has_newer);
	const [live, setLive] = useState(v.chat?.live ?? true);
	const [atBottom, setAtBottom] = useState(true);
	const [searching, setSearching] = useState(false);
	const [flash, setFlash] = useState<number | null>(p.at);
	const [unseen, setUnseen] = useState(0);
	const scroller = useRef<HTMLDivElement>(null);
	const busy = useRef(false);
	const msgsRef = useRef(msgs);
	msgsRef.current = msgs;
	const liveState = useLiveState();
	const refetch = useRefetcher();

	// A new page (another chat, or ?at=) replaces the window.
	useEffect(() => { setMsgs(p.messages); setOlder(p.has_older); setNewer(p.has_newer); setFlash(p.at); setUnseen(0); }, [p]);

	useLayoutEffect(() => {
		const el = scroller.current;
		if (!el) return;
		const t = p.at !== null ? document.getElementById('m' + p.at) : null;
		if (t) t.scrollIntoView({ block: 'center' });
		else el.scrollTop = el.scrollHeight;
	}, [p]);

	const loadOlder = useCallback(async () => {
		const el = scroller.current;
		const first = msgs[0];
		if (!el || !first || !older || busy.current) return;
		busy.current = true;
		try {
			const w = await fetchChatWindow(scope, chatId, { before: first.message_id });
			const h = el.scrollHeight;
			setMsgs((cur) => [...w.messages, ...cur]);
			setOlder(w.has_older);
			requestAnimationFrame(() => { el.scrollTop += el.scrollHeight - h; });
		} finally { busy.current = false; }
	}, [msgs, older, scope, chatId]);

	useEffect(() => {
		const el = scroller.current;
		if (!el) return;
		const on = () => {
			const bottom = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
			setAtBottom(bottom);
			if (bottom) setUnseen(0);
			if (el.scrollTop < 300) void loadOlder();
		};
		el.addEventListener('scroll', on, { passive: true });
		return () => el.removeEventListener('scroll', on);
	}, [loadOlder]);

	// The live tail (web-live-client.md section 4). New messages are
	// fetched from the cursor and appended: at the bottom the view
	// follows them, scrolled up nothing moves and the jump button counts
	// them. Edits, deletions and stored files replace the message in
	// place. The stream drives it; while it is down, poll every second.
	const fetchNewer = useCallback(async () => {
		for (let pages = 0; pages < 3; pages++) {
			const last = msgsRef.current[msgsRef.current.length - 1];
			if (!last) return;
			const w = await fetchChatWindow(scope, chatId, { after: last.message_id });
			const el = scroller.current;
			const stick = el ? el.scrollHeight - el.scrollTop - el.clientHeight < 80 : false;
			const have = new Set(msgsRef.current.map((c) => c.id));
			const add = w.messages.filter((x) => !have.has(x.id));
			if (add.length) {
				msgsRef.current = [...msgsRef.current, ...add];
				setMsgs(msgsRef.current);
				if (stick && el) requestAnimationFrame(() => { el.scrollTop = el.scrollHeight; });
				else setUnseen((n) => n + add.length);
			}
			if (!w.has_newer) return;
		}
		// Too far behind to follow: leave the window, as when scrolled back.
		setNewer(true);
	}, [scope, chatId]);

	const replaceMessage = useCallback(async (messageId: number) => {
		const w = await fetchChatWindow(scope, chatId, { around: messageId }, 3);
		const got = new Map(w.messages.map((m) => [m.id, m]));
		setMsgs((cur) => cur.map((m) => got.get(m.id) ?? m));
	}, [scope, chatId]);

	const following = live && !newer && !publicView;
	useLive(following ? [`chat:${chatId}`] : null, (e) => {
		const run = (key: string, fn: () => Promise<unknown>) => (refetch ? refetch.request(key, 250, fn) : void fn().catch(() => {}));
		if (e.kind === 'message.new' || e.kind === 'resync') run(`chat:${chatId}:tail`, fetchNewer);
		else if ((e.kind === 'message.edited' || e.kind === 'message.deleted' || e.kind === 'file.stored') && e.message_id !== undefined && msgsRef.current.some((m) => m.message_id === e.message_id)) {
			const id = e.message_id;
			run(`chat:${chatId}:m${id}`, () => replaceMessage(id));
		}
	});

	const polling = following && (liveState === 'polling' || liveState === 'idle');
	useEffect(() => {
		if (!polling) return;
		const t = setInterval(() => { if (!document.hidden) void fetchNewer().catch(() => {}); }, 1000);
		return () => clearInterval(t);
	}, [polling, fetchNewer]);

	const onReply = (id: number) => {
		const t = document.getElementById('m' + id);
		if (t) { t.scrollIntoView({ block: 'center', behavior: 'smooth' }); setFlash(null); requestAnimationFrame(() => setFlash(id)); }
		else navigate(setQuery(page.url, { at: id, date: undefined }), { replace: true, scroll: false });
	};
	const toLatest = () => {
		if (newer) navigate(setQuery(page.url, { at: undefined, date: undefined }), { replace: true, scroll: false });
		else scroller.current?.scrollTo({ top: scroller.current.scrollHeight, behavior: 'smooth' });
	};

	const title = p.chat.kind === 'group' ? p.chat.group.title : p.chat.user.name;
	const photo = p.chat.kind === 'group' ? p.chat.group.photo : p.chat.user.photo;
	const back = isGroup ? `/groups/${chatId}` : `/users/${chatId}`;
	const sub = p.chat.kind === 'group'
		? `${TYPE_LABEL[p.chat.group.type]} . ${fmt(p.chat.group.members)} members . ${fmt(p.chat.group.msg_count)} messages`
		: `private chat with the logging account . ${fmt(p.chat.count)} messages`;
	const body = (
		<div className={`c-chat ${publicView ? 'public' : ''}`}>
			<header className="c-head">
				{!publicView && <Link to={back} className="c-iconbtn" aria-label="Back to details"><ArrowLeft size={20} /></Link>}
				<Avatar id={chatId} name={title} photo={photo} size={40} square={isGroup} />
				<div className="c-head-title"><h1>{title}</h1><span className="c-sub">{sub}</span></div>
				{!publicView && (
					<button type="button" className={`c-live ${live ? 'on' : ''}`} onClick={() => setLive((x) => !x)} aria-pressed={live} title="Show new messages as they are logged">
						<Radio size={14} />{live ? 'Live' : 'Paused'}
					</button>
				)}
				<button type="button" className="c-iconbtn" onClick={() => setSearching((x) => !x)} aria-label="Search in chat"><Search size={19} /></button>
				<label className="c-iconbtn c-datebtn" aria-label="Jump to date" title="Jump to date">
					<CalendarDays size={19} />
					<input type="date" onChange={(e) => e.target.value && navigate(setQuery(page.url, { date: e.target.value, at: undefined }), { replace: true, scroll: false })} />
				</label>
				{!publicView && <Link to={back} className="c-iconbtn c-hide-sm" aria-label="Details"><Info size={19} /></Link>}
			</header>
			{searching && (
				<form className="c-searchbar" onSubmit={(e) => {
					e.preventDefault();
					const text = new FormData(e.currentTarget).get('q');
					navigate(`/${isGroup ? 'group' : 'private'}_messages?q=${encodeURIComponent(String(text ?? ''))}&f=${encodeURIComponent(JSON.stringify([{ field: 'chat_id', op: '=', value: String(chatId) }]))}`);
				}}>
					<Search size={15} />
					<input name="q" autoFocus placeholder={`Search in ${title}`} />
					<button type="button" className="c-iconbtn" onClick={() => setSearching(false)} aria-label="Close search"><X size={16} /></button>
				</form>
			)}
			<div className="c-scroll" ref={scroller}>
				<div className="c-feed">
					{older ? <div className="c-loading">Loading older messages...</div> : <div className="c-start">Beginning of the archived history</div>}
					<Feed msgs={msgs} v={v} isGroup={isGroup} flash={flash} onReply={onReply} />
				</div>
			</div>
			{(!atBottom || newer) && <button type="button" className="c-tobottom" aria-label={unseen ? `Jump to latest, ${unseen} new` : 'Jump to latest'} onClick={toLatest}><ChevronDown size={22} />{unseen > 0 && <span className="c-unseen">{unseen > 99 ? '99+' : unseen}</span>}</button>}
			<footer className="c-readonly">
				{publicView ? <><Globe size={14} />Public archive of {title}. Read only.</> : <>Archive view, read only. {!live ? 'Live updates paused.' : liveState === 'polling' ? 'Live updates unavailable; checking for new messages every second.' : 'New messages appear as they are logged.'}</>}
			</footer>
		</div>
	);
	if (publicView) {
		return (
			<div className="app c-publicpage">
				<header className="c-pubtop">
					<span className="a-logo">tg</span><b>tgloggerd</b><span className="c-sub">public archive</span>
					<span className="a-grow" />
					<ThemeButton viewer={v} />
					<a href={`/login?next=${encodeURIComponent(page.url)}`} className="a-btn a-btn-ghost"><LogIn size={14} />Sign in</a>
				</header>
				{body}
			</div>
		);
	}
	return <Shell flush crumbs={[[isGroup ? 'Groups' : 'Users', isGroup ? '/groups' : '/users'], [title, back], ['Chat', null]]}>{body}</Shell>;
}

export const chatRoute: RouteDef<ChatProps> = {
	Page: Chat,
	head: (p) => {
		const c = p.props.chat;
		const title = c.kind === 'group' ? c.group.title : c.user.name;
		// Only the public pages of exposed groups are for search engines.
		const pub = !p.viewer.signed_in && c.kind === 'group';
		return {
			title: `${title} - tgloggerd`,
			description: pub ? `Public archive of ${title}` : undefined,
			robots: pub ? undefined : 'noindex',
			canonical: pub ? p.url.split('?')[0] : undefined,
			og: pub ? { title, type: 'website' } : undefined,
		};
	},
};
