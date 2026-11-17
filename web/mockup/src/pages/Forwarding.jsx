import { useState } from 'react';
import { ArrowRight, Plus, Pencil, Trash2, Send, Eye, EyeOff, ArrowLeftRight } from 'lucide-react';
import { Link } from '../router.jsx';
import Shell from '../ui/Shell.jsx';
import { Avatar, Pill, PageHead, Btn } from '../ui/primitives.jsx';
import { Drawer, Field, Switch, ChatPicker, TypeChecks, CONTENT_KINDS } from '../ui/forms.jsx';
import { t2d, d2t, chatOptions } from '../data/admin.js';
import { fmt, ago } from '../data/seed.js';

const KIND_LABEL = Object.fromEntries(CONTENT_KINDS);

function maskUrl(u) {
	const i = u.lastIndexOf('/');
	return u.slice(0, i + 1) + u.slice(i + 1, i + 5) + '...';
}

function Types({ types, all }) {
	if (types.length === all.length) return <span className="a-muted">All content</span>;
	return types.map((t) => <Pill key={t}>{KIND_LABEL[t]}</Pill>);
}

function ChatCell({ chat }) {
	return (
		<Link to={chat.kind === 'group' ? `/groups/${chat.id}` : `/users/${chat.id}`} className="a-usercell">
			<Avatar e={chat} size={28} square={chat.kind === 'group'} />
			<span><span className="a-uname">{chat.title || chat.name}</span><span className="a-handle">{chat.kind === 'group' ? chat.type : 'private chat'}</span></span>
		</Link>
	);
}

function Status({ r, onToggle }) {
	return (
		<span className="a-statuscell">
			<Switch checked={r.enabled} onChange={onToggle} />
			<span className={r.enabled ? 'a-ok' : 'a-muted'}>{r.enabled ? 'On' : 'Off'}</span>
		</span>
	);
}

export function ForwardingIndex() {
	const cards = [
		['Telegram to Discord', '/platform-fwd/telegram-discord', 'Mirror new messages from a Telegram group, channel or private chat into a Discord channel through its incoming webhook.', t2d, 'webhook'],
		['Discord to Telegram', '/platform-fwd/discord-telegram', 'Mirror new messages from a Discord channel into a Telegram group, sent by a Telegram bot that is a member there.', d2t, 'route'],
	];
	return (
		<Shell crumbs={[['Platform forwarding', null]]}>
			<PageHead title="Platform forwarding" sub="Mirror messages between platforms. Each direction has its own transport and credentials, so each is set up on its own page." />
			<div className="a-dircards">
				{cards.map(([title, href, desc, rows, noun]) => {
					const on = rows.filter((r) => r.enabled).length;
					return (
						<Link key={href} to={href} className="a-card a-dircard">
							<div className="a-dircard-top"><ArrowLeftRight size={18} /><h2>{title}</h2><ArrowRight size={16} className="a-muted" /></div>
							<p className="a-muted">{desc}</p>
							<div className="a-dircard-foot">
								<span><b className="num">{on}</b> of {rows.length} {noun}s active</span>
								<span className="a-muted">{fmt(rows.reduce((a, r) => a + r.sent, 0))} messages forwarded</span>
							</div>
						</Link>
					);
				})}
			</div>
			<p className="a-muted a-foot">Both directions can run on the same pair of chats. Each side skips messages the other side sent, so nothing loops.</p>
		</Shell>
	);
}

