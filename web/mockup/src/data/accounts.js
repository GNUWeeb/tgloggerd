// Web accounts with basic permissions and access scopes. Synthetic.
import { groups } from './groups.js';
import { users } from './users.js';
import { NOW, DAY, iso } from './seed.js';

export const PERMISSIONS = [
	['view_files', 'View files and media', 'Open and download photos, videos and documents the archive stored.'],
	['fetch_files', 'Fetch files from Telegram', 'Download media the archive did not store, on demand.'],
	['view_private', 'Read private messages', 'Private chats with the logger account.'],
	['manage_integrations', 'Manage integrations', 'Platform forwarding and MCP.'],
	['manage_accounts', 'Manage accounts', 'Create, edit and disable accounts, this page included.'],
];

export const PAGES = [
	['dashboard', 'Dashboard'], ['users', 'Users'], ['groups', 'Groups'],
	['group_messages', 'Group messages'], ['private_messages', 'Private messages'],
	['files', 'Files'], ['platform_fwd', 'Platform forwarding'], ['mcp', 'MCP'],
];

const ALL = PERMISSIONS.map((p) => p[0]);

// scope: null means everything; otherwise a list.
export const accounts = [
	{ id: 1, username: 'admin', name: 'Administrator', role: 'admin', perms: ALL, pages: null, groups: null, users: null, active: true, last: iso(NOW - 120000), created: iso(NOW - 480 * DAY), hue: 250 },
	{ id: 2, username: 'ops', name: 'Operations', role: 'admin', perms: ALL, pages: null, groups: null, users: null, active: true, last: iso(NOW - 2 * DAY), created: iso(NOW - 470 * DAY), hue: 20 },
	{ id: 3, username: 'researcher', name: 'Research team', role: 'member', perms: ['view_files'], pages: ['dashboard', 'users', 'groups', 'group_messages', 'files'], groups: [groups[0], groups[2]], users: null, active: true, last: iso(NOW - 5 * 3600000), created: iso(NOW - 60 * DAY), hue: 160 },
	{ id: 4, username: 'mod_gnuweeb', name: 'GNU/Weeb moderators', role: 'member', perms: ['view_files', 'fetch_files'], pages: ['users', 'groups', 'group_messages', 'files'], groups: [groups[0]], users: null, active: true, last: iso(NOW - 26 * 3600000), created: iso(NOW - 30 * DAY), hue: 300 },
	{ id: 5, username: 'case_review', name: 'Case review', role: 'member', perms: ['view_files', 'view_private'], pages: ['users', 'private_messages', 'files'], groups: [], users: [users[3], users[5], users[8]], active: true, last: '', created: iso(NOW - 3 * DAY), hue: 200 },
	{ id: 6, username: 'intern2025', name: 'Intern (2025)', role: 'member', perms: [], pages: ['dashboard', 'groups'], groups: [groups[1]], users: null, active: false, last: iso(NOW - 200 * DAY), created: iso(NOW - 400 * DAY), hue: 90 },
];

export function scopeSummary(a) {
	if (a.role === 'admin') return 'Everything';
	const parts = [];
	parts.push(a.pages ? `${a.pages.length} of ${PAGES.length} pages` : 'all pages');
	parts.push(a.groups === null ? 'all groups' : a.groups.length ? `${a.groups.length} group${a.groups.length > 1 ? 's' : ''}` : 'no groups');
	if (a.users !== null) parts.push(`${a.users.length} users`);
	return parts.join(', ');
}
