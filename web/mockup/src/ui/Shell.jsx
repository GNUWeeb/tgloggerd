import { useState } from 'react';
import {
	LayoutDashboard, Users, UsersRound, MessagesSquare, MessageCircle, FileText,
	ArrowLeftRight, Bot, Settings, Search, Menu, LogOut, Sun, Moon, Monitor,
} from 'lucide-react';
import { Link, useRouter } from '../router.jsx';
import { useTheme } from './hooks.js';
import { compact } from '../data/seed.js';
import { TOTALS } from '../data/totals.js';

const NAV = [
	{ label: null, items: [['/', 'Dashboard', LayoutDashboard]] },
	{ label: 'Archive', items: [
		['/users', 'Users', Users, TOTALS.users],
		['/groups', 'Groups', UsersRound, TOTALS.groups],
		['/group_messages', 'Group messages', MessagesSquare, TOTALS.group_messages],
		['/private_messages', 'Private messages', MessageCircle, TOTALS.private_messages],
		['/files', 'Files', FileText, TOTALS.files],
	] },
	{ label: 'Integrations', items: [
		['/platform-fwd', 'Platform forwarding', ArrowLeftRight],
		['/mcp-admin', 'MCP', Bot],
	] },
	{ label: null, items: [['/settings', 'Settings', Settings]] },
];

function isOn(path, href) {
	if (href === '/') return path === '/';
	if (href === '/group_messages') return path.startsWith('/group_messages') || path.startsWith('/messages/group');
	if (href === '/private_messages') return path.startsWith('/private_messages') || path.startsWith('/messages/private');
	return path === href || path.startsWith(href + '/');
}

export function ThemeButton() {
	const [mode, cycle] = useTheme();
	const Icon = mode === 'light' ? Sun : mode === 'dark' ? Moon : Monitor;
	return (
		<button type="button" className="a-icon-btn" onClick={cycle} title={`Theme: ${mode} (click to change)`} aria-label={`Theme: ${mode}`}>
			<Icon size={16} />
		</button>
	);
}

// crumbs: [[label, href|null], ...]; flush: content fills the viewport
// (chat views), with no page padding or max width.
export default function Shell({ crumbs = [], flush = false, children }) {
	const { path } = useRouter();
	const [open, setOpen] = useState(false);
	return (
		<div className="app">
			<div className={`a-shell ${open ? 'nav-open' : ''} ${flush ? 'a-flush' : ''}`}>
				<aside className="a-side" onClick={(e) => e.target.closest('a') && setOpen(false)}>
					<Link to="/" className="a-brand">
						<span className="a-logo">tg</span>
						<span>tgloggerd</span>
					</Link>
					<nav aria-label="Main">
						{NAV.map((g, i) => (
							<div key={i} className="a-navgroup">
								{g.label && <div className="a-navlabel">{g.label}</div>}
								{g.items.map(([href, label, Icon, n]) => (
									<Link key={label} to={href} className={`a-navitem ${isOn(path, href) ? 'on' : ''}`} aria-current={isOn(path, href) ? 'page' : undefined}>
										<Icon size={16} strokeWidth={1.8} />
										<span>{label}</span>
										{n !== undefined && <span className="a-navcount">{compact(n)}</span>}
									</Link>
								))}
							</div>
						))}
					</nav>
					<div className="a-me">
						<span className="a-me-av">A</span>
						<span className="a-grow">admin</span>
						<ThemeButton />
						<Link to="/login" className="a-icon-btn" title="Sign out" aria-label="Sign out"><LogOut size={15} /></Link>
					</div>
				</aside>
				<div className="a-scrim" onClick={() => setOpen(false)} />
				<div className="a-main">
					<header className="a-top">
						<button type="button" className="a-icon-btn a-menu" onClick={() => setOpen(true)} aria-label="Open menu"><Menu size={18} /></button>
						<nav className="a-crumbs" aria-label="Breadcrumb">
							{crumbs.map(([t, h], i) => (
								<span key={i} className="a-crumb">
									{i > 0 && <span className="a-sep">/</span>}
									{h && i < crumbs.length - 1 ? <Link to={h}>{t}</Link> : <span className="a-cur">{t}</span>}
								</span>
							))}
						</nav>
						<button type="button" className="a-cmdk" aria-label="Search">
							<Search size={14} /><span>Search everything...</span><kbd>Ctrl K</kbd>
						</button>
					</header>
					<main className="a-content">{children}</main>
				</div>
			</div>
		</div>
	);
}
