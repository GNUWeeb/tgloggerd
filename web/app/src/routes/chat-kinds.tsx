// What a message's kind adds to its bubble (web/docs/message-kinds.md):
// a place, a contact, a poll, a checklist, a call and the others, from
// the message's detail; and the reactions, views, forwards, replies and
// pin it was last seen with. The chat and the message page share them.
// Whom a detail names comes as ChatRefs the viewer may see; the ones it
// may not are null or left out, and the *_total fields still count
// them, so the page says how many more there were but never who.
import type { ReactNode } from 'react';
import { Building2, Check, CircleDashed, CircleHelp, Contact, Dices, Eye, Gamepad2, Gift, LayoutTemplate, ListChecks, Lock, MapPin, MessageSquare, Navigation, Phone, PhoneMissed, PhoneOff, Pin, Receipt, Repeat2, Square, SquareCheck, Star, TimerOff, Trophy, Video, AppWindow, type LucideIcon } from 'lucide-react';
import { Link } from '../nav.tsx';
import { Avatar } from '../ui/primitives.tsx';
import { compact, dateTime, fmt } from '../format.ts';
import { duration } from '../ui/media.tsx';
import type { Viewer } from '../page.ts';
import type { S } from '../api/schema.ts';

type Detail = NonNullable<S['MessageDetail']>;
type Ref = S['ChatRef'];
type Kinds = S['MessageKinds'];

const refHref = (r: Ref) => (r.kind === 'user' ? `/users/${r.id}` : `/groups/${r.id}`);
function RefLink({ r }: { r: Ref }) {
	return <Link to={refHref(r)} className="k-ref"><Avatar id={r.id} name={r.title} photo={r.photo} size={18} square={r.kind === 'group'} />{r.title}</Link>;
}
// A list of people or chats: the ones shown, then how many more.
function Refs({ list, total, noun }: { list: Ref[]; total: number; noun: [string, string] }) {
	const more = total - list.length;
	return (
		<span className="k-refs">
			{list.map((r) => <RefLink key={`${r.kind}${r.id}`} r={r} />)}
			{more > 0 && <span className="c-sub">{list.length ? `and ${fmt(more)} more` : `${fmt(more)} ${more === 1 ? noun[0] : noun[1]}`}</span>}
		</span>
	);
}

// The header line of a detail card: its icon and what it is.
function Head({ icon: I, children }: { icon: LucideIcon; children: ReactNode }) {
	return <div className="k-head"><I size={15} /><span>{children}</span></div>;
}

const osm = (lat: number, lon: number) => `https://www.openstreetmap.org/?mlat=${lat}&mlon=${lon}#map=16/${lat}/${lon}`;
const coords = (lat: number, lon: number) => `${lat.toFixed(5)}, ${lon.toFixed(5)}`;
const FOREVER = 0x7fffffff;
function period(s: number): string {
	if (s >= FOREVER) return 'until stopped';
	if (s >= 3600) return `for ${Math.round(s / 3600)} hour${s >= 7200 ? 's' : ''}`;
	return `for ${Math.round(s / 60)} minutes`;
}

// An amount in its currency's smallest unit (as Telegram sends it), as
// the currency writes it. The unit follows ISO 4217's exponents, which
// Telegram uses, not the browser's own idea of a currency's decimals
// (some write IDR without any). XTR is Telegram Stars.
const ZERO_DECIMALS = new Set(['BIF', 'CLP', 'DJF', 'GNF', 'ISK', 'JPY', 'KMF', 'KRW', 'PYG', 'RWF', 'UGX', 'UYI', 'VND', 'VUV', 'XAF', 'XOF', 'XPF']);
const THREE_DECIMALS = new Set(['BHD', 'IQD', 'JOD', 'KWD', 'LYD', 'OMR', 'TND']);
export function money(amount: number, currency: string): string {
	if (currency === 'XTR') return `${fmt(amount)} Star${amount === 1 ? '' : 's'}`;
	const exp = ZERO_DECIMALS.has(currency) ? 0 : THREE_DECIMALS.has(currency) ? 3 : 2;
	try {
		return new Intl.NumberFormat('en-US', { style: 'currency', currency, minimumFractionDigits: exp, maximumFractionDigits: exp }).format(amount / 10 ** exp);
	} catch { return `${fmt(amount)} ${currency}`; }
}

