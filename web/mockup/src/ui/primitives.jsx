import { Link } from '../router.jsx';

function initials(name) {
	const parts = String(name || '').replace(/[^\p{L}\p{N} ]/gu, '').trim().split(/\s+/);
	return ((parts[0]?.[0] || '') + (parts[1]?.[0] || '')).toUpperCase() || '?';
}

// Stand-in for a profile/group photo: a gradient with initials.
export function Avatar({ e, size = 32, square = false, className = '' }) {
	const style = {
		width: size, height: size, borderRadius: square ? Math.round(size * 0.28) : '50%', flex: 'none',
		display: 'inline-grid', placeItems: 'center',
		fontSize: Math.round(size * 0.38), fontWeight: 600, color: '#fff', userSelect: 'none',
	};
	if (e.type === 'deleted') {
		return <span className={className} style={{ ...style, background: 'var(--avatar-deleted)' }}>?</span>;
	}
	const bg = e.has_photo === false
		? `hsl(${e.hue} 35% 55%)`
		: `linear-gradient(135deg, hsl(${e.hue} 70% 62%), hsl(${(e.hue + 40) % 360} 65% 48%))`;
	return <span className={className} style={{ ...style, background: bg }}>{initials(e.name || e.title)}</span>;
}

export function Pill({ tone = '', children }) {
	return <span className={`a-pill ${tone ? 'a-pill-' + tone : ''}`}>{children}</span>;
}

export function Flags({ flags }) {
	return flags.map((f) => <Pill key={f[0]} tone={f[1]}>{f[0]}</Pill>);
}

export function Sparkline({ data, width = 220, height = 44, bars = false, className = '' }) {
	const max = Math.max(1, ...data);
	if (bars) {
		const w = width / data.length;
		return (
			<svg className={className} width="100%" height={height} viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-hidden="true">
				{data.map((v, i) => {
					const h = Math.max(1, (v / max) * (height - 2));
					return <rect key={i} x={i * w + 1} y={height - h} width={Math.max(1, w - 2)} height={h} rx="1.5" fill="currentColor" opacity={i === data.length - 1 ? 1 : 0.55} />;
				})}
			</svg>
		);
	}
	const pts = data.map((v, i) => [(i / (data.length - 1)) * width, height - 2 - (v / max) * (height - 6)]);
	const d = pts.map((p, i) => (i ? 'L' : 'M') + p[0].toFixed(1) + ' ' + p[1].toFixed(1)).join(' ');
	return (
		<svg className={className} width="100%" height={height} viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-hidden="true">
			<path d={`${d} L ${width} ${height} L 0 ${height} Z`} fill="currentColor" opacity="0.12" />
			<path d={d} fill="none" stroke="currentColor" strokeWidth="1.6" vectorEffect="non-scaling-stroke" />
		</svg>
	);
}

export function PageHead({ title, sub, actions }) {
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

export function Card({ title, action, children, className = '' }) {
	return (
		<section className={`a-card ${className}`}>
			{(title || action) && <div className="a-cardhead">{title && <h2>{title}</h2>}{action}</div>}
			{children}
		</section>
	);
}

export function Stat({ label, value, sub, children }) {
	return (
		<div className={`a-card a-stat ${children ? 'a-stat-spark' : ''}`}>
			<span className="a-muted">{label}</span>
			<b className="num">{value}</b>
			{sub && <span className="a-muted sm">{sub}</span>}
			{children}
		</div>
	);
}

export function Tabs({ items }) {
	return (
		<nav className="a-tabs">
			{items.map(([label, href, on, count]) => (
				<Link key={label} to={href} className={on ? 'on' : ''}>
					{label}{count !== undefined && <span className="a-tabcount">{count}</span>}
				</Link>
			))}
		</nav>
	);
}

export function Seg({ items, value, onChange }) {
	return (
		<div className="a-seg" role="tablist">
			{items.map(([k, label]) => (
				<button key={k} type="button" role="tab" aria-selected={value === k} className={value === k ? 'on' : ''} onClick={() => onChange(k)}>{label}</button>
			))}
		</div>
	);
}

export function Empty({ children }) {
	return <div className="a-empty">{children}</div>;
}

export function KV({ rows }) {
	return (
		<dl className="a-kv">
			{rows.filter(Boolean).map(([Icon, k, v, mono]) => [
				<dt key={k + 't'}>{Icon && <Icon size={14} />}{k}</dt>,
				<dd key={k + 'd'} className={mono ? 'mono' : ''}>{v === '' || v === null || v === undefined ? <span className="a-muted">-</span> : v}</dd>,
			])}
		</dl>
	);
}

export function Btn({ to, primary, ghost, danger, icon: Icon, children, ...rest }) {
	const cls = `a-btn ${primary ? 'a-btn-primary' : ''} ${ghost ? 'a-btn-ghost' : ''} ${danger ? 'a-btn-danger' : ''}`;
	const inner = <>{Icon && <Icon size={14} />}{children}</>;
	if (to) return <Link to={to} className={cls} {...rest}>{inner}</Link>;
	return <button type="button" className={cls} {...rest}>{inner}</button>;
}
