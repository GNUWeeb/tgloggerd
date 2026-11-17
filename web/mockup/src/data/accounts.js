// Web accounts: roles, federations, admin restrictions, member grants.
// Synthetic. Follows docs/design/web-accounts.md.
import { groups } from './groups.js';
import { NOW, DAY, iso } from './seed.js';

// Federations: named sets of groups. "Default" is derived: every group
// in no other federation, including groups joined later.
export const federations = [
	{ id: 1, name: 'GNU/Weeb network', groupIds: [groups[0].id, groups[3].id, groups[4].id, groups[10].id], created: iso(NOW - 300 * DAY) },
	{ id: 2, name: 'Rust and systems', groupIds: [groups[2].id, groups[3].id, groups[11].id], created: iso(NOW - 200 * DAY) },
	{ id: 3, name: 'Security', groupIds: [groups[9].id, groups[5].id], created: iso(NOW - 90 * DAY) },
];
export const DEFAULT_FED = { id: 0, name: 'Default', derived: true };

export function fedGroups(fed) {
	if (fed.id === 0) {
		const taken = new Set(federations.flatMap((f) => f.groupIds));
		return groups.filter((g) => !taken.has(g.id));
	}
	return fed.groupIds.map((id) => groups.find((g) => g.id === id));
}

export function allFeds() {
	return [...federations, DEFAULT_FED];
}

export function fedById(id) {
	return allFeds().find((f) => f.id === id);
}

export const ROLES = [
	['superadmin', 'Superadmin', 'Everything: all accounts, roles, federations and settings.'],
	['admin', 'Admin', 'Everything except what a superadmin restricts. Manages members of its federations only.'],
	['member', 'Member', 'Only the groups granted in its federations, and the abilities given.'],
];

// Restrictions a superadmin sets per admin. A list, so more can follow.
export const RESTRICTIONS = [
	['hide_private', 'Hide private messages', 'No private-message menu, pages or API; their URLs answer 404.'],
	['hide_groups', 'Hide groups', 'The chosen groups, their messages, members, files, forwarding and MCP exposure are invisible.'],
	['join_chat', 'Allow Join chat', 'Make the logging account join groups, and add them to its federations.'],
];

export const ABILITIES = [
	['forward', 'Set up forwarding', 'Own Telegram and Discord forwarding for its granted groups.'],
	['fetch_files', 'Fetch files from Telegram', 'Download media the archive did not store.'],
];

export const accounts = [
	{ id: 1, username: 'admin', name: 'Administrator', email: 'admin@example.org', verified: true, role: 'superadmin', active: true, last: iso(NOW - 120000), created: iso(NOW - 480 * DAY), hue: 250 },
	{ id: 2, username: 'ops', name: 'Operations', email: '', verified: false, role: 'superadmin', active: true, last: iso(NOW - 2 * DAY), created: iso(NOW - 470 * DAY), hue: 20 },
	{ id: 3, username: 'gw_admin', name: 'GNU/Weeb admins', email: 'gw-admins@example.org', verified: true, role: 'admin', active: true, last: iso(NOW - 3 * 3600000), created: iso(NOW - 200 * DAY), hue: 300,
		adminOf: [1, 0], restrict: { hide_private: true, hide_groups: [groups[9].id], join_chat: true } },
	{ id: 4, username: 'sec_lead', name: 'Security lead', email: 'sec.lead@example.net', verified: true, role: 'admin', active: true, last: iso(NOW - 26 * 3600000), created: iso(NOW - 90 * DAY), hue: 0,
		adminOf: [3], restrict: { hide_private: false, hide_groups: [], join_chat: false } },
	{ id: 5, username: 'researcher', name: 'Research team', email: 'research@example.com', verified: true, role: 'member', active: true, last: iso(NOW - 5 * 3600000), created: iso(NOW - 60 * DAY), hue: 160,
		grants: [{ fed: 1, groupIds: [groups[0].id, groups[4].id] }, { fed: 2, groupIds: [groups[2].id] }], abilities: ['forward', 'fetch_files'] },
	{ id: 6, username: 'mod_kernel', name: 'Kernel moderators', email: 'kernel-mods@example.com', verified: false, invited: true, role: 'member', active: true, last: '', created: iso(NOW - 3 * DAY), hue: 200,
		grants: [{ fed: 1, groupIds: [groups[3].id] }], abilities: [] },
	{ id: 7, username: 'intern2025', name: 'Intern (2025)', email: 'intern@example.com', verified: true, role: 'member', active: false, last: iso(NOW - 200 * DAY), created: iso(NOW - 400 * DAY), hue: 90,
		grants: [{ fed: 0, groupIds: [] }], abilities: [] },
];

export function accountById(id) {
	return accounts.find((a) => a.id === id);
}

export const activeSuperadmins = () => accounts.filter((a) => a.role === 'superadmin' && a.active);

// Who administers a federation (for the federations page).
export function fedAdmins(fed) {
	return accounts.filter((a) => a.role === 'admin' && a.adminOf.includes(fed.id));
}
export function fedMembers(fed) {
	return accounts.filter((a) => a.role === 'member' && a.grants.some((g) => g.fed === fed.id));
}

// Audit trail per account (account changes made by or to it).
export function auditFor(a) {
	const rows = [
		['created', 'admin', a.created],
	];
	if (a.role === 'admin') rows.push(['restrictions changed: hide private messages', 'admin', iso(NOW - 40 * DAY)]);
	if (a.role === 'member') rows.push(['granted 2 groups in GNU/Weeb network', 'gw_admin', iso(NOW - 20 * DAY)]);
	if (!a.active) rows.push(['disabled', 'admin', iso(NOW - 150 * DAY)]);
	rows.push(['signed out everywhere', a.username, iso(NOW - 10 * DAY)]);
	return rows.sort((x, y) => (x[2] < y[2] ? 1 : -1)).map(([what, by, at]) => ({ what, by, at }));
}

// Sessions of the signed-in account ("my sessions").
export const mySessions = [
	{ id: 's1', device: 'Firefox on Linux', ip: '203.0.113.24', place: 'Jakarta, ID', started: iso(NOW - 3 * DAY), last: iso(NOW - 60000), current: true },
	{ id: 's2', device: 'Chrome on Android', ip: '198.51.100.7', place: 'Bandung, ID', started: iso(NOW - 12 * DAY), last: iso(NOW - 5 * 3600000) },
	{ id: 's3', device: 'Safari on macOS', ip: '192.0.2.81', place: 'Singapore, SG', started: iso(NOW - 40 * DAY), last: iso(NOW - 9 * DAY) },
];

// Security notices, always sent; no per-notice switches for now.
export const SECURITY_NOTICES = [
	'Your password was changed or reset',
	'Your email address was changed (sent to the old and the new address)',
	'Someone signed in from a new device or browser',
	'Your account was disabled',
	'Your role was changed',
];
