// The signed-in account and what it may see (the request scope of
// docs/design/web-accounts.md section 8). In the real app the server
// builds this and sends it as `viewer`; here a mockup-only switcher
// picks one of the sample accounts so each role can be previewed.
import { createContext, useContext, useState } from 'react';
import { accounts, accountById, federations, fedGroups, DEFAULT_FED } from './data/accounts.js';
import { groups, getGroupsOfUser } from './data/groups.js';

const Ctx = createContext(null);

export const PERSONAS = [
	[1, 'Superadmin'],
	[3, 'Admin (restricted)'],
	[5, 'Member'],
];

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

export function ViewerProvider({ children }) {
	const [id, setId] = useState(() => {
		try { return Number(localStorage.getItem('tgd-as')) || 1; } catch { return 1; }
	});
	const acct = accountById(id) || accounts[0];
	const choose = (n) => {
		try { localStorage.setItem('tgd-as', String(n)); } catch { /* private mode */ }
		setId(n);
	};
	return <Ctx.Provider value={{ ...buildScope(acct), choose }}>{children}</Ctx.Provider>;
}

export function useViewer() {
	return useContext(Ctx);
}

// Groups this viewer can see, for pickers and lists.
export function visibleGroups(v, list = groups) {
	return list.filter((g) => v.group(g));
}

export { fedGroups };

// Whether a page (by key) exists for this viewer. Pages it may not use
// are absent from the menu and answer 404, never shown disabled.
export function allowed(v, key) {
	if (!key) return true;
	if (key === 'accounts') return v.managesAccounts;
	if (key === 'federations') return v.role !== 'member';
	if (key === 'join_chat') return v.canJoin;
	return v.page(key);
}

// Mockup-only control: preview the UI as each kind of account.
export function PersonaSwitcher() {
	const v = useViewer();
	return (
		<div className="persona" role="group" aria-label="Mockup: view as">
			<span className="persona-l">View as</span>
			{PERSONAS.map(([id, label]) => (
				<button key={id} type="button" className={v.acct.id === id ? 'on' : ''} onClick={() => v.choose(id)} aria-pressed={v.acct.id === id}>{label}</button>
			))}
		</div>
	);
}
