// Integrations > Join chat: make the logging account join a group by
// invite link or username, and file it into federations on the way.
import { useState } from 'react';
import { LogIn, Search, Network, Users, CircleCheck, Clock, TriangleAlert, Info } from 'lucide-react';
import Shell from '../ui/Shell.jsx';
import { Avatar, Pill, PageHead, Btn, Card } from '../ui/primitives.jsx';
import { Field } from '../ui/forms.jsx';
import { useViewer } from '../viewer.jsx';
import { federations } from '../data/accounts.js';
import { NOW, iso, ago, fmt } from '../data/seed.js';

const RECENT = [
	{ at: iso(NOW - 3 * 3600000), by: 'gw_admin', ref: '@asm_lounge', title: 'Assembly Lounge', outcome: 'joined' },
	{ at: iso(NOW - 26 * 3600000), by: 'admin', ref: 'https://t.me/+AbCdEfGhIjKl', title: 'Homelab & Self-hosting', outcome: 'request_sent' },
	{ at: iso(NOW - 2 * 86400000), by: 'gw_admin', ref: '@rustlang_id', title: 'Rust Programming', outcome: 'already_member' },
	{ at: iso(NOW - 4 * 86400000), by: 'admin', ref: '@some_big_channel', title: 'Some Big Channel', outcome: 'flood_wait' },
	{ at: iso(NOW - 9 * 86400000), by: 'admin', ref: 'https://t.me/+ZzZzExpired', title: '', outcome: 'link_invalid' },
];
const OUTCOME = {
	joined: ['joined', 'ok'], already_member: ['already a member', ''], request_sent: ['request sent', 'info'],
	flood_wait: ['rate limited', 'danger'], link_invalid: ['invalid link', 'danger'],
};

export default function JoinChat() {
	const v = useViewer();
	const [ref, setRef] = useState('@compilernerds_id');
	const [preview, setPreview] = useState(null);
	const [chosen, setChosen] = useState([]);
	const [done, setDone] = useState(false);
	// Superadmins may file into any federation; an admin into its own.
	const feds = v.role === 'superadmin' ? federations : v.manageableFeds.filter((f) => f.id !== 0);
	const toggle = (id) => setChosen(chosen.includes(id) ? chosen.filter((x) => x !== id) : [...chosen, id]);
	return (
		<Shell crumbs={[['Join chat', null]]}>
			<PageHead title="Join chat" sub="Make the logging account join a group or channel. Once joined, it is logged from then on, media included." />
			<div className="a-cols">
				<div className="a-stack">
					<Card title="Group or channel">
						<form className="a-form" onSubmit={(e) => { e.preventDefault(); setDone(false); setPreview({ title: 'Compiler Nerds ID', type: 'supergroup', members: 1843, username: 'compilernerds_id', needs_approval: false, hue: 28 }); }}>
							<Field label="Invite link or username" hint="https://t.me/+..., https://t.me/joinchat/..., @name or t.me/name. Previewing costs one Telegram lookup.">
								<input value={ref} onChange={(e) => setRef(e.target.value)} placeholder="@groupname or https://t.me/+..." autoComplete="off" />
							</Field>
							<div><Btn primary icon={Search} type="submit">Preview</Btn></div>
						</form>
					</Card>
					{preview && (
						<Card title="Join this chat?">
							<div className="a-joinprev">
								<Avatar e={{ ...preview, has_photo: true }} size={56} square />
								<div className="a-grow">
									<b className="a-joint">{preview.title}</b>
									<span className="a-muted">@{preview.username} . {preview.type} . <Users size={12} /> {fmt(preview.members)} members</span>
								</div>
							</div>
							<div className="a-banner"><Info size={15} /><span>Joining is visible to the chat's members and counts against Telegram's limits.</span></div>
							{feds.length > 0 && (
								<div className="a-field">
									<span className="a-field-label">Add to federations</span>
									<div className="a-checkgrid">
										{feds.map((f) => {
											const on = chosen.includes(f.id);
											return (
												<label key={f.id} className={`a-checkchip ${on ? 'on' : ''}`}>
													<input type="checkbox" checked={on} onChange={() => toggle(f.id)} />
													<Network size={13} />{f.name}
												</label>
											);
										})}
									</div>
									<span className="a-field-hint">{chosen.length ? 'The group joins these federations once the account is a member.' : 'None chosen: the group stays in Default.'}</span>
								</div>
							)}
							{done
								? <div className="a-banner"><CircleCheck size={15} /><span>Joined. The chat is now logged, media included{chosen.length ? `, and is in ${chosen.length} federation${chosen.length > 1 ? 's' : ''}` : ''}.</span></div>
								: <div className="a-btnrow"><Btn primary icon={LogIn} onClick={() => setDone(true)}>Join</Btn><Btn ghost onClick={() => setPreview(null)}>Cancel</Btn></div>}
						</Card>
					)}
				</div>
				<Card title="Recent join attempts">
					<ol className="a-audit">
						{RECENT.map((r, i) => {
							const [label, tone] = OUTCOME[r.outcome];
							return (
								<li key={i}>
									{r.outcome === 'flood_wait' || r.outcome === 'link_invalid' ? <TriangleAlert size={14} /> : r.outcome === 'request_sent' ? <Clock size={14} /> : <CircleCheck size={14} />}
									<span className="a-grow a-minw">
										<span className="a-ellipsis">{r.title || r.ref}</span>
										<span className="a-muted sm a-ellipsis">{r.ref} . by {r.by}</span>
									</span>
									<Pill tone={tone}>{label}</Pill>
									<time className="a-muted sm" title={r.at}>{ago(r.at)}</time>
								</li>
							);
						})}
					</ol>
				</Card>
			</div>
		</Shell>
	);
}
