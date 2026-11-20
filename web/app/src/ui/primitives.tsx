import type { ReactNode, ComponentType, ButtonHTMLAttributes } from 'react';
import { Link } from '../nav.tsx';

function initials(name: string): string {
	const parts = name.replace(/[^\p{L}\p{N} ]/gu, '').trim().split(/\s+/);
	return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase() || '?';
}

// A photo when the archive has one, else initials on a colour derived
// from the id (stable across renders and between server and browser).
export function Avatar({ id, name, photo, size = 32, square = false, deleted = false }: { id: number; name: string; photo?: string | null; size?: number; square?: boolean; deleted?: boolean }) {
	const radius = square ? Math.round(size * 0.28) : '50%';
	const base = { width: size, height: size, borderRadius: radius, flex: 'none' as const };
	if (photo) return <img src={photo} alt="" width={size} height={size} style={{ ...base, objectFit: 'cover' }} loading="lazy" />;
	const hue = Math.abs(id) % 360;
	return (
		<span aria-hidden="true" style={{
			...base, display: 'inline-grid', placeItems: 'center', color: '#fff', fontWeight: 600, fontSize: Math.round(size * 0.38),
			background: deleted ? 'var(--avatar-deleted)' : `linear-gradient(135deg, hsl(${hue} 65% 60%), hsl(${(hue + 40) % 360} 60% 46%))`,
		}}>{deleted ? '?' : initials(name)}</span>
	);
}

export type Tone = '' | 'accent' | 'ok' | 'danger' | 'info' | 'muted';
export function Pill({ tone = '', children }: { tone?: Tone; children: ReactNode }) {
	return <span className={`a-pill ${tone ? 'a-pill-' + tone : ''}`}>{children}</span>;
}

export function PageHead({ title, sub, actions }: { title: ReactNode; sub?: ReactNode; actions?: ReactNode }) {
	return (
		<div className="a-pagehead">
			<div>
				<h1>{title}</h1>
				{sub && <p className="a-sub">{sub}</p>}
			</div>
			{actions && <div className="a-head-actions">{actions}</div>}
		</div>
	);
}

export function Card({ title, action, children, className = '' }: { title?: ReactNode; action?: ReactNode; children: ReactNode; className?: string }) {
	return (
		<section className={`a-card ${className}`}>
			{(title || action) && <div className="a-cardhead">{title && <h2>{title}</h2>}{action}</div>}
			{children}
		</section>
	);
}

export function Stat({ label, value, sub }: { label: string; value: ReactNode; sub?: ReactNode }) {
	return (
		<div className="a-card a-stat">
			<span className="a-muted">{label}</span>
			<b className="num">{value}</b>
			{sub && <span className="a-muted sm">{sub}</span>}
		</div>
	);
}

export function Tabs({ items }: { items: [label: string, href: string, on: boolean, count?: ReactNode][] }) {
	return (
		<nav className="a-tabs">
			{items.map(([label, href, on, count]) => (
				<Link key={label} to={href} className={on ? 'on' : ''} aria-current={on ? 'page' : undefined}>
					{label}{count !== undefined && <span className="a-tabcount">{count}</span>}
				</Link>
			))}
		</nav>
	);
}

type Icon = ComponentType<{ size?: number }>;

export function KV({ rows }: { rows: (false | null | undefined | [Icon | null, string, ReactNode, boolean?])[] }) {
	return (
		<dl className="a-kv">
			{rows.filter((r): r is [Icon | null, string, ReactNode, boolean?] => !!r).map(([I, k, v, mono]) => [
				<dt key={k + 't'}>{I && <I size={14} />}{k}</dt>,
				<dd key={k + 'd'} className={mono ? 'mono' : ''}>{v === '' || v === null || v === undefined ? <span className="a-muted">-</span> : v}</dd>,
			])}
		</dl>
	);
}

export function Btn({ to, primary, ghost, danger, icon: I, children, ...rest }: { to?: string; primary?: boolean; ghost?: boolean; danger?: boolean; icon?: Icon; children?: ReactNode } & ButtonHTMLAttributes<HTMLButtonElement>) {
	const cls = `a-btn ${primary ? 'a-btn-primary' : ''} ${ghost ? 'a-btn-ghost' : ''} ${danger ? 'a-btn-danger' : ''}`;
	const inner = <>{I && <I size={14} />}{children}</>;
	if (to) return <Link to={to} className={cls}>{inner}</Link>;
	return <button type="button" className={cls} {...rest}>{inner}</button>;
}

export function Empty({ children }: { children: ReactNode }) {
	return <div className="a-empty">{children}</div>;
}
