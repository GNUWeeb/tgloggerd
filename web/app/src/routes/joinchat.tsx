// Integrations > Join chat: preview a group by invite link or username,
// choose federations to file it in, join (web/docs/join-chat.md, with
// the federation step of web-accounts.md section 3).
import { useState, type FormEvent } from 'react';
import { CircleCheck, Clock, Info, LogIn, Network, Search, TriangleAlert, Users } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Avatar, Btn, Card, PageHead, Pill, type Tone } from '../ui/primitives.tsx';
import { CheckChips, Field } from '../ui/forms.tsx';
import { ApiError, api, ok } from '../api/client.ts';
import { ago, fmt } from '../format.ts';
import type { PageData } from '../page.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

interface PreviewChat { chat_id: number; title: string; type: string; member_count: number; username: string; is_member: boolean; needs_approval: boolean }
interface Outcome { outcome: string; message: string; chat: PreviewChat | null }

export type JoinChatProps = S['JoinChatProps'];

const OUTCOME: Record<string, [string, Tone]> = {
	joined: ['joined', 'ok'], already_member: ['already a member', ''], request_sent: ['request sent', 'info'],
	approval_required: ['approval required', 'info'], flood_wait: ['rate limited', 'danger'], link_invalid: ['invalid link', 'danger'],
	not_found: ['not found', 'danger'], banned: ['banned', 'danger'], error: ['error', 'danger'],
};

function JoinChat({ page }: { page: PageData<JoinChatProps> }) {
	const v = page.viewer;
	const [ref, setRef] = useState('');
	const [preview, setPreview] = useState<Outcome | null>(null);
	const [result, setResult] = useState<Outcome | null>(null);
	const [feds, setFeds] = useState<string[]>([]);
	const [err, setErr] = useState('');
	const [busy, setBusy] = useState(false);
	const run = async <T,>(fn: () => Promise<T>): Promise<T | null> => {
		setErr(''); setBusy(true);
		try { return await fn(); } catch (x) { setErr(x instanceof ApiError ? x.message : 'The server could not be reached.'); return null; } finally { setBusy(false); }
	};
	const doPreview = async (e: FormEvent) => {
		e.preventDefault();
		setResult(null);
		setPreview(await run(() => ok(api.POST('/api/join-chat/preview', { body: { ref } }))));
	};
	const join = async () => setResult(await run(() => ok(api.POST('/api/join-chat/join', { body: { ref, federation_ids: feds.map(Number) } }))));
	const c = preview?.chat;
	// Without the daemon's API nothing can be joined from here: say so
	// before anything is tried, as the Drogon page did.
	const off = !page.props.api_configured;
	return (
		<Shell crumbs={[['Join chat', null]]}>
			<PageHead title="Join chat" sub="Make the logging account join a group or channel. Once joined, it is logged from then on, media included." />
			<div className="a-cols">
				<div className="a-stack">
					<Card title="Group or channel">
						{off && <div className="a-banner warn" role="status"><TriangleAlert size={15} /><span>Joining is not available: this web service is not connected to the logger's API (WEB_TGLOGGERD_API). An administrator can set it up.</span></div>}
						<form className="a-form" onSubmit={(e) => void doPreview(e)}>
							<Field label="Invite link or username" hint="https://t.me/+..., https://t.me/joinchat/..., @name or t.me/name. Each preview costs one Telegram lookup.">
								<input value={ref} onChange={(e) => setRef(e.target.value)} placeholder="@groupname or https://t.me/+..." autoComplete="off" required disabled={off} />
							</Field>
							<div><button className="a-btn a-btn-primary" disabled={busy || off}><Search size={14} />Preview</button></div>
						</form>
						{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
						{preview && !c && <div className="a-banner warn"><TriangleAlert size={15} /><span>{preview.message}</span></div>}
					</Card>
					{c && (
						<Card title={c.is_member ? 'Already a member' : 'Join this chat?'}>
							<div className="a-joinprev">
								<Avatar id={c.chat_id} name={c.title} size={56} square />
								<div className="a-grow"><b className="a-joint">{c.title}</b><span className="a-muted">{c.username && `@${c.username} . `}{c.type} . <Users size={12} /> {fmt(c.member_count)} members{c.needs_approval && ' . needs approval'}</span></div>
							</div>
							{!c.is_member && <div className="a-banner"><Info size={15} /><span>Joining is visible to the chat's members and counts against Telegram's limits.</span></div>}
							{!c.is_member && page.props.federations.length > 0 && (
								<div className="a-field">
									<span className="a-field-label">Add to federations</span>
									<CheckChips items={page.props.federations.map((f) => [String(f.id), <><Network size={13} />{f.name}</>])} value={feds} onChange={setFeds} />
									<span className="a-field-hint">{feds.length ? 'The group joins these federations once the account is a member.' : 'None chosen: the group stays in Default.'}</span>
								</div>
							)}
							{result ? <div className={`a-banner ${OUTCOME[result.outcome]?.[1] === 'danger' ? 'warn' : ''}`}><CircleCheck size={15} /><span>{result.message}</span></div>
								: !c.is_member && <div className="a-btnrow"><Btn primary icon={LogIn} disabled={busy} onClick={() => void join()}>Join</Btn><Btn ghost onClick={() => setPreview(null)}>Cancel</Btn></div>}
						</Card>
					)}
				</div>
				<Card title="Recent join attempts">
					<ol className="a-audit">{page.props.recent.map((r, i) => {
						const [label, tone] = OUTCOME[r.outcome] ?? [r.outcome, ''];
						return (
							<li key={i}>
								{tone === 'danger' ? <TriangleAlert size={14} /> : r.outcome === 'request_sent' ? <Clock size={14} /> : <CircleCheck size={14} />}
								<span className="a-grow a-minw"><span className="a-ellipsis">{r.title || r.ref}</span><span className="a-muted sm a-ellipsis">{r.ref} . by {r.by}</span></span>
								<Pill tone={tone}>{label}</Pill>
								<time className="a-muted sm" dateTime={r.at}>{ago(r.at, v)}</time>
							</li>
						);
					})}</ol>
				</Card>
			</div>
		</Shell>
	);
}

export const joinChatRoute: RouteDef<JoinChatProps> = { Page: JoinChat, head: () => ({ title: 'Join chat - tgloggerd', robots: 'noindex' }) };
