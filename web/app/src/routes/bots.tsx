// Platform forwarding > Bots (decisions/2026-11-18-ownership-and-bots.md):
// the viewer's Telegram and Discord bots, or every bot with its owner
// for admins and superadmins. A token is write-only: typed once, never
// shown again, checked here in shape only (whether it works shows in
// the status discordd reports).
import { useState, type FormEvent } from 'react';
import { Bot as BotIcon, Pencil, Plus, Trash2, TriangleAlert, Users as UsersIcon } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Link, useNav } from '../nav.tsx';
import { Avatar, Btn, PageHead, Pill } from '../ui/primitives.tsx';
import { Field, NO_FILL, SecretInput, Seg, Switch } from '../ui/forms.tsx';
import { Modal } from '../ui/Modal.tsx';
import { ApiError, api, ok } from '../api/client.ts';
import { ago } from '../format.ts';
import type { PageData } from '../page.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

export type BotsProps = S['BotsProps'];
type Bot = S['Bot'];
type Kind = S['BotKind'];
type Status = S['BotStatus'];

const KIND_LABEL: Record<Kind, string> = { telegram: 'Telegram', discord: 'Discord' };

// Each status as a pill, with what to do about it.
export const BOT_STATUS: Record<Status, [string, '' | 'ok' | 'danger' | 'info' | 'muted', string | null]> = {
	connected: ['connected', 'ok', null],
	connecting: ['connecting', 'info', 'Logging in; the status follows in a moment.'],
	intent_missing: ['intent missing', 'danger', 'Turn on Message Content Intent in the Discord developer portal (Bot page), then enable the bot again.'],
	token_refused: ['token refused', 'danger', 'The platform refused the token. Paste a new one.'],
	disabled: ['disabled', 'muted', null],
};

// The shapes the server checks (webapi/openapi.yaml, BotCreate.token).
export function tokenShapeOk(kind: Kind, token: string): boolean {
	if (kind === 'telegram') return /^\d+:[A-Za-z0-9_-]{30,}$/.test(token);
	const parts = token.split('.');
	if (parts.length !== 3 || !parts.every((x) => /^[A-Za-z0-9_-]+$/.test(x))) return false;
	try { return /^\d{15,25}$/.test(atob(parts[0]!.replace(/-/g, '+').replace(/_/g, '/'))); } catch { return false; }
}

const TOKEN_HINT: Record<Kind, string> = {
	telegram: 'From @BotFather: <digits>:<secret>. Saved write-only; it is never shown again.',
	discord: 'From the Discord developer portal, the Bot page, Reset Token. Saved write-only; it is never shown again.',
};

// A bot's avatar and name, each a link to its user page when it has one.
// The avatar's link is only a bigger target for the mouse: out of the tab
// order and hidden from screen readers, so the bot is one link, named by
// its name.
type BotFace = { id: number; name: string; photo?: string | null; profile?: number | null };
export function BotAvatar({ b, size }: { b: BotFace; size: number }) {
	const a = <Avatar id={b.id} name={b.name} photo={b.photo} size={size} />;
	return b.profile != null ? <Link to={`/users/${b.profile}`} className="a-botavatar" tabIndex={-1} aria-hidden="true">{a}</Link> : a;
}
export function BotName({ b, className }: { b: BotFace; className: string }) {
	return b.profile != null ? <Link to={`/users/${b.profile}`} className={`${className} a-botlink`}>{b.name}</Link> : <span className={className}>{b.name}</span>;
}

const errText = (x: unknown) => (x instanceof ApiError ? x.message : 'The server could not be reached.');

function TokenField({ kind, value, onChange, label = 'Token' }: { kind: Kind; value: string; onChange: (v: string) => void; label?: string }) {
	return (
		<Field label={label} hint={TOKEN_HINT[kind]}>
			<SecretInput id="bot-token" value={value} onChange={(v) => onChange(v.trim())} what="token" />
		</Field>
	);
}

