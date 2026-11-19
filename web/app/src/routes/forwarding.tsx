// Integrations > Platform forwarding: both directions, with each route's
// owner and federation (web-accounts.md section 6). Members see only
// their own routes; a "People with access" dialog transfers ownership.
import { useEffect, useState } from 'react';
import { track } from '../ui/busy.ts';
import { ArrowLeftRight, ArrowRight, BotMessageSquare, Crown, Network, Pencil, Plus, Send, Trash2, TriangleAlert, Users } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Link, useNav } from '../nav.tsx';
import { Avatar, Btn, PageHead, Pill } from '../ui/primitives.tsx';
import { ChatPicker, CheckChips, Field, SecretInput, Seg, Switch, type PickItem } from '../ui/forms.tsx';
import { Modal } from '../ui/Modal.tsx';
import { ApiError, api, ok } from '../api/client.ts';
import { ago, fmt } from '../format.ts';
import type { PageData } from '../page.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';
import { BOT_STATUS, BotAvatar, BotName } from './bots.tsx';

type Person = S['FwdPerson'];
type Owned = S['Owned'];
export type T2DRoute = S['T2DRoute'];
export type D2TRoute = S['D2TRoute'];
type BotRef = S['BotRef'];
type BotKind = S['BotKind'];

export type ForwardingIndexProps = S['ForwardingIndexProps'];
export type T2DProps = S['T2DProps'];
export type D2TProps = S['D2TProps'];

type T2DKind = S['T2DRoute']['types'][number];
type D2TKind = S['D2TRoute']['types'][number];
const KINDS: [T2DKind, string][] = [['sticker', 'Sticker'], ['text', 'Text message'], ['photo', 'Photo'], ['video', 'Video'], ['document', 'Document'], ['other', 'Others']];
const D2T_KINDS = KINDS.filter((k): k is [D2TKind, string] => k[0] !== 'other');
const LABEL = Object.fromEntries(KINDS);

function errText(x: unknown) { return x instanceof ApiError ? x.message : 'The server could not be reached.'; }

function Types({ types, all }: { types: string[]; all: [string, string][] }) {
	if (types.length === all.length) return <span className="a-muted">All content</span>;
	return <>{types.map((t) => <Pill key={t}>{LABEL[t]}</Pill>)}</>;
}

function ChatCell({ c }: { c: PickItem }) {
	return (
		<Link to={c.kind === 'group' ? `/groups/${c.id}` : `/users/${c.id}`} className="a-usercell">
			<Avatar id={c.id} name={c.title} photo={c.photo} size={28} square={c.kind === 'group'} />
			<span><span className="a-uname">{c.title}</span><span className="a-handle">{c.sub ?? (c.kind === 'group' ? 'group' : 'private chat')}</span></span>
		</Link>
	);
}

function OwnerCell({ r }: { r: Owned }) {
	return (
		<span className="a-ownercell">
			<Avatar id={r.owner.id} name={r.owner.name} size={22} />
			<span className={r.owner.active ? '' : 'a-muted'}>{r.owner.name}</span>
			{!r.owner.active && <Pill tone="danger"><TriangleAlert size={11} />owner disabled</Pill>}
		</span>
	);
}

// The owner today; editors and viewers can join this list later.
type Dir = 'telegram-discord' | 'discord-telegram';

// Handing a route to another owner is designed but not served yet
// (the transfer operations are planned in webapi/openapi.yaml): until it
// is, the dialog shows the owner only. Turn this on with the service.
const ROUTE_TRANSFER_SERVED = false;

