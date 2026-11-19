// The page frame: sidebar (only the pages the viewer has, pinned groups
// first), breadcrumbs and the content area. Menu entries come from the
// viewer's `nav` page keys; hidden pages are absent, never disabled.
import { useCallback, useEffect, useState, type ReactNode } from 'react';
import {
	LayoutDashboard, Users, UsersRound, MessagesSquare, MessageCircle, FileText,
	ArrowLeftRight, Bot, Settings, Search, Menu, LogOut, Sun, Moon, Monitor,
	PanelLeftClose, PanelLeftOpen, UserCog, LogIn, Network, RefreshCw, X,
	BotMessageSquare,
} from 'lucide-react';
import { Link, useNav } from '../nav.tsx';
import { Avatar } from './primitives.tsx';
import { QuickJump, useQuickJumpKey } from './QuickJump.tsx';
import { applyTheme, savePrefs, storeTheme, storedTheme, type Theme } from '../api/prefs.ts';
import type { Viewer } from '../page.ts';
import { logout } from '../api/auth.ts';
import { useLiveState } from '../live/react.tsx';
import { useLiveNotice } from '../live/PageLive.tsx';

type Item = [href: string, label: string, icon: typeof Users, key: string | null];
const NAV: { label: string | null; items: Item[] }[] = [
	{ label: null, items: [['/', 'Dashboard', LayoutDashboard, 'dashboard']] },
	{ label: 'Archive', items: [
		['/users', 'Users', Users, 'users'],
		['/groups', 'Groups', UsersRound, 'groups'],
		['/group_messages', 'Group messages', MessagesSquare, 'group_messages'],
		['/private_messages', 'Private messages', MessageCircle, 'private_messages'],
		['/files', 'Files', FileText, 'files'],
	] },
	{ label: 'Integrations', items: [
		['/platform-fwd', 'Platform forwarding', ArrowLeftRight, 'platform_fwd'],
		['/platform-fwd/bots', 'Bots', BotMessageSquare, 'platform_fwd'],
		['/join-chat', 'Join chat', LogIn, 'join_chat'],
		['/mcp-admin', 'MCP', Bot, 'mcp'],
	] },
	{ label: 'Administration', items: [
		['/settings/accounts', 'Accounts', UserCog, 'accounts'],
		['/settings/federations', 'Federations', Network, 'federations'],
	] },
	{ label: null, items: [['/settings', 'Settings', Settings, null]] },
];

// Before the server sends `nav` (per-account settings do not exist yet),
// a signed-in viewer gets the archive pages.
const FALLBACK_NAV = ['dashboard', 'users', 'groups', 'group_messages', 'files'];

function isOn(path: string, href: string): boolean {
	if (href === '/') return path === '/';
	if (href === '/group_messages') return path.startsWith('/group_messages') || path.startsWith('/messages/group');
	if (href === '/private_messages') return path.startsWith('/private_messages') || path.startsWith('/messages/private');
	// The Bots page has its own entry under Platform forwarding.
	if (href === '/platform-fwd') return path.startsWith('/platform-fwd') && !path.startsWith('/platform-fwd/bots');
	if (href === '/settings') return ['/settings', '/settings/preferences', '/settings/sessions'].includes(path);
	return path === href || path.startsWith(href + '/');
}

const THEMES = ['system', 'light', 'dark'] as const;

// System, light, dark in turn. The choice is kept: as the account's
// preference when signed in, else in this browser (the public pages).
export function ThemeButton({ viewer }: { viewer: Viewer }) {
	const [mode, setMode] = useState<Theme>(viewer.theme || 'system');
	// Signed out, the browser's own choice; after hydration, so the
	// server's page and the first render agree.
	useEffect(() => {
		if (viewer.signed_in) return;
		const t = storedTheme();
		if (t) { setMode(t); applyTheme(t); }
	}, [viewer.signed_in]);
	const next = () => {
		const m = THEMES[(THEMES.indexOf(mode) + 1) % THEMES.length] ?? 'system';
		setMode(m);
		applyTheme(m);
		if (viewer.signed_in) void savePrefs({ theme: m }).catch(() => { /* this page keeps it; the next one shows the saved one */ });
		else storeTheme(m);
	};
	const I = mode === 'light' ? Sun : mode === 'dark' ? Moon : Monitor;
	return <button type="button" className="a-icon-btn" onClick={next} title={`Theme: ${mode}`} aria-label={`Theme: ${mode}`}><I size={16} /></button>;
}

const LIVE_LABEL = {
	connecting: ['Connecting', 'Connecting to live updates'],
	live: ['Live', 'Pages update as messages are logged'],
	reconnecting: ['Reconnecting', 'Live updates interrupted; reconnecting'],
	polling: ['Polling', 'Live updates unavailable; refreshing periodically'],
} as const;