function AddBot({ onClose, onSaved }: { onClose: () => void; onSaved: () => void }) {
	const [kind, setKind] = useState<Kind>('discord');
	const [name, setName] = useState('');
	const [token, setToken] = useState('');
	const [err, setErr] = useState('');
	const submit = async (e: FormEvent) => {
		e.preventDefault();
		setErr('');
		if (!name.trim()) { setErr('Give the bot a name.'); return; }
		if (!tokenShapeOk(kind, token)) { setErr(`That does not look like a ${KIND_LABEL[kind]} bot token.`); return; }
		try {
			await ok(api.POST('/api/bots', { body: { kind, name: name.trim(), token } }));
			setToken('');
			onSaved(); onClose();
		} catch (x) { setErr(errText(x)); }
	};
	return (
		<Modal title="Add a bot" icon={<BotIcon size={16} />} onClose={onClose}
			footer={<><span className="a-grow" /><Btn ghost onClick={onClose}>Cancel</Btn><button className="a-btn a-btn-primary" type="submit" form="add-bot">Add bot</button></>}>
			<form id="add-bot" className="a-form" onSubmit={(e) => void submit(e)}>
				{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
				<Seg value={kind} onChange={(k) => { setKind(k); setErr(''); }} items={[['discord', 'Discord'], ['telegram', 'Telegram']]} />
				<p className="a-muted sm">{kind === 'discord' ? 'A Discord bot reads the Discord channels of your Discord to Telegram routes.' : 'A Telegram bot sends into the Telegram chats of your Discord to Telegram routes; add it to those chats.'}</p>
				<Field label="Name" hint="Your own label for it."><input id="bot-label" name="bot-label" value={name} onChange={(e) => setName(e.target.value)} maxLength={64} autoFocus {...NO_FILL} /></Field>
				<TokenField kind={kind} value={token} onChange={setToken} />
			</form>
		</Modal>
	);
}

function EditBot({ bot, onClose, onSaved }: { bot: Bot; onClose: () => void; onSaved: () => void }) {
	const [name, setName] = useState(bot.name);
	const [token, setToken] = useState('');
	const [err, setErr] = useState('');
	const submit = async (e: FormEvent) => {
		e.preventDefault();
		setErr('');
		if (!name.trim()) { setErr('Give the bot a name.'); return; }
		if (token && !tokenShapeOk(bot.kind, token)) { setErr(`That does not look like a ${KIND_LABEL[bot.kind]} bot token.`); return; }
		try {
			await ok(api.PATCH('/api/bots/{id}', { params: { path: { id: bot.id } }, body: { ...(name.trim() !== bot.name ? { name: name.trim() } : {}), ...(token ? { token } : {}) } }));
			setToken('');
			onSaved(); onClose();
		} catch (x) { setErr(errText(x)); }
	};
	return (
		<Modal title={bot.name} icon={<BotIcon size={16} />} onClose={onClose}
			footer={<><span className="a-grow" /><Btn ghost onClick={onClose}>Cancel</Btn><button className="a-btn a-btn-primary" type="submit" form="edit-bot">Save</button></>}>
			<form id="edit-bot" className="a-form" onSubmit={(e) => void submit(e)}>
				{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
				<Field label="Name"><input id="bot-label" name="bot-label" value={name} onChange={(e) => setName(e.target.value)} maxLength={64} {...NO_FILL} /></Field>
				<TokenField kind={bot.kind} value={token} onChange={setToken} label="New token" />
				<p className="a-muted sm">Leave the token empty to keep the bot's current one.</p>
			</form>
		</Modal>
	);
}

function Transfer({ bot, onClose, onSaved }: { bot: Bot; onClose: () => void; onSaved: () => void }) {
	const [to, setTo] = useState('');
	const [err, setErr] = useState('');
	const go = async () => {
		try { await ok(api.POST('/api/bots/{id}/transfer', { params: { path: { id: bot.id } }, body: { owner_id: Number(to) } })); onSaved(); onClose(); } catch (x) { setErr(errText(x)); }
	};
	return (
		<Modal title={`Give ${bot.name} to`} icon={<UsersIcon size={16} />} onClose={onClose}
			footer={<><span className="a-grow" /><Btn ghost onClick={onClose}>Cancel</Btn><Btn primary disabled={!to} onClick={() => void go()}>Transfer</Btn></>}>
			{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
			<Field label="New owner">
				<select className="a-select" value={to} onChange={(e) => setTo(e.target.value)}>
					<option value="">Choose an account...</option>
					{bot.transfer_candidates.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
				</select>
			</Field>
		</Modal>
	);
}

function Bots({ page }: { page: PageData<BotsProps> }) {
	const { refresh } = useNav();
	const v = page.viewer;
	const p = page.props;
	const [kind, setKind] = useState<'all' | Kind>('all');
	const [adding, setAdding] = useState(false);
	const [edit, setEdit] = useState<Bot | null>(null);
	const [giving, setGiving] = useState<Bot | null>(null);
	const [err, setErr] = useState('');
	const act = async (fn: () => Promise<unknown>) => { setErr(''); try { await fn(); await refresh(); } catch (x) { setErr(errText(x)); } };
	const rows: Bot[] = p.bots.filter((b) => kind === 'all' || b.kind === kind);
	return (
		<Shell crumbs={[['Platform forwarding', '/platform-fwd'], ['Bots', null]]}>
			<PageHead title="Bots" sub={p.show_owner ? 'Every account\'s Telegram and Discord bots.' : 'Your Telegram and Discord bots, for your Discord to Telegram routes.'}
				actions={<Btn primary icon={Plus} onClick={() => setAdding(true)}>Add bot</Btn>} />
			{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
			<div className="a-toolbar"><Seg value={kind} onChange={setKind} items={[['all', 'All'], ['discord', 'Discord'], ['telegram', 'Telegram']]} /></div>
			<div className="a-card a-tablecard">
				{rows.length ? (
					<table className="a-table">
						<thead><tr><th>Bot</th><th>Kind</th><th>Status</th>{p.show_owner && <th>Owner</th>}<th className="r">Routes</th><th>On</th><th /></tr></thead>
						<tbody>{rows.map((b) => {
							const [label, tone, advice] = BOT_STATUS[b.status];
							return (
								<tr key={b.id} className={b.enabled ? '' : 'a-off'}>
									<td><span className="a-usercell"><BotAvatar b={b} size={28} /><span><BotName b={b} className="a-uname" /><span className="a-handle">{b.identity ? (b.identity.username ? `@${b.identity.username}` : `id ${b.identity.user_id}`) : 'not logged in yet'}</span></span></span></td>
									<td>{KIND_LABEL[b.kind]}</td>
									<td className="a-botstatus"><Pill tone={tone}>{label}</Pill>{advice && <span className="a-muted sm a-block">{advice}</span>}{b.status_detail && <span className="a-muted sm a-block" title="As discordd reported it">{b.status_detail}</span>}{b.status_at && <span className="a-muted sm a-block">{ago(b.status_at, v)}</span>}</td>
									{p.show_owner && <td><span className={b.owner.active ? '' : 'a-muted'}>{b.owner.name}</span></td>}
									<td className="r num">{b.routes}</td>
									<td><Switch checked={b.enabled} onChange={(on) => void act(() => ok(api.PATCH('/api/bots/{id}', { params: { path: { id: b.id } }, body: { enabled: on } })))} label={<span className="a-sr">{b.enabled ? `Disable ${b.name}` : `Enable ${b.name}`}</span>} /></td>
									<td className="a-rowactions">
										<button type="button" className="a-icon-btn" aria-label={`Edit ${b.name}`} onClick={() => setEdit(b)}><Pencil size={15} /></button>
										{b.transfer_candidates.length > 0 && <button type="button" className="a-icon-btn" aria-label={`Give ${b.name} to another account`} onClick={() => setGiving(b)}><UsersIcon size={15} /></button>}
										<button type="button" className="a-icon-btn a-danger-hover" aria-label={`Remove ${b.name}`} onClick={() => { if (confirm(`Remove ${b.name}? Its token is deleted.`)) void act(() => ok(api.DELETE('/api/bots/{id}', { params: { path: { id: b.id } } }))); }}><Trash2 size={15} /></button>
									</td>
								</tr>
							);
						})}</tbody>
					</table>
				) : <p className="a-muted a-empty">{p.bots.length ? 'No bot of this kind.' : 'No bots yet. Add one to use it in a Discord to Telegram route.'}</p>}
			</div>
			{adding && <AddBot onClose={() => setAdding(false)} onSaved={() => void refresh()} />}
			{edit && <EditBot bot={edit} onClose={() => setEdit(null)} onSaved={() => void refresh()} />}
			{giving && <Transfer bot={giving} onClose={() => setGiving(null)} onSaved={() => void refresh()} />}
		</Shell>
	);
}

export const botsRoute: RouteDef<BotsProps> = { Page: Bots, head: () => ({ title: 'Bots - tgloggerd', robots: 'noindex' }) };