// What a call was, as one line: "Missed video call", "Voice chat . live now".
function callTitle(d: S['CallDetail']): string {
	const kind = d.group ? (d.video ? 'Video chat' : 'Voice chat') : d.video ? 'Video call' : 'Voice call';
	switch (d.outcome) {
		case 'active': return `${kind} . live now`;
		case 'missed': return `Missed ${kind.toLowerCase()}`;
		case 'declined': return `Declined ${kind.toLowerCase()}`;
		case 'disconnected': return `${kind}, disconnected`;
		case 'upgraded': return `${kind}, moved to a group call`;
		default: return d.group ? `${kind} ended` : kind;
	}
}
const CALL_ICON: Record<S['CallDetail']['outcome'], LucideIcon> = { missed: PhoneMissed, declined: PhoneOff, disconnected: PhoneOff, hung_up: Phone, upgraded: Phone, ended: Phone, active: Phone };

const PLACEHOLDER: Record<'rich' | 'widget' | 'expired' | 'unsupported', [string, LucideIcon]> = {
	rich: ['A rich message, which this archive does not show', LayoutTemplate],
	widget: ['A widget, which this archive does not show', AppWindow],
	expired: ['Self-destructing media that has expired', TimerOff],
	unsupported: ['A message Telegram itself could not read', CircleHelp],
};

// The kinds whose message text belongs inside the card (a checklist's
// title, a game's text): the bubble leaves the text to the card.
export const TEXT_INSIDE: ReadonlySet<string> = new Set(['checklist']);

