import { useState } from 'react';
import {
	LayoutDashboard, Users, UsersRound, MessagesSquare, MessageCircle, FileText,
	ArrowLeftRight, Bot, Settings, Search, Menu, LogOut, Sun, Moon, Monitor,
	PanelLeftClose, PanelLeftOpen, UserCog, LogIn, Network,
} from 'lucide-react';
import { Link, useRouter } from '../router.jsx';
import { useTheme } from './hooks.js';
import { compact } from '../data/seed.js';
import { TOTALS } from '../data/totals.js';
import { useViewer, allowed } from '../viewer.jsx';

// [href, label, icon, page key, count shown to full-access roles]
const NAV = [
	{ label: null, items: [['/', 'Dashboard', LayoutDashboard, 'dashboard']] },
	{ label: 'Archive', items: [
		['/users', 'Users', Users, 'users', TOTALS.users],
		['/groups', 'Groups', UsersRound, 'groups', TOTALS.groups],
		['/group_messages', 'Group messages', MessagesSquare, 'group_messages', TOTALS.group_messages],
		['/private_messages', 'Private messages', MessageCircle, 'private_messages', TOTALS.private_messages],
		['/files', 'Files', FileText, 'files', TOTALS.files],
	] },
	{ label: 'Integrations', items: [
		['/platform-fwd', 'Platform forwarding', ArrowLeftRight, 'platform_fwd'],
		['/join-chat', 'Join chat', LogIn, 'join_chat'],
		['/mcp-admin', 'MCP', Bot, 'mcp'],
	] },
	{ label: 'Administration', items: [
		['/settings/accounts', 'Accounts', UserCog, 'accounts'],
		['/settings/federations', 'Federations', Network, 'federations'],
	] },
	{ label: null, items: [['/settings', 'Settings', Settings, null]] },
];

function isOn(path, href) {
	if (href === '/') return path === '/';
	if (href === '/group_messages') return path.startsWith('/group_messages') || path.startsWith('/messages/group');
	if (href === '/private_messages') return path.startsWith('/private_messages') || path.startsWith('/messages/private');
	if (href === '/settings') return path === '/settings' || path === '/settings/sessions';
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
	const v = useViewer();
	const [open, setOpen] = useState(false);
	// Only the pages this account has; a group with none left disappears.
	const nav = NAV.map((g) => ({ ...g, items: g.items.filter((it) => allowed(v, it[3])) })).filter((g) => g.items.length);
	// Collapsed sidebar (icons only). The real app would keep this in a
	// cookie so the server renders the right width with no flash.
	const [collapsed, setCollapsed] = useState(() => {
		try { return localStorage.getItem('tgd-side') === 'collapsed'; } catch { return false; }
	});
	const toggle = () => setCollapsed((c) => {
		try { localStorage.setItem('tgd-side', c ? 'open' : 'collapsed'); } catch { /* private mode */ }
		return !c;
	});
	return (
		<div className="app">
			<div className={`a-shell ${open ? 'nav-open' : ''} ${flush ? 'a-flush' : ''} ${collapsed ? 'a-collapsed' : ''}`}>
				<aside className="a-side" onClick={(e) => e.target.closest('a') && setOpen(false)}>
					<Link to="/" className="a-brand">
						<span className="a-logo">tg</span>
						<span className="a-label">tgloggerd</span>
					</Link>
					<nav aria-label="Main">
						{nav.map((g, i) => (
							<div key={i} className="a-navgroup">
								{g.label && <div className="a-navlabel"><span className="a-label">{g.label}</span></div>}
								{g.items.map(([href, label, Icon, , n]) => (
									<Link key={label} to={href} className={`a-navitem ${isOn(path, href) ? 'on' : ''}`} aria-current={isOn(path, href) ? 'page' : undefined} title={collapsed ? label : undefined}>
										<Icon size={16} strokeWidth={1.8} />
										<span className="a-label">{label}</span>
										{n !== undefined && v.role !== 'member' && <span className="a-navcount a-label">{compact(n)}</span>}
									</Link>
								))}
							</div>
						))}
					</nav>
					<div className="a-me">
						<span className="a-me-av" style={{ background: `hsl(${v.acct.hue} 55% 52%)` }}>{v.acct.username[0].toUpperCase()}</span>
						<span className="a-grow a-label a-me-name"><span>{v.acct.username}</span><span className="a-muted sm">{v.role}</span></span>
						<ThemeButton />
						<Link to="/login" className="a-icon-btn" title="Sign out" aria-label="Sign out"><LogOut size={15} /></Link>
					</div>
				</aside>
				<div className="a-scrim" onClick={() => setOpen(false)} />
				<div className="a-main">
					<header className="a-top">
						<button type="button" className="a-icon-btn a-menu" onClick={() => setOpen(true)} aria-label="Open menu"><Menu size={18} /></button>
						<button type="button" className="a-icon-btn a-collapse" onClick={toggle} aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'} title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'} aria-pressed={collapsed}>
							{collapsed ? <PanelLeftOpen size={18} /> : <PanelLeftClose size={18} />}
						</button>
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