export function TelegramDiscord() {
	const [rows, setRows] = useState(t2d);
	const [edit, setEdit] = useState(null);
	const [showUrl, setShowUrl] = useState(false);
	const toggle = (r) => setRows(rows.map((x) => (x.id === r.id ? { ...x, enabled: !x.enabled } : x)));
	return (
		<Shell crumbs={[['Platform forwarding', '/platform-fwd'], ['Telegram to Discord', null]]}>
			<PageHead title="Telegram to Discord" sub="Each new message in the Telegram chat is posted to the Discord channel behind the webhook."
				actions={<Btn primary icon={Plus} onClick={() => setEdit({ chat: null, url: '', types: CONTENT_KINDS.map((k) => k[0]), enabled: true })}>Add webhook</Btn>} />
			<div className="a-card a-tablecard">
				<div className="a-tablescroll">
					<table className="a-table">
						<thead><tr><th>Telegram chat</th><th>Discord webhook</th><th>Forwards</th><th className="r">Sent</th><th>Last sent</th><th>Status</th><th /></tr></thead>
						<tbody>
							{rows.map((r) => (
								<tr key={r.id} className={r.enabled ? '' : 'a-off'}>
									<td><ChatCell chat={r.chat} /></td>
									<td className="mono a-dim">{maskUrl(r.url)}</td>
									<td className="a-wrapcell"><Types types={r.types} all={CONTENT_KINDS} /></td>
									<td className="r num">{fmt(r.sent)}</td>
									<td title={r.last}>{ago(r.last)}</td>
									<td><Status r={r} onToggle={() => toggle(r)} /></td>
									<td className="a-rowactions">
										<button type="button" className="a-icon-btn" aria-label="Edit" onClick={() => setEdit(r)}><Pencil size={15} /></button>
										<button type="button" className="a-icon-btn a-danger-hover" aria-label="Delete"><Trash2 size={15} /></button>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
				<ul className="a-list">
					{rows.map((r) => (
						<li key={r.id}>
							<div className="a-list-row">
								<Avatar e={r.chat} size={36} square={r.chat.kind === 'group'} />
								<span className="a-list-main"><span className="a-uname">{r.chat.title || r.chat.name}</span><span className="a-handle">{fmt(r.sent)} sent . last {ago(r.last)}</span></span>
								<Switch checked={r.enabled} onChange={() => toggle(r)} />
								<button type="button" className="a-icon-btn" aria-label="Edit" onClick={() => setEdit(r)}><Pencil size={15} /></button>
							</div>
						</li>
					))}
				</ul>
			</div>
			{edit && (
				<Drawer title={edit.id ? 'Edit webhook' : 'Add webhook'} onClose={() => setEdit(null)}
					footer={<><Btn ghost icon={Send}>Send test message</Btn><span className="a-grow" /><Btn ghost onClick={() => setEdit(null)}>Cancel</Btn><Btn primary onClick={() => setEdit(null)}>Save</Btn></>}>
					<Field label="Telegram chat" hint="A group, channel or user the logger has already seen.">
						<ChatPicker value={edit.chat} onChange={(c) => setEdit({ ...edit, chat: c })} options={chatOptions} />
					</Field>
					<Field label="Discord webhook URL" hint="Discord: channel settings, Integrations, Webhooks, Copy Webhook URL.">
						<span className="a-inputwrap">
							<input type={showUrl ? 'text' : 'password'} value={edit.url} onChange={(e) => setEdit({ ...edit, url: e.target.value })} placeholder="https://discord.com/api/webhooks/..." autoComplete="off" />
							<button type="button" className="a-icon-btn sm" onClick={() => setShowUrl((v) => !v)} aria-label={showUrl ? 'Hide' : 'Show'}>{showUrl ? <EyeOff size={14} /> : <Eye size={14} />}</button>
						</span>
					</Field>
					<Field label="Content to forward" hint="Each part of a message is filtered on its own: a caption follows Text message, the media its own type.">
						<TypeChecks value={edit.types} onChange={(t) => setEdit({ ...edit, types: t })} />
					</Field>
					<Switch checked={edit.enabled} onChange={(v) => setEdit({ ...edit, enabled: v })} label="Enabled" />
				</Drawer>
			)}
		</Shell>
	);
}

export function DiscordTelegram() {
	const [rows, setRows] = useState(d2t);
	const [edit, setEdit] = useState(null);
	const toggle = (r) => setRows(rows.map((x) => (x.id === r.id ? { ...x, enabled: !x.enabled } : x)));
	const kinds = CONTENT_KINDS.filter((k) => k[0] !== 'other');
	return (
		<Shell crumbs={[['Platform forwarding', '/platform-fwd'], ['Discord to Telegram', null]]}>
			<PageHead title="Discord to Telegram" sub="Each new message in the Discord channel is sent to the Telegram chat by a bot that is a member there."
				actions={<Btn primary icon={Plus} onClick={() => setEdit({ channel: '', chat: null, token: '', types: kinds.map((k) => k[0]), enabled: true })}>Add route</Btn>} />
			<div className="a-card a-tablecard">
				<div className="a-tablescroll">
					<table className="a-table">
						<thead><tr><th>Discord channel</th><th /><th>Telegram chat</th><th>Sending bot</th><th>Forwards</th><th className="r">Sent</th><th>Status</th><th /></tr></thead>
						<tbody>
							{rows.map((r) => (
								<tr key={r.id} className={r.enabled ? '' : 'a-off'}>
									<td><span className="a-uname">{r.channel_name}</span><div className="mono a-dim sm">{r.channel}</div></td>
									<td className="a-muted"><ArrowRight size={14} /></td>
									<td><ChatCell chat={r.chat} /></td>
									<td>{r.bot}</td>
									<td className="a-wrapcell"><Types types={r.types} all={kinds} /></td>
									<td className="r num">{fmt(r.sent)}</td>
									<td><Status r={r} onToggle={() => toggle(r)} /></td>
									<td className="a-rowactions">
										<button type="button" className="a-icon-btn" aria-label="Edit" onClick={() => setEdit(r)}><Pencil size={15} /></button>
										<button type="button" className="a-icon-btn a-danger-hover" aria-label="Delete"><Trash2 size={15} /></button>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
				<ul className="a-list">
					{rows.map((r) => (
						<li key={r.id}>
							<div className="a-list-row">
								<span className="a-list-main"><span className="a-uname">{r.channel_name} <ArrowRight size={12} /> {r.chat.title}</span><span className="a-handle">via {r.bot}</span></span>
								<Switch checked={r.enabled} onChange={() => toggle(r)} />
								<button type="button" className="a-icon-btn" aria-label="Edit" onClick={() => setEdit(r)}><Pencil size={15} /></button>
							</div>
						</li>
					))}
				</ul>
			</div>
			{edit && (
				<Drawer title={edit.id ? 'Edit route' : 'Add route'} onClose={() => setEdit(null)}
					footer={<><span className="a-grow" /><Btn ghost onClick={() => setEdit(null)}>Cancel</Btn><Btn primary onClick={() => setEdit(null)}>Save</Btn></>}>
					<Field label="Discord channel ID" hint="Developer mode on, then right-click the channel and Copy Channel ID.">
						<input value={edit.channel} onChange={(e) => setEdit({ ...edit, channel: e.target.value })} inputMode="numeric" placeholder="1093447120571433021" />
					</Field>
					<Field label="Telegram chat">
						<ChatPicker value={edit.chat} onChange={(c) => setEdit({ ...edit, chat: c })} options={chatOptions.filter((c) => c.kind === 'group')} />
					</Field>
					<Field label="Bot token" hint={edit.id ? `Stored for ${edit.bot}. Leave empty to keep it.` : 'From @BotFather. The bot must already be a member of the chat.'}>
						<input type="password" value={edit.token} onChange={(e) => setEdit({ ...edit, token: e.target.value })} placeholder={edit.id ? '(unchanged)' : '123456:ABC-...'} autoComplete="off" />
					</Field>
					<Field label="Content to forward">
						<TypeChecks value={edit.types} onChange={(t) => setEdit({ ...edit, types: t })} kinds={kinds} />
					</Field>
					<Switch checked={edit.enabled} onChange={(v) => setEdit({ ...edit, enabled: v })} label="Enabled" />
				</Drawer>
			)}
		</Shell>
	);
}