// The state of the live stream, in the top bar; nothing when this page
// listens to nothing.
function LiveDot() {
	const st = useLiveState();
	if (st === 'idle') return null;
	const [label, title] = LIVE_LABEL[st];
	return <span className={`a-livestate is-${st}`} role="status" title={title}><span className="a-livestate-dot" /><span className="a-livestate-label">{label}</span></span>;
}

function LiveNoticeBar() {
	const n = useLiveNotice();
	if (!n.shown) return null;
	return (
		<div className="a-livenotice" role="status">
			<RefreshCw size={14} /><span>New activity since this page loaded.</span>
			<button type="button" className="a-link" onClick={n.apply}>Refresh</button>
			<button type="button" className="a-icon-btn" aria-label="Dismiss" onClick={n.dismiss}><X size={14} /></button>
		</div>
	);
}

export default function Shell({ crumbs = [], flush = false, children }: { crumbs?: [string, string | null][]; flush?: boolean; children: ReactNode }) {
	const { page, viewer } = useNav();
	const path = new URL(page.url, 'http://x').pathname;
	const [open, setOpen] = useState(false);
	const [collapsed, setCollapsed] = useState(viewer.sidebar === 'collapsed');
	const keys = new Set(viewer.nav ?? FALLBACK_NAV);
	const groups = NAV.map((g) => ({ ...g, items: g.items.filter((it) => it[3] === null || keys.has(it[3])) })).filter((g) => g.items.length);
	const acct = viewer.account;
	const [jump, setJump] = useState(false);
	useQuickJumpKey(useCallback(() => setJump((x) => !x), []));
	const jumpPages = groups.flatMap((g) => g.items.map(([href, label, icon]) => ({ href, label, icon })));
	return (
		<div className="app">
			<div className={`a-shell ${open ? 'nav-open' : ''} ${flush ? 'a-flush' : ''} ${collapsed ? 'a-collapsed' : ''}`}>
				<aside className="a-side" onClick={(e) => (e.target as HTMLElement).closest('a') && setOpen(false)}>
					<Link to="/" className="a-brand"><span className="a-logo">tg</span><span className="a-label">tgloggerd</span></Link>
					<nav aria-label="Main">
						{!!viewer.pinned?.length && (
							<div className="a-navgroup">
								<div className="a-navlabel"><span className="a-label">Pinned</span></div>
								{viewer.pinned.map((g) => (
									<Link key={g.id} to={`/groups/${g.id}/chat`} className={`a-navitem ${path.startsWith(`/groups/${g.id}`) ? 'on' : ''}`} title={collapsed ? g.title : undefined}>
										<Avatar id={g.id} name={g.title} photo={g.photo} size={16} square />
										<span className="a-label a-ellipsis">{g.title}</span>
									</Link>
								))}
							</div>
						)}
						{groups.map((g, i) => (
							<div key={i} className="a-navgroup">
								{g.label && <div className="a-navlabel"><span className="a-label">{g.label}</span></div>}
								{g.items.map(([href, label, I]) => (
									<Link key={href} to={href} className={`a-navitem ${isOn(path, href) ? 'on' : ''}`} aria-current={isOn(path, href) ? 'page' : undefined} title={collapsed ? label : undefined}>
										<I size={16} />
										<span className="a-label">{label}</span>
									</Link>
								))}
							</div>
						))}
					</nav>
					{acct && (
						<div className="a-me">
							<Avatar id={acct.id} name={acct.name} size={24} />
							<span className="a-grow a-label a-me-name"><span>{acct.username}</span><span className="a-muted sm">{acct.role}</span></span>
							<ThemeButton viewer={viewer} />
							<button type="button" className="a-icon-btn" title="Sign out" aria-label="Sign out" onClick={() => void logout().finally(() => location.assign('/login'))}><LogOut size={15} /></button>
						</div>
					)}
				</aside>
				<div className="a-scrim" onClick={() => setOpen(false)} />
				<div className="a-main">
					<header className="a-top">
						<button type="button" className="a-icon-btn a-menu" onClick={() => setOpen(true)} aria-label="Open menu"><Menu size={18} /></button>
						<button type="button" className="a-icon-btn a-collapse" onClick={() => {
							const c = !collapsed;
							setCollapsed(c);
							// The account keeps it, for every page and device.
							if (viewer.signed_in) void savePrefs({ sidebar: c ? 'collapsed' : 'open' }).catch(() => {});
						}} aria-pressed={collapsed} aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}>
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
						<LiveDot />
						<button type="button" className="a-cmdk" aria-label="Search everything" aria-keyshortcuts="Control+K Meta+K" onClick={() => setJump(true)}><Search size={14} /><span>Search everything...</span><kbd>Ctrl K</kbd></button>
					</header>
					<LiveNoticeBar />
					<main className="a-content">{children}</main>
					{jump && <QuickJump pages={jumpPages} onClose={() => setJump(false)} />}
				</div>
			</div>
		</div>
	);
}
