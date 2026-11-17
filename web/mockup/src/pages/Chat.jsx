// Telegram-like chat view, inside the console shell (or standalone for
// the public, signed-out view of an MCP-exposed group).
import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import {
	ArrowLeft, Search, CalendarDays, Info, ChevronDown, Forward, Play, FileText,
	Download, Pencil, Trash2, CloudOff, Radio, Globe, LogIn, X, Mic,
} from 'lucide-react';
import { Link, useQuery } from '../router.jsx';
import Shell, { ThemeButton } from '../ui/Shell.jsx';
import { Avatar } from '../ui/primitives.jsx';
import { duration } from '../ui/Media.jsx';
import { chatStream, chatOf, senderOf } from '../data/messages.js';
import { bytes, fmt, dayLabel } from '../data/seed.js';
import { TYPE_LABEL } from './Groups.jsx';
import { snippet } from './Messages.jsx';
import NotFound from './NotFound.jsx';
import { useViewer } from '../viewer.jsx';

const PAGE = 60;

// Telegram's own palette for sender names, picked per user.
const NAME_COLORS = ['#e17076', '#eda86c', '#a695e7', '#7bc862', '#6ec9cb', '#65aadd', '#ee7aae'];
const nameColor = (u) => NAME_COLORS[Math.abs(Number(u.id)) % NAME_COLORS.length];

function Photo({ media, single }) {
	const ratio = media.w && media.h ? `${media.w} / ${media.h}` : '4 / 3';
	if (!media.stored) {
		return <div className="c-media c-missing" style={{ aspectRatio: ratio }}><CloudOff size={22} /><span>Not stored</span></div>;
	}
	return (
		<button type="button" className={`c-media ${single ? 'single' : ''}`} style={{ aspectRatio: ratio, background: `linear-gradient(135deg, hsl(${media.hue} 65% 66%), hsl(${(media.hue + 50) % 360} 55% 42%))` }} aria-label="Open photo" />
	);
}

function Voice({ media }) {
	return (
		<div className="c-voice">
			<button type="button" className="c-play" aria-label="Play"><Play size={16} fill="currentColor" /></button>
			<div>
				<div className="c-wave">{media.wave.map((v, i) => <span key={i} style={{ height: 4 + v * 18 }} />)}</div>
				<span className="c-sub">{duration(media.duration)}</span>
			</div>
		</div>
	);
}

function Bubble({ m, chat, first, last, flash }) {
	const s = senderOf(m);
	const out = m.is_outgoing;
	const ct = m.content_type;
	const sticker = ct === 'sticker';
	const mediaOnly = (ct === 'photo' || ct === 'video') && !m.text && !m.reply_to && !m.forward;
	const [showEdits, setShowEdits] = useState(false);
	return (
		<div id={`m${m.message_id}`} className={`c-row ${out ? 'out' : 'in'} ${last ? 'last' : ''} ${flash ? 'flash' : ''}`}>
			{chat.kind === 'group' && !out && (
				<div className="c-avslot">{last && <Link to={`/users/${s.id}`} aria-label={s.name}><Avatar e={s} size={34} /></Link>}</div>
			)}
			<div className={`c-bubble ${sticker ? 'sticker' : ''} ${mediaOnly ? 'mediaonly' : ''} ${m.deleted_at ? 'deleted' : ''} ${last ? 'tail' : ''}`}>
				{chat.kind === 'group' && !out && first && !sticker && (
					<Link to={`/users/${s.id}`} className="c-sender" style={{ color: nameColor(s) }}>{s.name}</Link>
				)}
				{m.forward && <div className="c-fwd"><Forward size={13} />Forwarded from <b>{m.forward.from}</b></div>}
				{m.reply_to && (
					<a href={`#m${m.reply_to.message_id}`} className="c-reply" style={{ '--rc': nameColor(senderOf(m.reply_to) || { id: 0 }) }}>
						<b>{senderOf(m.reply_to)?.name}</b>
						<span>{snippet(m.reply_to)}</span>
					</a>
				)}
				{ct === 'photo' && (m.album
					? <div className={`c-album n${Math.min(4, m.album.length + 1)}`}><Photo media={m.media} />{m.album.map((a, i) => <Photo key={i} media={{ ...a, stored: true }} />)}</div>
					: <Photo media={m.media} single />)}
				{ct === 'video' && (
					<div className="c-videowrap">
						<Photo media={m.media} single />
						{m.media.stored && <span className="c-playbig"><Play size={22} fill="currentColor" /></span>}
						<span className="c-dur">{duration(m.media.duration)} . {bytes(m.media.size)}</span>
					</div>
				)}
				{(ct === 'document' || ct === 'audio') && (
					<div className="c-doc">
						<span className="c-docicon">{m.media.stored ? <Download size={18} /> : <CloudOff size={18} />}</span>
						<div className="c-docmeta"><b>{m.media.name}</b><span className="c-sub">{bytes(m.media.size)}{!m.media.stored && ' . not stored'}</span></div>
					</div>
				)}
				{sticker && <div className="c-sticker" role="img" aria-label="sticker">{m.media.emoji}</div>}
				{ct === 'voice' && <Voice media={m.media} />}
				{m.text && <div className="c-text">{m.text}</div>}
				{m.link_preview && (
					<div className="c-lp">
						<b className="c-lp-site">{m.link_preview.site}</b>
						<b>{m.link_preview.title}</b>
						<span>{m.link_preview.desc}</span>
					</div>
				)}
				<div className="c-meta">
					{m.deleted_at && <span className="c-deltag" title={`Deleted at ${m.deleted_at}`}><Trash2 size={11} />deleted</span>}
					{m.edits && <button type="button" className="c-edited" onClick={() => setShowEdits((v) => !v)} aria-expanded={showEdits}><Pencil size={10} />edited</button>}
					<Link to={`/messages/${m.scope}/${m.id}`} className="c-time" title={m.date}>{m.date.slice(11, 16)}</Link>
				</div>
				{showEdits && (
					<div className="c-edits">
						{m.edits.map((e, i) => <div key={i}><span className="c-sub">{e.at.slice(0, 16)}</span><div>{e.text}</div></div>)}
						<div><span className="c-sub">{m.edited_at?.slice(0, 16)} (current)</span><div>{m.text}</div></div>
					</div>
				)}
			</div>
		</div>
	);
}

