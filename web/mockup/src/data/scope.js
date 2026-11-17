// What a sample account may see and do: the request scope of
// docs/design/web-accounts.md section 8, as the mockup and the app's
// stand-in fixtures model it. Plain data and functions, no React, so
// the stand-in can import it with nothing but the app's own packages.
import { federations, DEFAULT_FED } from './accounts.js';
import { getGroupsOfUser } from './groups.js';

export function buildScope(acct) {
	const role = acct.role;
	const all = () => true;
	if (role === 'superadmin') {
		return {
			acct, role,
			page: all, group: all, user: all, fullUser: true,
			canFetch: true, canForward: true, canJoin: true, canLookupName: true,
			managesAccounts: true, managesFeds: true,
			manageableFeds: [...federations, DEFAULT_FED],
		};
	}
	if (role === 'admin') {
		const hidden = new Set(acct.restrict.hide_groups);
		const hidePm = acct.restrict.hide_private;
		return {
			acct, role,
			page: (k) => !(hidePm && k === 'private_messages'),
			group: (g) => !hidden.has(g.id),
			user: all, fullUser: true,
			canFetch: true, canForward: true, canJoin: acct.restrict.join_chat, canLookupName: false,
			managesAccounts: true, managesFeds: false,
			manageableFeds: acct.adminOf.map((id) => [...federations, DEFAULT_FED].find((f) => f.id === id)),
		};
	}
	const granted = new Set(acct.grants.flatMap((g) => g.groupIds));
	const pages = new Set(['dashboard', 'users', 'groups', 'group_messages', 'files', 'settings']);
	if (acct.abilities.includes('forward')) pages.add('platform_fwd');
	return {
		acct, role,
		page: (k) => pages.has(k),
		group: (g) => granted.has(g.id),
		// A member sees people only as they appear in its groups.
		user: (u) => getGroupsOfUser(u).some((g) => granted.has(g.id)),
		fullUser: false,
		canFetch: acct.abilities.includes('fetch_files'),
		canForward: acct.abilities.includes('forward'),
		canJoin: false, canLookupName: false,
		managesAccounts: false, managesFeds: false,
		manageableFeds: [],
	};
}

// Whether a page (by key) exists for this viewer. Pages it may not use
// are absent from the menu and answer 404, never shown disabled.
export function allowed(v, key) {
	if (!key) return true;
	if (key === 'accounts') return v.managesAccounts;
	if (key === 'federations') return v.role !== 'member';
	if (key === 'join_chat') return v.canJoin;
	return v.page(key);
}
