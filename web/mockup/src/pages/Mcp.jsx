import { useState } from 'react';
import { ArrowRight, Plus, Copy, Globe, KeyRound, TriangleAlert, Trash2, Check, ShieldAlert } from 'lucide-react';
import { Link } from '../router.jsx';
import Shell from '../ui/Shell.jsx';
import { Avatar, Pill, PageHead, Btn, Card } from '../ui/primitives.jsx';
import { Drawer, Field, ChatPicker } from '../ui/forms.jsx';
import { GroupCell } from './Groups.jsx';
import { mcpExposed, mcpTokens } from '../data/admin.js';
import { groups } from '../data/groups.js';
import { fmt, ago, compact } from '../data/seed.js';

export function McpIndex() {
	const live = mcpTokens.filter((t) => !t.revoked).length;
	const msgs = mcpExposed.reduce((a, e) => a + e.group.msg_count, 0);
	return (
		<Shell crumbs={[['MCP', null]]}>
			<PageHead title="MCP" sub="The Model Context Protocol endpoint lets a language model read this archive directly. Two things gate it: which groups are exposed, and who holds a token." />
			<div className="a-dircards">
				<Link to="/mcp-admin/groups" className="a-card a-dircard">
					<div className="a-dircard-top"><Globe size={18} /><h2>Exposed groups</h2><ArrowRight size={16} className="a-muted" /></div>
					<p className="a-muted">The allowlist. Nothing is readable through MCP until a group is added here. Private groups and direct messages are never exposed.</p>
					<div className="a-dircard-foot"><span><b className="num">{mcpExposed.length}</b> groups</span><span className="a-muted">{compact(msgs)} messages readable</span></div>
				</Link>
				<Link to="/mcp-admin/tokens" className="a-card a-dircard">
					<div className="a-dircard-top"><KeyRound size={18} /><h2>Access tokens</h2><ArrowRight size={16} className="a-muted" /></div>
					<p className="a-muted">Bearer tokens for MCP clients. Shown once when minted and stored only as a hash, so a lost token is replaced, never recovered.</p>
					<div className="a-dircard-foot"><span><b className="num">{live}</b> active tokens</span><span className="a-muted">last used {ago(mcpTokens[0].last_used)}</span></div>
				</Link>
			</div>
			<Card title="Connecting a client" className="a-mt">
				<p className="a-muted">Point an MCP client at this endpoint and pass the token as a bearer credential. The endpoint is read only: it lists exposed groups, reads and searches their messages, and looks up users.</p>
				<div className="a-code">
					<pre>{'URL            https://tgd.example.org/mcp\nAuthorization  Bearer tgmcp_...'}</pre>
					<button type="button" className="a-icon-btn" aria-label="Copy"><Copy size={14} /></button>
				</div>
			</Card>
		</Shell>
	);
}