function Access({ r, dir, title, canTransfer, onClose, onSaved }: { r: Owned; dir: Dir; title: string; canTransfer: boolean; onClose: () => void; onSaved: () => void }) {
	const [to, setTo] = useState('');
	const [err, setErr] = useState('');
	const transfer = async () => {
		try {
			const req = { params: { path: { id: r.id } }, body: { owner_id: Number(to) } };
			await ok(dir === 'telegram-discord' ? api.POST('/api/forwarding/telegram-discord/{id}/transfer', req) : api.POST('/api/forwarding/discord-telegram/{id}/transfer', req));
			onSaved(); onClose();
		} catch (x) { setErr(errText(x)); }
	};
	return (
		<Modal title={`People with access: ${title}`} icon={<Users size={16} />} onClose={onClose}
			footer={<><span className="a-grow" /><Btn ghost onClick={onClose}>Close</Btn>{canTransfer && <Btn primary disabled={!to} onClick={() => void transfer()}>Transfer ownership</Btn>}</>}>
			{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
			<p className="a-muted sm">In the <b>{r.federation.name}</b> federation.</p>
			<ul className="a-people"><li>
				<Avatar id={r.owner.id} name={r.owner.name} size={32} />
				<span className="a-grow"><b>{r.owner.name}</b><span className="a-muted sm">{r.owner.active ? 'owner' : 'disabled; the route keeps running'}</span></span>
				<Pill tone="accent"><Crown size={11} />Owner</Pill>
			</li></ul>
			{canTransfer && (
				<Field label="Transfer ownership to" hint="The route keeps running throughout.">
					<select className="a-select" value={to} onChange={(e) => setTo(e.target.value)}>
						<option value="">Choose an account...</option>
						{r.transfer_candidates.filter((p) => p.id !== r.owner.id).map((p) => <option key={p.id} value={p.id}>{p.name}{p.role ? ` (${p.role})` : ''}</option>)}
					</select>
				</Field>
			)}
		</Modal>
	);
}

function OwnerFilter({ value, onChange, routes, meId }: { value: string; onChange: (v: string) => void; routes: Owned[]; meId: number }) {
	const owners = [...new Map(routes.map((r) => [r.owner.id, r.owner])).values()].filter((o) => o.id !== meId);
	return <div className="a-toolbar"><Seg value={value} onChange={onChange} items={[['all', 'All'], ['mine', 'Mine'], ...owners.map((o) => [String(o.id), o.name] as [string, string])]} /></div>;
}

function byOwner<T extends Owned>(rows: T[], f: string, meId: number): T[] {
	if (f === 'all') return rows;
	if (f === 'mine') return rows.filter((r) => r.owner.id === meId);
	return rows.filter((r) => String(r.owner.id) === f);
}

function ForwardingIndex({ page }: { page: PageData<ForwardingIndexProps> }) {
	const p = page.props;
	const cards: [string, string, string, ForwardingIndexProps['t2d'], string][] = [
		['Telegram to Discord', '/platform-fwd/telegram-discord', 'Mirror new messages from a Telegram group, channel or private chat into a Discord channel through its webhook.', p.t2d, 'webhook'],
		['Discord to Telegram', '/platform-fwd/discord-telegram', 'Mirror new messages from a Discord channel into a Telegram group, sent by a Telegram bot that is a member there.', p.d2t, 'route'],
	];
	return (
		<Shell crumbs={[['Platform forwarding', null]]}>
			<PageHead title="Platform forwarding" sub="Mirror messages between platforms. Each direction has its own transport and credentials." />
			<div className="a-dircards">{cards.map(([title, href, desc, n, noun]) => (
				<Link key={href} to={href} className="a-card a-dircard">
					<div className="a-dircard-top"><ArrowLeftRight size={18} /><h2>{title}</h2><ArrowRight size={16} className="a-muted" /></div>
					<p className="a-muted">{desc}</p>
					<div className="a-dircard-foot"><span><b className="num">{n.active}</b> of {n.total} {noun}s active</span><span className="a-muted">{fmt(n.sent)} messages forwarded</span></div>
				</Link>
			))}
				<Link to="/platform-fwd/bots" className="a-card a-dircard">
					<div className="a-dircard-top"><BotMessageSquare size={18} /><h2>Bots</h2><ArrowRight size={16} className="a-muted" /></div>
					<p className="a-muted">The Discord bots that read channels and the Telegram bots that send into chats for Discord to Telegram routes, with their status.</p>
				</Link>
			</div>
			<p className="a-muted a-foot">Both directions can run on the same pair of chats. Each side skips messages the other side sent, so nothing loops.</p>
		</Shell>
	);
}

function FederationSelect({ value, onChange, options }: { value: number | null; onChange: (v: number) => void; options: { id: number; name: string }[] }) {
	// Asked only when more than one of the owner's federations holds the group.
	if (options.length < 2) return null;
	return (
		<Field label="Federation" hint="The group is in several of your federations; the route belongs to one.">
			<select className="a-select" value={value ?? ''} onChange={(e) => onChange(Number(e.target.value))}>
				<option value="">Choose a federation...</option>
				{options.map((f) => <option key={f.id} value={f.id}>{f.name}</option>)}
			</select>
		</Field>
	);
}

function TelegramDiscord({ page }: { page: PageData<T2DProps> }) {
	const { navigate } = useNav();
	const v = page.viewer;
	const meId = v.account?.id ?? 0;
	const member = v.account?.role === 'member';
	const [own, setOwn] = useState('all');
	const [access, setAccess] = useState<T2DRoute | null>(null);
	type Draft = { id?: number; chat: PickItem | null; was?: PickItem; url: string; types: T2DRoute['types']; enabled: boolean; federation_id: number | null };
	const [edit, setEdit] = useState<Draft | null>(null);
	const [err, setErr] = useState('');
	const [note, setNote] = useState('');
	const refresh = () => navigate(page.url, { replace: true, scroll: false });
	const act = async (fn: () => Promise<unknown>, done?: string) => {
		setErr(''); setNote('');
		try { await fn(); if (done) setNote(done); else { setEdit(null); } refresh(); } catch (x) { setErr(errText(x)); }
	};
	const save = (d: Draft) => act(() => (d.id
		? ok(api.PATCH('/api/forwarding/telegram-discord/{id}', { params: { path: { id: d.id } }, body: {
			types: d.types, enabled: d.enabled, ...(d.url ? { webhook_url: d.url } : {}),
			// A new chat goes with its kind; an unchanged one is left out.
			...(d.chat && (d.chat.id !== d.was?.id || d.chat.kind !== d.was?.kind) ? { chat_id: d.chat.id, chat_kind: d.chat.kind } : {}),
		} }))
		: ok(api.POST('/api/forwarding/telegram-discord', { body: { chat_id: d.chat!.id, chat_kind: d.chat!.kind, webhook_url: d.url, types: d.types, enabled: d.enabled, federation_id: d.federation_id ?? undefined } }))));
	const rows = byOwner(page.props.routes, own, meId);
	return (
		<Shell crumbs={[['Platform forwarding', '/platform-fwd'], ['Telegram to Discord', null]]}>
			<PageHead title="Telegram to Discord" sub="Each new message in the Telegram chat is posted to the Discord channel behind the webhook."
				actions={<Btn primary icon={Plus} onClick={() => setEdit({ chat: null, url: '', types: KINDS.map((k) => k[0]), enabled: true, federation_id: null })}>Add webhook</Btn>} />
			{!member && <OwnerFilter value={own} onChange={setOwn} routes={page.props.routes} meId={meId} />}
			{err && !edit && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
			<div className="a-card a-tablecard">
				<div className="a-tablescroll">
					<table className="a-table">
						<thead><tr><th>Telegram chat</th><th>Discord webhook</th><th>Forwards</th><th>Owner</th><th className="a-hide-md">Federation</th><th className="r">Sent</th><th>Status</th><th /></tr></thead>
						<tbody>{rows.map((r) => (
							<tr key={r.id} className={r.enabled ? '' : 'a-off'}>
								<td><ChatCell c={r.chat} /></td>
								<td className="mono a-dim a-clip a-urlcell">{r.url_masked}</td>
								<td className="a-wrapcell"><Types types={r.types} all={KINDS} /></td>
								<td><OwnerCell r={r} /></td>
								<td className="a-hide-md"><Pill><Network size={11} />{r.federation.name}</Pill></td>
								<td className="r num" title={r.last_sent ? `last sent ${ago(r.last_sent, v)}` : 'never sent'}>{fmt(r.sent)}</td>
								<td><span className="a-statuscell"><Switch checked={r.enabled} onChange={(on) => void act(() => ok(api.PATCH('/api/forwarding/telegram-discord/{id}', { params: { path: { id: r.id } }, body: { enabled: on } })))} /><span className={r.enabled ? 'a-ok' : 'a-muted'}>{r.enabled ? 'On' : 'Off'}</span></span></td>
								<td className="a-rowactions">
									<button type="button" className="a-icon-btn" aria-label="People with access" onClick={() => setAccess(r)}><Users size={15} /></button>
									<button type="button" className="a-icon-btn" aria-label="Edit" onClick={() => setEdit({ id: r.id, chat: r.chat, was: r.chat, url: '', types: r.types, enabled: r.enabled, federation_id: r.federation.id })}><Pencil size={15} /></button>
									<button type="button" className="a-icon-btn a-danger-hover" aria-label="Delete" onClick={() => { if (confirm(`Delete the webhook for ${r.chat.title}?`)) void act(() => ok(api.DELETE('/api/forwarding/telegram-discord/{id}', { params: { path: { id: r.id } } }))); }}><Trash2 size={15} /></button>
								</td>
							</tr>
						))}</tbody>
					</table>
				</div>
				{!rows.length && <div className="a-empty">No webhooks{own !== 'all' ? ' for this owner' : ''}.</div>}
			</div>
			{edit && (
				<Modal title={edit.id ? 'Edit webhook' : 'Add webhook'} onClose={() => { setEdit(null); setErr(''); setNote(''); }}
					footer={<>
						<Btn ghost icon={Send} disabled={!edit.id && !edit.url} onClick={() => void act(() => ok(api.POST('/api/forwarding/telegram-discord/test', { body: edit.id && !edit.url ? { id: edit.id } : { webhook_url: edit.url } })), 'Test message sent.')}>Send test message</Btn>
						<span className="a-grow" /><Btn ghost onClick={() => setEdit(null)}>Cancel</Btn><Btn primary disabled={!edit.chat} onClick={() => void save(edit)}>Save</Btn>
					</>}>
					{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
					{note && <div className="a-banner" role="status"><span>{note}</span></div>}
					<Field label="Telegram chat" hint={member ? 'One of the groups granted to you.' : 'A group, channel or user the logger has already seen.'}>
						<ChatPicker value={edit.chat} onChange={(c) => setEdit({ ...edit, chat: c })} search={{ kind: 'any' }} />
					</Field>
					{!edit.id && <FederationSelect value={edit.federation_id} onChange={(f) => setEdit({ ...edit, federation_id: f })} options={page.props.federations} />}
					<Field label="Discord webhook URL" hint={edit.id ? 'Stored and never shown again; enter a new URL to replace it.' : 'Discord: channel settings, Integrations, Webhooks, Copy Webhook URL.'}>
						<SecretInput id="webhook-url" value={edit.url} onChange={(url) => setEdit({ ...edit, url })} what="webhook URL" placeholder={edit.id ? '(unchanged)' : 'https://discord.com/api/webhooks/...'} />
					</Field>
					<Field label="Content to forward" hint="Each part of a message is filtered on its own: a caption follows Text message, the media its own type.">
						<CheckChips items={KINDS} value={edit.types} onChange={(t) => setEdit({ ...edit, types: t })} />
					</Field>
					<Switch checked={edit.enabled} onChange={(on) => setEdit({ ...edit, enabled: on })} label="Enabled" />
				</Modal>
			)}
			{access && <Access r={access} dir="telegram-discord" title={access.chat.title} canTransfer={ROUTE_TRANSFER_SERVED && !member} onClose={() => setAccess(null)} onSaved={refresh} />}
		</Shell>
	);
}

const BOT_KIND: Record<BotKind, string> = { discord: 'Discord', telegram: 'Telegram' };

// A route's bot: its name with its status, as on the Bots page. A route
// forwards only while it and both its bots are enabled.
function BotCell({ b, kind }: { b: BotRef | null; kind: BotKind }) {
	if (!b) return <Pill tone="danger"><TriangleAlert size={11} />no {BOT_KIND[kind]} bot</Pill>;
	const [label, tone] = BOT_STATUS[b.enabled ? b.status : 'disabled'];
	return (
		<span className="a-botcell">
			<BotAvatar b={b} size={24} />
			<span className="a-botcell-text">
				<BotName b={b} className={b.enabled ? 'a-uname' : 'a-uname a-muted'} />
				<span className="a-handle">{b.identity?.username ? `@${b.identity.username} . ` : ''}<Pill tone={tone}>{b.enabled ? label : 'bot disabled'}</Pill></span>
			</span>
		</span>
	);
}

// Why an enabled route forwards nothing, if a bot stops it.
function stoppedBy(r: D2TRoute): string | null {
	if (!r.discord_bot) return 'no Discord bot';
	if (!r.discord_bot.enabled || !r.telegram_bot.enabled) return 'bot disabled';
	return null;
}

// One of the bots the viewer may use (GET /api/bots), of one kind.
function BotSelect({ kind, value, onChange }: { kind: BotKind; value: number | null; onChange: (id: number) => void }) {
	const [bots, setBots] = useState<BotRef[] | null>(null);
	const [err, setErr] = useState('');
	useEffect(() => {
		let live = true;
		track(ok(api.GET('/api/bots', { params: { query: { kind } } }))).then((r) => { if (live) setBots(r.bots); }, (x) => { if (live) setErr(errText(x)); });
		return () => { live = false; };
	}, [kind]);
	if (err) return <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>;
	if (!bots) return <select className="a-select" disabled aria-label={`${BOT_KIND[kind]} bot`}><option>Loading bots...</option></select>;
	if (!bots.length) return <p className="a-muted sm">No {BOT_KIND[kind]} bots yet. <Link to="/platform-fwd/bots" className="a-link">Add one on the Bots page</Link>.</p>;
	return (
		<select className="a-select" aria-label={`${BOT_KIND[kind]} bot`} value={value ?? ''} onChange={(e) => onChange(Number(e.target.value))}>
			<option value="">Choose a bot...</option>
			{bots.map((b) => <option key={b.id} value={b.id}>{b.name}{b.identity?.username ? ` (@${b.identity.username})` : ''} - {b.enabled ? BOT_STATUS[b.status][0] : 'disabled'}</option>)}
		</select>
	);
}

function DiscordTelegram({ page }: { page: PageData<D2TProps> }) {
	const { navigate } = useNav();
	const v = page.viewer;
	const meId = v.account?.id ?? 0;
	const member = v.account?.role === 'member';
	const [own, setOwn] = useState('all');
	const [access, setAccess] = useState<D2TRoute | null>(null);
	type Draft = { id?: number; was?: D2TRoute; channel_id: string; chat: PickItem | null; discord_bot_id: number | null; telegram_bot_id: number | null; types: D2TRoute['types']; enabled: boolean; federation_id: number | null };
	const [edit, setEdit] = useState<Draft | null>(null);
	const [err, setErr] = useState('');
	const refresh = () => navigate(page.url, { replace: true, scroll: false });
	const act = async (fn: () => Promise<unknown>) => {
		setErr('');
		try { await fn(); setEdit(null); refresh(); } catch (x) { setErr(errText(x)); }
	};
	// An edit sends only what changed besides the types and the switch.
	const changes = (d: Draft, r: D2TRoute) => ({
		types: d.types, enabled: d.enabled,
		...(d.channel_id !== r.channel_id ? { discord_channel_id: d.channel_id } : {}),
		...(d.chat && d.chat.id !== r.chat.id ? { chat_id: d.chat.id } : {}),
		...(d.discord_bot_id !== null && d.discord_bot_id !== r.discord_bot?.id ? { discord_bot_id: d.discord_bot_id } : {}),
		...(d.telegram_bot_id !== null && d.telegram_bot_id !== r.telegram_bot.id ? { telegram_bot_id: d.telegram_bot_id } : {}),
	});
	const save = (d: Draft) => act(() => (d.id && d.was
		? ok(api.PATCH('/api/forwarding/discord-telegram/{id}', { params: { path: { id: d.id } }, body: changes(d, d.was) }))
		: ok(api.POST('/api/forwarding/discord-telegram', { body: { discord_channel_id: d.channel_id, chat_id: d.chat!.id, discord_bot_id: d.discord_bot_id!, telegram_bot_id: d.telegram_bot_id!, types: d.types, enabled: d.enabled, federation_id: d.federation_id ?? undefined } }))));
	const editOf = (r: D2TRoute): Draft => ({ id: r.id, was: r, channel_id: r.channel_id, chat: r.chat, discord_bot_id: r.discord_bot?.id ?? null, telegram_bot_id: r.telegram_bot.id, types: r.types, enabled: r.enabled, federation_id: r.federation.id });
	const ready = (d: Draft) => !!d.channel_id && !!d.chat && d.discord_bot_id !== null && d.telegram_bot_id !== null;
	const rows = byOwner(page.props.routes, own, meId);
	return (
		<Shell crumbs={[['Platform forwarding', '/platform-fwd'], ['Discord to Telegram', null]]}>
			<PageHead title="Discord to Telegram" sub="Each new message in the Discord channel is read by a Discord bot and sent to the Telegram chat by a Telegram bot that is a member there."
				actions={<><Btn ghost to="/platform-fwd/bots" icon={BotMessageSquare}>Bots</Btn><Btn primary icon={Plus} onClick={() => setEdit({ channel_id: '', chat: null, discord_bot_id: null, telegram_bot_id: null, types: D2T_KINDS.map((k) => k[0]), enabled: true, federation_id: null })}>Add route</Btn></>} />
			{!member && <OwnerFilter value={own} onChange={setOwn} routes={page.props.routes} meId={meId} />}
			{err && !edit && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
			<div className="a-card a-tablecard">
				<div className="a-tablescroll">
					<table className="a-table">
						<thead><tr><th>Discord channel</th><th /><th>Telegram chat</th><th>Discord bot</th><th>Telegram bot</th><th>Forwards</th><th>Owner</th><th className="a-hide-md">Federation</th><th className="r">Sent</th><th>Status</th><th /></tr></thead>
						<tbody>{rows.map((r) => {
							const stop = r.enabled ? stoppedBy(r) : null;
							return (
								<tr key={r.id} className={r.enabled && !stop ? '' : 'a-off'}>
									<td><span className="a-uname">{r.channel_name}</span><div className="mono a-dim sm">{r.channel_id}</div></td>
									<td className="a-muted"><ArrowRight size={14} /></td>
									<td><ChatCell c={r.chat} /></td>
									<td>{r.discord_bot ? <BotCell b={r.discord_bot} kind="discord" /> : <button type="button" className="a-plain a-linkbtn" onClick={() => setEdit(editOf(r))}><BotCell b={null} kind="discord" /><span className="a-link sm a-block">Pick one</span></button>}</td>
									<td><BotCell b={r.telegram_bot} kind="telegram" /></td>
									<td className="a-wrapcell"><Types types={r.types} all={D2T_KINDS} /></td>
									<td><OwnerCell r={r} /></td>
									<td className="a-hide-md"><Pill><Network size={11} />{r.federation.name}</Pill></td>
									<td className="r num">{fmt(r.sent)}</td>
									<td><span className="a-statuscell"><Switch checked={r.enabled} onChange={(on) => void act(() => ok(api.PATCH('/api/forwarding/discord-telegram/{id}', { params: { path: { id: r.id } }, body: { enabled: on } })))} /><span className={r.enabled && !stop ? 'a-ok' : 'a-muted'}>{!r.enabled ? 'Off' : stop ? `Stopped: ${stop}` : 'On'}</span></span></td>
									<td className="a-rowactions">
										<button type="button" className="a-icon-btn" aria-label="People with access" onClick={() => setAccess(r)}><Users size={15} /></button>
										<button type="button" className="a-icon-btn" aria-label="Edit" onClick={() => setEdit(editOf(r))}><Pencil size={15} /></button>
										<button type="button" className="a-icon-btn a-danger-hover" aria-label="Delete" onClick={() => { if (confirm(`Delete the route from ${r.channel_name}?`)) void act(() => ok(api.DELETE('/api/forwarding/discord-telegram/{id}', { params: { path: { id: r.id } } }))); }}><Trash2 size={15} /></button>
									</td>
								</tr>
							);
						})}</tbody>
					</table>
				</div>
				{!rows.length && <div className="a-empty">No routes{own !== 'all' ? ' for this owner' : ''}.</div>}
			</div>
			{edit && (
				<Modal title={edit.id ? 'Edit route' : 'Add route'} onClose={() => { setEdit(null); setErr(''); }}
					footer={<><span className="a-grow" /><Btn ghost onClick={() => setEdit(null)}>Cancel</Btn><Btn primary disabled={!ready(edit)} onClick={() => void save(edit)}>Save</Btn></>}>
					{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
					<Field label="Discord channel ID" hint="Developer mode on, then right-click the channel and Copy Channel ID.">
						<input value={edit.channel_id} onChange={(e) => setEdit({ ...edit, channel_id: e.target.value.trim() })} inputMode="numeric" />
					</Field>
					<Field label="Discord bot" hint="Reads the channel; it must be in the Discord server, with Message Content Intent on.">
						<BotSelect kind="discord" value={edit.discord_bot_id} onChange={(id) => setEdit({ ...edit, discord_bot_id: id })} />
					</Field>
					<Field label="Telegram chat" hint="A group, channel or private chat the logger has already seen.">
						<ChatPicker value={edit.chat} onChange={(c) => setEdit({ ...edit, chat: c })} search={{ kind: 'any' }} placeholder="Search groups, channels and users" />
					</Field>
					{!edit.id && <FederationSelect value={edit.federation_id} onChange={(f) => setEdit({ ...edit, federation_id: f })} options={page.props.federations} />}
					<Field label="Telegram bot" hint="Sends into the chat; it must already be a member there.">
						<BotSelect kind="telegram" value={edit.telegram_bot_id} onChange={(id) => setEdit({ ...edit, telegram_bot_id: id })} />
					</Field>
					<Field label="Content to forward"><CheckChips items={D2T_KINDS} value={edit.types} onChange={(t) => setEdit({ ...edit, types: t })} /></Field>
					<Switch checked={edit.enabled} onChange={(on) => setEdit({ ...edit, enabled: on })} label="Enabled" />
				</Modal>
			)}
			{access && <Access r={access} dir="discord-telegram" title={access.channel_name} canTransfer={ROUTE_TRANSFER_SERVED && !member} onClose={() => setAccess(null)} onSaved={refresh} />}
		</Shell>
	);
}

export const forwardingIndexRoute: RouteDef<ForwardingIndexProps> = { Page: ForwardingIndex, head: () => ({ title: 'Platform forwarding - tgloggerd', robots: 'noindex' }) };
export const t2dRoute: RouteDef<T2DProps> = { Page: TelegramDiscord, head: () => ({ title: 'Telegram to Discord - tgloggerd', robots: 'noindex' }) };
export const d2tRoute: RouteDef<D2TProps> = { Page: DiscordTelegram, head: () => ({ title: 'Discord to Telegram - tgloggerd', robots: 'noindex' }) };
