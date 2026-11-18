// The Telegram-like chat view (group or private chat), inside the shell,
// or on its own as the public page of a group exposed through MCP.
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactElement } from 'react';
import { track } from '../ui/busy.ts';
import { ArrowLeft, CalendarDays, ChevronDown, ChevronsUp, ExternalLink, CloudOff, Download, Forward, Globe, Info, Pencil, Play, Radio, Search, Trash2, X } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { PublicFrame } from '../ui/PublicFrame.tsx';
import { Link, setQuery, useNav } from '../nav.tsx';
import { Avatar } from '../ui/primitives.tsx';
import { MessageMedia } from './chat-media.tsx';
import { fetchChatWindow } from '../api/chat.ts';
import { useLive, useLiveState, useRefetcher } from '../live/react.tsx';
import { useFreshGroup, useFreshUser } from '../live/useEntities.tsx';
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

// The viewer's chat preferences (Settings, Preferences); each one's site
// default is on, except media, which loads automatically.
const prefs = (v: Viewer) => ({
	autoLoad: v.chat?.auto_load !== false,
	mediaOnClick: v.chat?.media === 'click',
	showDeleted: v.chat?.show_deleted !== false,
	showEdited: v.chat?.show_edited !== false,
	showService: v.chat?.show_service !== false,
});

// A chat's page, opened at a message when given: a negative id is a
// group, a positive one a private chat.
const chatPath = (id: number, at?: number) => `${id < 0 ? '/groups' : '/users'}/${id}/chat${at !== undefined ? `?at=${at}` : ''}`;

// A service message's type as words, when it came without its text.
const serviceText = (t?: string) => (t ? t.replace(/_/g, ' ').replace(/^chat /, '') : '');

// Where a forward came from: a link to the user, or to the channel's
// post (or the channel), else just the name.
function ForwardLine({ m }: { m: ChatMessage }) {
	const f = m.forward_from;
	const name = f?.name ?? m.forward ?? '';
	const to = f?.user_id !== undefined ? `/users/${f.user_id}` : f?.chat_id !== undefined ? (f.message_id !== undefined ? chatPath(f.chat_id, f.message_id) : `/groups/${f.chat_id}`) : null;
	return (
		<div className="c-fwd" title={f?.date ? `Sent ${f.date}` : undefined}>
			<Forward size={13} />Forwarded from {to ? <Link to={to} className="c-fwdlink"><b>{name}</b></Link> : <b>{name}</b>}
		</div>
	);
}

// An earlier version's media in the edit list: its picture, small, or
// its type and name.
function EditMedia({ m, type }: { m: NonNullable<ChatMessage['edits'][number]['media']>; type: string }) {
	if (m.thumb) return <a className="c-editmedia" href={m.url ?? undefined} target="_blank" rel="noopener" aria-label={`Earlier ${type}`}><img src={m.thumb} alt="" loading="lazy" /></a>;
	return <div className="c-editfile c-sub">{type}{m.name ? `: ${m.name}` : ''}{m.url ? '' : ' (not stored)'}</div>;
}