export function McpGroups() {
	const [adding, setAdding] = useState(false);
	const [pick, setPick] = useState(null);
	return (
		<Shell crumbs={[['MCP', '/mcp-admin'], ['Exposed groups', null]]}>
			<PageHead title="Exposed groups" sub="Only these groups can be read through MCP. Everything else in the archive stays private."
				actions={<Btn primary icon={Plus} onClick={() => setAdding(true)}>Expose a group</Btn>} />
			<div className="a-banner warn">
				<ShieldAlert size={15} />
				<span><b>Exposing a group also publishes its chat on the web.</b> Anyone can read its history and files at /groups/&lt;id&gt;/chat without signing in.</span>
			</div>
			<div className="a-card a-tablecard">
				<div className="a-tablescroll">
					<table className="a-table">
						<thead><tr><th>Group</th><th className="r">Messages</th><th>Public on Telegram</th><th>Note</th><th>Added</th><th /></tr></thead>
						<tbody>
							{mcpExposed.map((e) => (
								<tr key={e.group.id}>
									<td><GroupCell g={e.group} /></td>
									<td className="r num">{fmt(e.group.msg_count)}</td>
									<td>{e.public_now ? <Pill tone="ok">public</Pill> : <Pill tone="danger"><TriangleAlert size={11} />no longer public</Pill>}</td>
									<td className="a-muted a-clip">{e.note}</td>
									<td title={e.added}>{ago(e.added)}</td>
									<td className="a-rowactions"><Btn ghost danger icon={Trash2}>Remove</Btn></td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
				<ul className="a-list">
					{mcpExposed.map((e) => (
						<li key={e.group.id}>
							<Link to={`/groups/${e.group.id}`}>
								<Avatar e={e.group} size={36} square />
								<span className="a-list-main"><span className="a-uname">{e.group.title}</span><span className="a-handle">{e.note}</span></span>
								{!e.public_now && <TriangleAlert size={16} className="a-warnicon" />}
							</Link>
						</li>
					))}
				</ul>
			</div>
			<p className="a-muted a-foot"><b>Public on Telegram</b> says whether the group still has a public username. A group that went private stays exposed until it is removed here.</p>
			{adding && (
				<Drawer title="Expose a group" onClose={() => setAdding(false)}
					footer={<><span className="a-grow" /><Btn ghost onClick={() => setAdding(false)}>Cancel</Btn><Btn primary onClick={() => setAdding(false)}>Expose</Btn></>}>
					<Field label="Group" hint="Busiest groups first. Groups already exposed are not listed.">
						<ChatPicker value={pick} onChange={setPick} options={groups.filter((g) => !g.exposed).slice(0, 40)} />
					</Field>
					<Field label="Note" hint="Why it is exposed, for whoever reviews this later.">
						<input maxLength={255} placeholder="e.g. public community, admins agreed" />
					</Field>
					<div className="a-banner warn"><ShieldAlert size={15} /><span>Its whole history becomes readable on the web without signing in.</span></div>
				</Drawer>
			)}
		</Shell>
	);
}

export function McpTokens() {
	const [minting, setMinting] = useState(false);
	const [minted, setMinted] = useState(null);
	const [copied, setCopied] = useState(false);
	return (
		<Shell crumbs={[['MCP', '/mcp-admin'], ['Access tokens', null]]}>
			<PageHead title="Access tokens" sub="A token reads every exposed group, so treat one like a password."
				actions={<Btn primary icon={Plus} onClick={() => setMinting(true)}>Mint a token</Btn>} />
			{minted && (
				<div className="a-card a-minted">
					<div className="a-cardhead"><h2><KeyRound size={15} /> Copy this token now</h2></div>
					<p className="a-muted">This is the only time it is shown. Only a hash is stored; if it is lost, revoke it and mint another.</p>
					<div className="a-code">
						<pre>{minted}</pre>
						<button type="button" className="a-btn" onClick={() => setCopied(true)}>{copied ? <><Check size={14} />Copied</> : <><Copy size={14} />Copy</>}</button>
					</div>
					<Btn ghost onClick={() => setMinted(null)}>Done</Btn>
				</div>
			)}
			<div className="a-card a-tablecard">
				<div className="a-tablescroll">
					<table className="a-table">
						<thead><tr><th>Name</th><th>Owner</th><th>Created</th><th>Last used</th><th>Status</th><th /></tr></thead>
						<tbody>
							{mcpTokens.map((t) => (
								<tr key={t.id} className={t.revoked ? 'a-off' : ''}>
									<td>{t.name ? <b>{t.name}</b> : <span className="a-muted">(unnamed)</span>}</td>
									<td>{t.owner}</td>
									<td title={t.created}>{ago(t.created)}</td>
									<td title={t.last_used}>{t.last_used ? ago(t.last_used) : <span className="a-muted">never</span>}</td>
									<td>{t.revoked ? <span className="a-muted">revoked {ago(t.revoked)}</span> : <Pill tone="ok">active</Pill>}</td>
									<td className="a-rowactions">{!t.revoked && <Btn ghost danger>Revoke</Btn>}</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
				<ul className="a-list">
					{mcpTokens.map((t) => (
						<li key={t.id}>
							<div className="a-list-row">
								<KeyRound size={18} className="a-muted" />
								<span className="a-list-main"><span className="a-uname">{t.name || '(unnamed)'}</span><span className="a-handle">{t.revoked ? `revoked ${ago(t.revoked)}` : `last used ${t.last_used ? ago(t.last_used) : 'never'}`}</span></span>
								{!t.revoked && <Btn ghost danger>Revoke</Btn>}
							</div>
						</li>
					))}
				</ul>
			</div>
			<p className="a-muted a-foot">Revoking keeps the row for the audit trail; the token stops working at once. Deactivating an account disables its tokens too.</p>
			{minting && (
				<Drawer title="Mint a token" onClose={() => setMinting(false)}
					footer={<><span className="a-grow" /><Btn ghost onClick={() => setMinting(false)}>Cancel</Btn><Btn primary onClick={() => { setMinting(false); setCopied(false); setMinted('tgmcp_' + 'x'.repeat(8) + '-example-token-shown-once'); }}>Mint</Btn></>}>
					<Field label="Name" hint="A label, so this one can be revoked later without guessing which is which.">
						<input maxLength={64} placeholder="laptop" autoComplete="off" />
					</Field>
				</Drawer>
			)}
		</Shell>
	);
}
