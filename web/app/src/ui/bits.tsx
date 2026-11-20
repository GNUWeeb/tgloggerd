import { BadgeCheck } from 'lucide-react';
import { Link } from '../nav.tsx';
import { Avatar, Pill, type Tone } from './primitives.tsx';
import type { GroupSummary, UserSummary } from '../types.ts';
import { useFreshGroup, useFreshUser } from '../live/useEntities.tsx';

export function userFlags(u: UserSummary): [string, Tone][] {
	const f: [string, Tone][] = [];
	if (u.is_verified) f.push(['verified', 'ok']);
	if (u.is_premium) f.push(['premium', 'accent']);
	if (u.type === 'bot') f.push(['bot', 'info']);
	if (u.is_support) f.push(['support', 'info']);
	if (u.is_scam) f.push(['scam', 'danger']);
	if (u.is_fake) f.push(['fake', 'danger']);
	if (u.type === 'deleted') f.push(['deleted', 'muted']);
	return f;
}

export function Flags({ flags }: { flags: [string, Tone][] }) {
	return <>{flags.map(([t, tone]) => <Pill key={t} tone={tone}>{t}</Pill>)}</>;
}

export function UserCell({ u: page, size = 28 }: { u: UserSummary; size?: number }) {
	const u = useFreshUser(page);
	return (
		<Link to={`/users/${u.id}`} className="a-usercell">
			<Avatar id={u.id} name={u.name} photo={u.photo} size={size} deleted={u.type === 'deleted'} />
			<span>
				<span className="a-uname">{u.name}{u.is_verified && <BadgeCheck size={13} className="a-verified" aria-label="verified" />}</span>
				{u.username && <span className="a-handle">@{u.username}</span>}
			</span>
		</Link>
	);
}

export function GroupCell({ g: page, size = 28 }: { g: GroupSummary; size?: number }) {
	const g = useFreshGroup(page);
	return (
		<Link to={`/groups/${g.id}`} className="a-usercell">
			<Avatar id={g.id} name={g.title} photo={g.photo} size={size} square />
			<span>
				<span className="a-uname">{g.title}</span>
				{g.username && <span className="a-handle">@{g.username}</span>}
			</span>
		</Link>
	);
}

export function Bar({ value, max }: { value: number; max: number }) {
	return <span className="a-bar"><span style={{ width: `${(value / Math.max(1, max)) * 100}%` }} /></span>;
}

// A small line of daily counts, for the profile pages' stat cards.
export function Sparkline({ data, height = 44 }: { data: number[]; height?: number }) {
	const width = 220;
	const max = Math.max(1, ...data);
	const pts = data.map((v, i) => [(i / Math.max(1, data.length - 1)) * width, height - 2 - (v / max) * (height - 6)] as const);
	const d = pts.map((p, i) => (i ? 'L' : 'M') + p[0].toFixed(1) + ' ' + p[1].toFixed(1)).join(' ');
	return (
		<svg width="100%" height={height} viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-hidden="true">
			<path d={`${d} L ${width} ${height} L 0 ${height} Z`} fill="currentColor" opacity="0.12" />
			<path d={d} fill="none" stroke="currentColor" strokeWidth="1.6" vectorEffect="non-scaling-stroke" />
		</svg>
	);
}
