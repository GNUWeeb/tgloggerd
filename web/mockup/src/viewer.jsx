// The signed-in account and what it may see (the request scope of
// docs/design/web-accounts.md section 8). In the real app the server
// builds this and sends it as `viewer`; here a mockup-only switcher
// picks one of the sample accounts so each role can be previewed.
import { createContext, useContext, useState } from 'react';
import { useRouter } from './router.jsx';
import { accounts, accountById, fedGroups } from './data/accounts.js';
import { groups } from './data/groups.js';
import { buildScope, allowed } from './data/scope.js';

// The scope itself is plain data (data/scope.js), so the app's stand-in
// can use it without React.
export { buildScope, allowed };

const Ctx = createContext(null);

export const PERSONAS = [
	[1, 'Superadmin'],
	[3, 'Admin (restricted)'],
	[5, 'Member'],
];

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

// Mockup-only control: preview the UI as each kind of account.
const SIGNED_OUT = ['/login', '/forgot-password', '/reset-password', '/set-password', '/verify-email', '/welcome/email'];

export function PersonaSwitcher() {
	const v = useViewer();
	const { path } = useRouter();
	if (SIGNED_OUT.includes(path)) return null;
	return (
		<div className="persona" role="group" aria-label="Mockup: view as">
			<span className="persona-l">View as</span>
			{PERSONAS.map(([id, label]) => (
				<button key={id} type="button" className={v.acct.id === id ? 'on' : ''} onClick={() => v.choose(id)} aria-pressed={v.acct.id === id}>{label}</button>
			))}
		</div>
	);
}