export function KindDetail({ d, text, v }: { d: Detail; text: string; v: Viewer }) {
	switch (d.kind) {
		case 'location':
			return (
				<div className="k-card k-place">
					<Head icon={d.live ? Navigation : MapPin}>{d.live ? <>Live location <span className="c-sub">. shared {period(d.live.period)}</span></> : 'Location'}</Head>
					<a href={osm(d.lat, d.lon)} target="_blank" rel="noopener noreferrer" className="k-coords mono">{coords(d.lat, d.lon)}</a>
					{(d.accuracy !== null || d.live) && (
						<div className="c-sub">
							{[d.accuracy !== null ? `within ${fmt(d.accuracy)} m` : '', d.live?.heading ? `heading ${d.live.heading} deg` : '', d.live?.proximity_radius ? `alert within ${fmt(d.live.proximity_radius)} m` : '',
								d.live ? `latest position; ${fmt(d.live.updates)} earlier one${d.live.updates === 1 ? '' : 's'} kept` : ''].filter(Boolean).join(' . ')}
						</div>
					)}
				</div>
			);
		case 'venue':
			return (
				<div className="k-card k-place">
					<Head icon={Building2}>{d.title || 'Venue'}</Head>
					{d.address && <div>{d.address}</div>}
					<a href={osm(d.lat, d.lon)} target="_blank" rel="noopener noreferrer" className="k-coords mono">{coords(d.lat, d.lon)}</a>
				</div>
			);
		case 'contact': {
			const name = [d.first_name, d.last_name].filter(Boolean).join(' ') || 'Contact';
			return (
				<div className="k-card k-contact">
					<Avatar id={d.user?.id ?? name.length} name={name} size={40} />
					<div className="a-minw">
						<Head icon={Contact}>{d.user ? <Link to={refHref(d.user)} className="a-rowlink">{name}</Link> : name}</Head>
						{d.phone && <div className="mono">{d.phone}</div>}
					</div>
				</div>
			);
		}
		case 'dice':
			return (
				<div className="k-dice" aria-label={d.stake ? `Stake dice, ${d.value}` : `${d.emoji} ${d.value}`}>
					<span className="k-dice-face" aria-hidden="true">{d.stake ? <Dices size={36} /> : d.emoji}</span>
					<span className="k-dice-value">{d.value || 'rolling'}</span>
					{d.stake && <span className="c-sub">stake dice</span>}
				</div>
			);
		case 'game':
			return (
				<div className="k-card">
					{d.photo?.thumb && <img className="k-cover" src={d.photo.thumb} alt="" loading="lazy" />}
					<Head icon={Gamepad2}>{d.title} <span className="c-sub">. game</span></Head>
					{d.description && <div className="c-sub">{d.description}</div>}
				</div>
			);
		case 'poll': {
			const label = d.type === 'quiz' ? 'Quiz' : d.anonymous ? 'Anonymous poll' : 'Poll';
			const more = d.options_total - d.options.length;
			return (
				<div className="k-card k-poll">
					<div className="k-question">{d.question}</div>
					<div className="c-sub">{label}{d.multiple ? ' . several answers' : ''}{d.closed ? ' . closed' : d.close_date ? ` . closes ${dateTime(d.close_date, v)}` : ''}</div>
					<ul className="k-options">{d.options.map((o, i) => (
						<li key={i} className={o.correct ? 'correct' : ''}>
							<span className="k-opt"><span>{o.text}{o.correct && <Check size={13} className="k-right" aria-label="correct answer" />}</span><span className="num">{o.percent}%</span></span>
							<span className="k-bar"><span style={{ width: `${o.percent}%` }} /></span>
						</li>
					))}</ul>
					<div className="c-sub">{fmt(d.total_voters)} {d.total_voters === 1 ? 'vote' : 'votes'}{more > 0 ? ` . ${fmt(more)} more option${more === 1 ? '' : 's'} not shown` : ''}</div>
					{d.explanation && <div className="k-explain">{d.explanation}</div>}
				</div>
			);
		}
		case 'checklist': {
			const more = d.tasks_total - d.tasks.length;
			return (
				<div className="k-card k-checklist">
					<Head icon={ListChecks}>{text || 'Checklist'}</Head>
					<ul>{d.tasks.map((t) => (
						<li key={t.id} className={t.done ? 'done' : ''}>
							{t.done ? <SquareCheck size={15} aria-label="done" /> : <Square size={15} aria-label="not done" />}
							<span className="a-grow"><span className="k-task">{t.text}</span>{t.done && (t.done_by || t.done_at) && <span className="c-sub"> . {t.done_by ? <>by <RefLink r={t.done_by} /></> : 'done'}{t.done_at ? ` ${dateTime(t.done_at, v)}` : ''}</span>}</span>
						</li>
					))}</ul>
					{more > 0 && <div className="c-sub">and {fmt(more)} more task{more === 1 ? '' : 's'}</div>}
				</div>
			);
		}
		case 'story':
			return (
				<div className="k-card">
					<Head icon={CircleDashed}>{d.poster ? <>Story by <RefLink r={d.poster} /></> : 'A story'}</Head>
					<div className="c-sub">Stories are not archived.</div>
				</div>
			);
		case 'invoice':
			return (
				<div className="k-card">
					{d.photo?.thumb && <img className="k-cover" src={d.photo.thumb} alt="" loading="lazy" />}
					<Head icon={Receipt}>{d.title}</Head>
					{d.description && <div className="c-sub">{d.description}</div>}
					<div className="k-amount">{money(d.total_amount, d.currency)}{d.is_test && <span className="c-sub"> . test invoice</span>}</div>
				</div>
			);
		case 'giveaway': {
			const prize = !d.prize ? 'A giveaway' : 'premium_months' in d.prize ? `Telegram Premium for ${d.prize.premium_months} month${d.prize.premium_months === 1 ? '' : 's'}` : `${fmt(d.prize.stars)} Stars`;
			return (
				<div className="k-card">
					<Head icon={Gift}>{prize}</Head>
					<div>{fmt(d.winner_count)} winner{d.winner_count === 1 ? '' : 's'}{d.winners_selection_date ? `, chosen ${dateTime(d.winners_selection_date, v)}` : ''}{d.only_new_members ? ', new members only' : ''}</div>
					{d.prize_description && <div className="c-sub">{d.prize_description}</div>}
					{d.chats_total > 0 && <div className="k-line"><Trophy size={13} /><Refs list={d.chats} total={d.chats_total} noun={['chat', 'chats']} /></div>}
				</div>
			);
		}
		case 'call': {
			const I = d.video ? Video : CALL_ICON[d.outcome];
			return (
				<div className={`k-call ${d.outcome}`}>
					<span className="k-callicon"><I size={17} /></span>
					<span><b>{callTitle(d)}</b>{d.duration > 0 && <span className="c-sub"> . {duration(d.duration)}</span>}</span>
					{d.group && d.participants_total > 0 && <div className="k-line"><Refs list={d.participants} total={d.participants_total} noun={['participant', 'participants']} /></div>}
				</div>
			);
		}
		case 'paid_media':
			return (
				<div className="k-card">
					<Head icon={Star}>{fmt(d.star_count)} Stars <span className="c-sub">. paid media</span></Head>
					<div className="k-paid">{d.items.map((it, i) => (
						<span key={i} className="k-paiditem" title={it.kind === 'preview' ? 'Not paid for: only a blurred preview is known' : it.kind}>
							{it.media?.thumb ? <img src={it.media.thumb} alt="" loading="lazy" /> : it.kind === 'preview' ? <Lock size={16} /> : it.kind === 'video' ? <Video size={16} /> : <Star size={16} />}
						</span>
					))}</div>
				</div>
			);
		default: {
			const [label, I] = PLACEHOLDER[d.kind];
			return <div className="c-unknown c-sub k-placeholder"><I size={14} />{label}</div>;
		}
	}
}