function Feed({ chat, msgs, at }) {
	const items = [];
	let day = '';
	msgs.forEach((m, i) => {
		const d = m.date.slice(0, 10);
		if (d !== day) {
			day = d;
			items.push(<div key={'d' + d} className="c-day"><span>{dayLabel(d)}</span></div>);
		}
		if (m.content_type === 'service') {
			const s = senderOf(m);
			items.push(<div key={m.id} id={`m${m.message_id}`} className="c-service"><span><Link to={`/users/${s.id}`}>{s.name}</Link> {m.text}</span></div>);
			return;
		}
		const prev = msgs[i - 1], next = msgs[i + 1];
		const same = (a) => a && a.content_type !== 'service' && a.date.slice(0, 10) === d && a.is_outgoing === m.is_outgoing && senderOf(a)?.id === senderOf(m)?.id;
		items.push(<Bubble key={m.id} m={m} chat={chat} first={!same(prev)} last={!same(next)} flash={String(m.message_id) === at} />);
	});
	return items;
}

export default function Chat({ kind, id, publicView }) {
	const chat = chatOf(kind, id);
	const v = useViewer();
	const [p, set] = useQuery();
	const [live, setLive] = useState(true);
	const [searching, setSearching] = useState(false);
	const scroller = useRef(null);
	const all = chat ? chatStream(chat) : [];
	// Window of messages: either the latest page, or a page around ?at=.
	const atIdx = p.at ? all.findIndex((m) => String(m.message_id) === p.at) : -1;
	const [from, setFrom] = useState(() => Math.max(0, (atIdx >= 0 ? atIdx - PAGE / 2 : all.length - PAGE)));
	const msgs = all.slice(from, atIdx >= 0 ? Math.min(all.length, atIdx + PAGE / 2) : all.length);
	const [atBottom, setAtBottom] = useState(true);

	useLayoutEffect(() => {
		const el = scroller.current;
		if (!el) return;
		const target = p.at && document.getElementById('m' + p.at);
		if (target) target.scrollIntoView({ block: 'center' });
		else el.scrollTop = el.scrollHeight;
	}, [id, p.at]); // eslint-disable-line react-hooks/exhaustive-deps

	useEffect(() => {
		const el = scroller.current;
		if (!el) return;
		const on = () => {
			setAtBottom(el.scrollHeight - el.scrollTop - el.clientHeight < 80);
			if (el.scrollTop < 200 && from > 0) {
				const h = el.scrollHeight;
				setFrom((f) => Math.max(0, f - PAGE));
				requestAnimationFrame(() => { el.scrollTop += el.scrollHeight - h; });
			}
		};
		el.addEventListener('scroll', on, { passive: true });
		return () => el.removeEventListener('scroll', on);
	}, [from]);

	if (!chat || (kind === 'group' && !publicView && !v.group(chat))) return <NotFound what="chat" />;
	const title = chat.title || chat.name;
	const back = kind === 'group' ? `/groups/${chat.id}` : `/users/${chat.id}`;
	const sub = kind === 'group'
		? `${TYPE_LABEL[chat.type]} . ${fmt(chat.members)} members . ${fmt(chat.msg_count)} messages`
		: `private chat with the logger account . ${fmt(all.length)} messages`;
	const body = (
		<div className={`c-chat ${publicView ? 'public' : ''}`}>
			<header className="c-head">
				{!publicView && <Link to={back} className="c-iconbtn" aria-label="Back to details"><ArrowLeft size={20} /></Link>}
				<Avatar e={chat} size={40} />
				<div className="c-head-title">
					<h1>{title}</h1>
					<span className="c-sub">{sub}</span>
				</div>
				{!publicView && (
					<button type="button" className={`c-live ${live ? 'on' : ''}`} onClick={() => setLive((v) => !v)} aria-pressed={live} title="Load new messages as they arrive">
						<Radio size={14} />{live ? 'Live' : 'Paused'}
					</button>
				)}
				<button type="button" className="c-iconbtn" onClick={() => setSearching((v) => !v)} aria-label="Search in chat"><Search size={19} /></button>
				<label className="c-iconbtn c-datebtn" aria-label="Jump to date" title="Jump to date">
					<CalendarDays size={19} />
					<input type="date" min={all[0]?.date.slice(0, 10)} max={all[all.length - 1]?.date.slice(0, 10)}
						onChange={(e) => {
							const m = all.find((x) => x.date.slice(0, 10) >= e.target.value);
							if (m) { setFrom(Math.max(0, all.indexOf(m) - PAGE / 2)); set({ at: String(m.message_id) }, { replace: true }); }
						}} />
				</label>
				{!publicView && <Link to={back} className="c-iconbtn c-hide-sm" aria-label="Details"><Info size={19} /></Link>}
			</header>
			{searching && (
				<div className="c-searchbar">
					<Search size={15} />
					<input autoFocus placeholder={`Search in ${title}`} />
					<span className="c-sub">Enter to search . results open in Group messages</span>
					<button type="button" className="c-iconbtn" onClick={() => setSearching(false)} aria-label="Close search"><X size={16} /></button>
				</div>
			)}
			<div className="c-scroll" ref={scroller}>
				<div className="c-feed">
					{from > 0 ? <div className="c-loading">Loading older messages...</div> : <div className="c-start">Beginning of the archived history</div>}
					<Feed chat={chat} msgs={msgs} at={p.at} />
				</div>
			</div>
			{!atBottom && (
				<button type="button" className="c-tobottom" aria-label="Jump to latest"
					onClick={() => { setFrom(Math.max(0, all.length - PAGE)); set({ at: undefined }, { replace: true }); const el = scroller.current; requestAnimationFrame(() => { el.scrollTop = el.scrollHeight; }); }}>
					<ChevronDown size={22} />
				</button>
			)}
			{publicView
				? <footer className="c-readonly"><Globe size={14} />Public archive of {title}. Read only.</footer>
				: <footer className="c-readonly">Archive view, read only. {live ? 'New messages appear as they are logged.' : 'Live updates paused.'}</footer>}
		</div>
	);
	if (publicView) {
		return (
			<div className="app c-publicpage">
				<header className="c-pubtop">
					<span className="a-logo">tg</span><b>tgloggerd</b><span className="c-sub">public archive</span>
					<span className="a-grow" />
					<ThemeButton />
					<Link to="/login" className="a-btn a-btn-ghost"><LogIn size={14} />Sign in</Link>
				</header>
				{body}
			</div>
		);
	}
	return (
		<Shell flush crumbs={[[kind === 'group' ? 'Groups' : 'Users', kind === 'group' ? '/groups' : '/users'], [title, back], ['Chat', null]]}>
			{body}
		</Shell>
	);
}
