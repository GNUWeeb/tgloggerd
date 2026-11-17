import { useEffect, useState } from 'react';
import { LogIn, TriangleAlert } from 'lucide-react';
import { ThemeButton } from '../ui/Shell.tsx';
import { login, loginToken } from '../api/auth.ts';
import type { PageData } from '../page.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

export type LoginProps = S['LoginProps'];

// Only same-site paths are followed after signing in.
function safeNext(n: string | null): string {
	return n && n.startsWith('/') && !n.startsWith('//') ? n : '/';
}

const MESSAGES: Record<string, string> = {
	invalid_credentials: 'Wrong username or password.',
	rate_limited: 'Too many failed attempts. Wait a while before trying again.',
	csrf: 'The sign-in form expired. Try again.',
};

function Login({ page }: { page: PageData<LoginProps> }) {
	const next = safeNext(page.props.next);
	const [csrf, setCsrf] = useState('');
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState('');

	useEffect(() => {
		void loginToken().then((r) => {
			if (r.signed_in) location.assign(next);
			else setCsrf(r.csrf ?? '');
		}).catch(() => setErr('The server could not be reached.'));
	}, [next]);

	const submit = async (e: React.FormEvent<HTMLFormElement>) => {
		e.preventDefault();
		const f = new FormData(e.currentTarget);
		setBusy(true);
		setErr('');
		const r = await login(csrf, String(f.get('username') ?? ''), String(f.get('password') ?? '')).catch(() => null);
		setBusy(false);
		if (!r) { setErr('The server could not be reached.'); return; }
		// A full load: the new session changes the whole viewer.
		if (r.ok) { location.assign(next); return; }
		if (r.csrf) setCsrf(r.csrf);
		setErr(r.message || MESSAGES[r.error] || 'Signing in failed.');
	};

	return (
		<div className="app a-loginpage">
			<form className="a-card a-login" onSubmit={submit}>
				<div className="a-brand a-login-brand"><span className="a-logo">tg</span><span>tgloggerd</span></div>
				<h1>Sign in</h1>
				<p className="a-muted">Telegram archive. Sign in to browse it.</p>
				{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
				<label className="a-field"><span className="a-field-label">Username</span><input name="username" autoComplete="username" autoFocus required /></label>
				<label className="a-field"><span className="a-field-label">Password</span><input name="password" type="password" autoComplete="current-password" required /></label>
				<button className="a-btn a-btn-primary a-wide" disabled={busy || !csrf}><LogIn size={15} />{busy ? 'Signing in...' : 'Sign in'}</button>
				<div className="a-login-foot"><ThemeButton viewer={page.viewer} /></div>
			</form>
		</div>
	);
}

export const loginRoute: RouteDef<LoginProps> = { Page: Login, head: () => ({ title: 'Sign in - tgloggerd', robots: 'noindex' }) };