// A link preview: its site, title and description, and its picture,
// linking to the page.
function LinkPreview({ lp }: { lp: NonNullable<ChatMessage['link_preview']> }) {
	const inner = (
		<>
			<b className="c-lp-site">{lp.site}</b><b>{lp.title}</b><span>{lp.desc}</span>
			{lp.media?.thumb && <img className="c-lp-img" src={lp.media.thumb} alt="" loading="lazy" style={lp.media.w && lp.media.h ? { aspectRatio: `${lp.media.w} / ${lp.media.h}` } : undefined} />}
		</>
	);
	return lp.url
		? <a className="c-lp" href={lp.url} target="_blank" rel="noopener noreferrer nofollow">{inner}<span className="c-lp-url"><ExternalLink size={11} />{lp.url.replace(/^https?:\/\//, '')}</span></a>
		: <div className="c-lp">{inner}</div>;
}

function Bubble({ m, v, isGroup, first, last, flash, onReply }: { m: ChatMessage; v: Viewer; isGroup: boolean; first: boolean; last: boolean; flash: boolean; onReply: (id: number) => void }) {
	const out = !m.sender;
	const ct = m.content_type;
	const sticker = ct === 'sticker';
	const mediaOnly = (ct === 'photo' || ct === 'video' || ct === 'animation') && !m.text && !m.reply_to && !m.forward;
	const [edits, setEdits] = useState(false);
	const s = useFreshUser(m.sender);
	const pref = prefs(v);
	// A message posted as a chat (a channel, an anonymous admin) links to
	// that chat, not to a user page.
	const asChat = m.sender_chat;
	// A chat the viewer may not see comes as a sender without an id
	// ("(hidden chat)"): named, but linked nowhere.
	const senderTo = asChat ? `/groups/${asChat.id}` : s && s.id !== 0 ? `/users/${s.id}` : '';
	const r = m.reply_to;
	const replyTo = r?.chat_id !== undefined ? chatPath(r.chat_id, r.message_id) : r ? `?at=${r.message_id}` : '';
	return (
		<div id={`m${m.message_id}`} className={`c-row ${out ? 'out' : 'in'} ${last ? 'last' : ''} ${flash ? 'flash' : ''}`}>
			{isGroup && !out && <div className="c-avslot">{last && s && (senderTo
					? <Link to={senderTo} aria-label={asChat ? asChat.title : s.name}>{asChat ? <Avatar id={asChat.id} name={asChat.title} photo={asChat.photo} size={34} square /> : <Avatar id={s.id} name={s.name} photo={s.photo} size={34} />}</Link>
					: <Avatar id={0} name={s.name} photo={null} size={34} square />)}</div>}
			<div className={`c-bubble ${sticker ? 'sticker' : ''} ${mediaOnly ? 'mediaonly' : ''} ${m.deleted_at ? 'deleted' : ''} ${last ? 'tail' : ''}`}>
				{isGroup && s && first && !sticker && (senderTo
					? <Link to={senderTo} className="c-sender" style={{ color: nameColor(s.id) }}>{asChat ? asChat.title : s.name}</Link>
					: <span className="c-sender c-sender-hidden">{s.name}</span>)}
				{(m.forward_from || m.forward) && <ForwardLine m={m} />}
				{r && (r.chat_id !== undefined
					// A reply to another chat opens that chat at the message.
					? <Link to={replyTo} className="c-reply c-reply-other" style={{ ['--rc' as string]: nameColor(r.sender_id) }}>
						<b>{r.resolved === false ? `message #${r.message_id}` : r.sender_name}<span className="c-sub"> . in another chat</span></b><span>{r.snippet}{r.deleted ? ' (deleted)' : ''}</span>
					</Link>
					: <a href={replyTo} className="c-reply" style={{ ['--rc' as string]: nameColor(r.sender_id) }}
						onClick={(e) => { e.preventDefault(); onReply(r.message_id); }}>
						<b>{r.resolved === false ? `message #${r.message_id}` : r.sender_name}</b>
						{r.resolved === false ? <span className="c-sub">not in the archive</span> : <span className={r.deleted ? 'c-replydel' : ''}>{r.snippet}{r.deleted ? ' (deleted)' : ''}</span>}
					</a>)}
				<MessageMedia msg={m} onClick={pref.mediaOnClick} />
				{m.content_type === 'unknown' && <div className="c-unknown c-sub">{m.tdlib_type ? `A kind of message this view does not show yet (${m.tdlib_type})` : 'A kind of message this view does not show'}</div>}
				{m.text && <div className="c-text">{m.text}</div>}
				{m.link_preview && <LinkPreview lp={m.link_preview} />}
				{m.author_signature && <div className="c-sign">- {m.author_signature}</div>}
				<div className="c-meta">
					{m.deleted_at && <span className="c-deltag" title={`Deleted ${m.deleted_at}`}><Trash2 size={11} />deleted</span>}
					{pref.showEdited && m.edits.length > 0 && <button type="button" className="c-edited" onClick={() => setEdits((x) => !x)} aria-expanded={edits}><Pencil size={10} />edited</button>}
					{/* Edited, but no earlier version was recorded. */}
					{pref.showEdited && m.edits.length === 0 && m.edited_at && <span className="c-edited" title={`Edited ${m.edited_at}; no earlier version recorded`}><Pencil size={10} />edited</span>}
					<Link to={`/messages/${m.scope}/${m.id}`} className="c-time" title={m.date}>{hm(m.date, v)}</Link>
				</div>
				{edits && (
					<div className="c-editswrap"><div className="c-edits">
						{m.edits.map((e, i) => (
							<div key={i}>
								<span className="c-sub">{hm(e.at, v)}{e.content_type && e.content_type !== m.content_type ? ` . was ${e.content_type}` : ''}</span>
								{e.media && <EditMedia m={e.media} type={e.content_type ?? m.content_type} />}
								{e.text ? <div>{e.text}</div> : !e.media && <div className="c-sub">(no text)</div>}
							</div>
						))}
						<div><span className="c-sub">{m.edited_at ? hm(m.edited_at, v) : ''} (current)</span><div>{m.text}</div></div>
					</div></div>
				)}
			</div>
		</div>
	);
}

function Feed({ msgs: all, v, isGroup, flash, onReply }: { msgs: ChatMessage[]; v: Viewer; isGroup: boolean; flash: number | null; onReply: (id: number) => void }) {
	const pref = prefs(v);
	const msgs = all.filter((m) => (pref.showDeleted || !m.deleted_at) && (pref.showService || m.content_type !== 'service'));
	const out: ReactElement[] = [];
	let day = '';
	msgs.forEach((m, i) => {
		const d = dayKey(m.date, v);
		if (d !== day) {
			day = d;
			out.push(<div key={'d' + d} className="c-day"><span>{dayLabel(m.date, v)}</span></div>);
		}
		if (m.content_type === 'service') {
			// The actor links to its user, or to its chat when it acted as
			// one; the text falls back to the action's type.
			const actor = m.sender_chat ? <Link to={`/groups/${m.sender_chat.id}`}>{m.sender_chat.title}</Link> : m.sender ? (m.sender.id !== 0 ? <Link to={`/users/${m.sender.id}`}>{m.sender.name}</Link> : <span>{m.sender.name}</span>) : null;
			out.push(
				<div key={m.id} id={`m${m.message_id}`} className={`c-service ${m.deleted_at ? 'deleted' : ''}`}>
					<span>{actor} {m.text || serviceText(m.service_type)} <Link to={`/messages/${m.scope}/${m.id}`} className="c-svctime" title={m.date}>{hm(m.date, v)}</Link>{m.deleted_at && <span className="c-deltag" title={`Deleted ${m.deleted_at}`}><Trash2 size={11} />deleted</span>}</span>
				</div>,
			);
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
	const group = useFreshGroup(p.chat.kind === 'group' ? p.chat.group : null);
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
	// A scroll to make once the next messages are rendered: keep the view
	// where it was after older ones went in above it, or follow new ones
	// at the bottom. Done in a layout effect, not a frame callback, which
	// can run before React has rendered them.
	const afterRender = useRef<{ prepend: number } | { bottom: true } | null>(null);
	useLayoutEffect(() => {
		const el = scroller.current;
		const a = afterRender.current;
		afterRender.current = null;
		if (!el || !a) return;
		if ('prepend' in a) el.scrollTop += el.scrollHeight - a.prepend;
		else el.scrollTop = el.scrollHeight;
	}, [msgs]);
	const liveState = useLiveState();
	const refetch = useRefetcher();

	// The Drogon chat's #msg-<message id> anchors (bookmarks, links from
	// elsewhere) never reach the server: open the chat at that message.
	useEffect(() => {
		const h = /^#msg-(\d+)$/.exec(location.hash);
		if (h) navigate(setQuery(page.url, { at: Number(h[1]), date: undefined, after: undefined, after_ts: undefined }), { replace: true, scroll: false });
	}, []);

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
			const w = await track(fetchChatWindow(scope, chatId, { before: first.message_id }));
			// Once the older messages are rendered, keep the reader where
			// they were (see the layout effect below).
			afterRender.current = { prepend: el.scrollHeight };
			setMsgs((cur) => [...w.messages, ...cur]);
			setOlder(w.has_older);
		} finally { busy.current = false; }
	}, [msgs, older, scope, chatId]);

	useEffect(() => {
		const el = scroller.current;
		if (!el) return;
		const on = () => {
			const bottom = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
			setAtBottom(bottom);
			if (bottom) setUnseen(0);
			if (el.scrollTop < 300 && prefs(v).autoLoad) void loadOlder();
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
				if (stick && el) afterRender.current = { bottom: true };
				else setUnseen((n) => n + add.length);
				setMsgs(msgsRef.current);
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
		else navigate(setQuery(page.url, { at: id, date: undefined, after: undefined, after_ts: undefined }), { replace: true, scroll: false });
	};
	const toLatest = () => {
		if (newer) navigate(setQuery(page.url, { at: undefined, date: undefined, after: undefined, after_ts: undefined }), { replace: true, scroll: false });
		else scroller.current?.scrollTo({ top: scroller.current.scrollHeight, behavior: 'smooth' });
	};

	const title = group ? group.title : p.chat.kind === 'user' ? p.chat.user.name : '';
	const photo = group ? group.photo : p.chat.kind === 'user' ? p.chat.user.photo : null;
	const back = isGroup ? `/groups/${chatId}` : `/users/${chatId}`;
	// Signed out, only an exposed group is shown: its public page.
	const info = publicView ? (isGroup ? back : null) : back;
	const sub = p.chat.kind === 'group'
		? `${TYPE_LABEL[p.chat.group.type]}${p.chat.group.members ? ` . ${fmt(p.chat.group.members)} members seen` : ''} . ${fmt(group?.msg_count ?? p.chat.group.msg_count)} messages`
		: `private chat with the logging account . ${fmt(p.chat.count)} messages`;
	const body = (
		<div className={`c-chat ${publicView ? 'public' : ''}`}>
			<header className="c-head">
				{!publicView && <Link to={back} className="c-iconbtn" aria-label="Back to details"><ArrowLeft size={20} /></Link>}
				{/* The chat's name opens its details: the group's page, or the
				    profile of the user a private chat is with. */}
				{info ? (
					<Link to={info} className="c-head-link" title={isGroup ? 'Group details' : 'Profile'}>
						<Avatar id={chatId} name={title} photo={photo} size={40} square={isGroup} />
						<div className="c-head-title"><h1>{title}</h1><span className="c-sub">{sub}</span></div>
					</Link>
				) : (
					<>
						<Avatar id={chatId} name={title} photo={photo} size={40} square={isGroup} />
						<div className="c-head-title"><h1>{title}</h1><span className="c-sub">{sub}</span></div>
					</>
				)}
				{!publicView && (
					<button type="button" className={`c-live ${live ? 'on' : ''}`} onClick={() => setLive((x) => !x)} aria-pressed={live} title="Show new messages as they are logged">
						<Radio size={14} />{live ? 'Live' : 'Paused'}
					</button>
				)}
				<button type="button" className="c-iconbtn" onClick={() => setSearching((x) => !x)} aria-label="Search in chat"><Search size={19} /></button>
				<button type="button" className="c-iconbtn" onClick={() => navigate(setQuery(page.url, { after: 0, at: undefined, date: undefined, after_ts: undefined }), { replace: true, scroll: false })} aria-label="Jump to the oldest messages" title="Oldest messages"><ChevronsUp size={19} /></button>
				<label className="c-iconbtn c-datebtn" aria-label="Jump to date and time" title="Jump to a date and time">
					<CalendarDays size={19} />
					{/* The server reads the time in the viewer's timezone. */}
					<input type="datetime-local" onChange={(e) => e.target.value && navigate(setQuery(page.url, { date: e.target.value, at: undefined, after: undefined, after_ts: undefined }), { replace: true, scroll: false })} />
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
					{older ? (prefs(v).autoLoad ? <div className="c-loading">Loading older messages...</div> : <div className="c-loading"><button type="button" className="a-link" onClick={() => void loadOlder()}>Load older messages</button></div>) : <div className="c-start">Beginning of the archived history</div>}
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
			<PublicFrame viewer={v} url={page.url}>{body}</PublicFrame>
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
