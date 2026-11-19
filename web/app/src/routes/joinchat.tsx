// Integrations > Join chat: preview a group by invite link or username,
// choose federations to file it in, join (web/docs/join-chat.md, with
// the federation step of web-accounts.md section 3). A join that waits
// for the chat's approval holds its filings, listed here to cancel.
import { useState, type FormEvent } from 'react';
import { CircleCheck, Clock, Info, LogIn, Network, Search, TriangleAlert, Users, X } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Avatar, Btn, Card, PageHead, Pill, type Tone } from '../ui/primitives.tsx';
import { CheckChips, Field } from '../ui/forms.tsx';
import { ApiError, api, ok } from '../api/client.ts';
import { ago, fmt } from '../format.ts';
import { useNav } from '../nav.tsx';
import type { PageData } from '../page.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

type Outcome = S['JoinOutcome'];
type Filing = S['JoinFiling'];

export type JoinChatProps = S['JoinChatProps'];

// Every outcome a join can end in: the daemon's (JoinOutcome.outcome,
// whose description lists them; the schema types it as a plain string,
// so this list is kept by hand) and the two the audit records when the
// daemon gave none. A recent attempt can also read http_<status>.
type JoinResult = 'joined' | 'already_member' | 'request_sent' | 'approval_required' | 'declined' | 'not_a_group' | 'not_found'
	| 'link_invalid' | 'flood_wait' | 'banned' | 'too_many_chats' | 'error' | 'api_off' | 'no_answer';
const OUTCOME: Record<JoinResult, [string, Tone]> = {
	joined: ['joined', 'ok'], already_member: ['already a member', ''], request_sent: ['request sent', 'info'],
	approval_required: ['approval required', 'info'], declined: ['declined', 'danger'], not_a_group: ['not a group', 'danger'],
	flood_wait: ['rate limited', 'danger'], link_invalid: ['invalid link', 'danger'], not_found: ['not found', 'danger'],
	banned: ['banned', 'danger'], too_many_chats: ['too many chats', 'danger'], error: ['error', 'danger'],
	api_off: ['API not set up', 'danger'], no_answer: ['no answer', 'danger'],
};
const outcomeOf = (o: string): [string, Tone] => {
	if (Object.hasOwn(OUTCOME, o)) return OUTCOME[o as JoinResult];
	const http = o.match(/^http_(\d{3})$/);
	return http ? [`API error ${http[1]}`, 'danger'] : [o, ''];
};

function JoinChat({ page }: { page: PageData<JoinChatProps> }) {
	const v = page.viewer;
	const { refresh } = useNav();
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
	const join = async () => {
		const r = await run(() => ok(api.POST('/api/join-chat/join', { body: { ref, federation_ids: feds.map(Number) } })));
		setResult(r);
		// The attempt, and any filing it holds, are in the lists now.
		if (r) void refresh();
	};
	const [pendErr, setPendErr] = useState('');
	const cancel = async (f: Filing) => {
		if (!confirm(`Cancel filing ${f.title || f.ref} into ${f.federation.name}? The group stays out of it when the account becomes a member.`)) return;
		setPendErr('');
		try {
			await ok(api.DELETE('/api/join-chat/pending/{id}', { params: { path: { id: f.id } } }));
			void refresh();
		} catch (x) { setPendErr(x instanceof ApiError ? x.message : 'The server could not be reached.'); }
	};
	const fedName = (id: number) => page.props.federations.find((f) => f.id === id)?.name ?? `Federation ${id}`;
	const filed = result?.federation_ids ?? [];
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
							{!result && !c.is_member && page.props.federations.length > 0 && (
								<div className="a-field">
									<span className="a-field-label">Add to federations</span>
									<CheckChips items={page.props.federations.map((f) => [String(f.id), <><Network size={13} />{f.name}</>])} value={feds} onChange={setFeds} />
									<span className="a-field-hint">{feds.length ? 'The group joins these federations once the account is a member.' : 'None chosen: the group stays in Default.'}</span>
								</div>
							)}
							{result ? <div className={`a-banner ${outcomeOf(result.outcome)[1] === 'danger' ? 'warn' : ''}`}><CircleCheck size={15} /><span>{result.message}</span></div>
								: !c.is_member && <div className="a-btnrow"><Btn primary icon={LogIn} disabled={busy} onClick={() => void join()}>Join</Btn><Btn ghost onClick={() => setPreview(null)}>Cancel</Btn></div>}
							{filed.length > 0 && (
								<div className="a-field">
									<span className="a-field-label" id="join-filed">{result?.filing_pending ? 'To be filed in' : 'Filed in'}</span>
									<ul className="a-chips" aria-labelledby="join-filed">{filed.map((id) => <li key={id}><Pill tone="info"><Network size={12} />{fedName(id)}</Pill></li>)}</ul>
									{result?.filing_pending && <span className="a-field-hint">The filing waits: the group joins these federations once the chat approves the request and the account is a member.</span>}
								</div>
							)}
						</Card>
					)}
				</div>
				<div className="a-stack">
					{page.props.pending.length > 0 && (
						<Card title="Pending filings">
							<p className="a-muted sm">Groups whose join waits for approval, and the federation each goes into once the account is a member.</p>
							{pendErr && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{pendErr}</span></div>}
							<ol className="a-audit">{page.props.pending.map((f) => (
								<li key={f.id}>
									<Clock size={14} />
									<span className="a-grow a-minw"><span className="a-ellipsis">{f.title || f.ref}</span><span className="a-muted sm a-ellipsis">{f.title ? `${f.ref} . ` : ''}by {f.by}</span></span>
									<Pill tone="info"><Network size={12} />{f.federation.name}</Pill>
									<time className="a-muted sm" dateTime={f.at}>{ago(f.at, v)}</time>
									<Btn ghost icon={X} aria-label={`Cancel filing ${f.title || f.ref} into ${f.federation.name}`} onClick={() => void cancel(f)}>Cancel</Btn>
								</li>
							))}</ol>
						</Card>
					)}
					<Card title="Recent join attempts">
						<ol className="a-audit">{page.props.recent.map((r, i) => {
							const [label, tone] = outcomeOf(r.outcome);
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
			</div>
		</Shell>
	);
}

export const joinChatRoute: RouteDef<JoinChatProps> = { Page: JoinChat, head: () => ({ title: 'Join chat - tgloggerd', robots: 'noindex' }) };