// The reactions a message was last seen with.
export function Reactions({ list }: { list: Kinds['reactions'] }) {
	if (!list.length) return null;
	return (
		<div className="k-reactions" aria-label="Reactions">
			{list.map((r, i) => (
				<span key={i} className="k-reaction" title={r.paid ? 'Paid reaction (Stars)' : r.custom_emoji_id ? 'Custom emoji' : r.emoji}>
					<span aria-hidden="true">{r.paid ? <Star size={12} fill="currentColor" /> : r.emoji ?? '◆'}</span><span className="num">{compact(r.count)}</span>
				</span>
			))}
		</div>
	);
}

// Views, forwards, replies and the pin, for the bubble's meta line.
export function Counts({ k }: { k: Pick<Kinds, 'pinned' | 'views' | 'forwards' | 'replies'> }) {
	return (
		<>
			{k.pinned && <span className="k-count" title="Pinned"><Pin size={11} /></span>}
			{k.replies !== null && k.replies > 0 && <span className="k-count" title={`${fmt(k.replies)} replies`}><MessageSquare size={11} />{compact(k.replies)}</span>}
			{k.forwards !== null && k.forwards > 0 && <span className="k-count" title={`${fmt(k.forwards)} forwards`}><Repeat2 size={11} />{compact(k.forwards)}</span>}
			{k.views !== null && <span className="k-count" title={`${fmt(k.views)} views`}><Eye size={11} />{compact(k.views)}</span>}
		</>
	);
}

type Topic = S['ChatTopic'];
export const topicColor = (t: Topic) => `#${(t.icon_color & 0xffffff).toString(16).padStart(6, '0')}`;

// A forum's topics above its chat: all of them, or one at a time. The
// choice is the page's ?topic=, so it survives a reload and a link.
export function TopicStrip({ topics, current, hrefOf }: { topics: Topic[]; current: number | null; hrefOf: (id: number | null) => string }) {
	return (
		<nav className="k-topics" aria-label="Forum topics">
			<Link to={hrefOf(null)} className={current === null ? 'on' : ''} aria-current={current === null ? 'page' : undefined}>All topics</Link>
			{topics.filter((t) => !t.hidden || t.id === current).map((t) => (
				<Link key={t.id} to={hrefOf(t.id)} className={current === t.id ? 'on' : ''} aria-current={current === t.id ? 'page' : undefined} title={t.closed ? `${t.name} (closed)` : t.name}>
					<span className="k-topicdot" style={{ background: topicColor(t) }} />{t.name}{t.closed && <Lock size={11} aria-label="closed" />}
				</Link>
			))}
		</nav>
	);
}

// The topic of a message, on the bubble, while the chat shows them all.
export function TopicTag({ t, href }: { t: Topic; href: string }) {
	return <Link to={href} className="k-topictag" style={{ ['--tc' as string]: topicColor(t) }}><span className="k-topicdot" style={{ background: topicColor(t) }} />{t.name}</Link>;
}
