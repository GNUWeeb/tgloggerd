// The step an account without a verified email address must take
// before anything else (web-accounts.md section 8). The server answers
// every page with this route until the address is verified; `next` is
// the page that was asked for, which is also this page's URL, so once
// the link is followed, fetching the page data again lands there.
import { useEffect, useState, type FormEvent } from 'react';
import { LogOut, MailCheck, Send } from 'lucide-react';
import { useNav } from '../nav.tsx';
import { logout } from '../api/auth.ts';
import { ApiError, api, ok } from '../api/client.ts';
import { AuthCard, Alert, Done } from './password.tsx';
import type { PageData } from '../page.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

export type EmailRequiredProps = S['EmailRequiredProps'];

// How often to look whether the link was followed (in this browser or
// another), besides whenever the window gets focus.
export const RECHECK_MS = 15_000;

function EmailRequired({ page }: { page: PageData<EmailRequiredProps> }) {
	const { refresh } = useNav();
	const { pending } = page.props;
	const [editing, setEditing] = useState(!pending);
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState('');
	useEffect(() => { setEditing(!pending); }, [pending]);

	// Once verified, the server answers with the page asked for.
	useEffect(() => {
		if (!pending) return;
		const check = () => { if (!document.hidden) void refresh(); };
		const t = setInterval(check, RECHECK_MS);
		addEventListener('focus', check);
		return () => { clearInterval(t); removeEventListener('focus', check); };
	}, [pending, refresh]);

	const send = async (email: string, password: string) => {
		setBusy(true); setErr('');
		try {
			await ok(api.POST('/api/account/email', { body: { email, password } }));
			await refresh();
			return true;
		} catch (x) {
			setErr(x instanceof ApiError ? x.message : 'The server could not be reached.');
			return false;
		} finally { setBusy(false); }
	};
	const submit = async (e: FormEvent<HTMLFormElement>) => {
		e.preventDefault();
		const f = new FormData(e.currentTarget);
		if (await send(String(f.get('email') ?? '').trim(), String(f.get('password') ?? ''))) setEditing(false);
	};

	const signOut = <button type="button" className="a-link a-login-alt" onClick={() => void logout().finally(() => location.assign('/login'))}><LogOut size={13} /> Sign out</button>;
	if (pending && !editing) {
		return (
			<AuthCard page={page} title="Check your inbox" sub="Your account needs a verified email address for password resets and security notices.">
				<Done>A verification link was sent to <b>{pending}</b>. Open it to continue; this page moves on by itself once the address is verified.</Done>
				{err && <Alert text={err} />}
				<button type="button" className="a-btn a-btn-primary a-wide" onClick={() => void refresh()}><MailCheck size={15} />I have verified it</button>
				<p className="a-login-alt"><button type="button" className="a-link" onClick={() => { setEditing(true); setErr(''); }}>Send a new link, or use another address</button></p>
				{signOut}
			</AuthCard>
		);
	}
	return (
		<AuthCard page={page} title="Add your email address" sub="Your account needs a verified email address before you go on: it is used for password resets and security notices, never shown to others." onSubmit={(e) => void submit(e)}>
			{err && <Alert text={err} />}
			<label className="a-field"><span className="a-field-label">Email address</span><input name="email" type="email" autoComplete="email" defaultValue={pending ?? ''} autoFocus required /></label>
			<label className="a-field"><span className="a-field-label">Your password</span><input name="password" type="password" autoComplete="current-password" required /><span className="a-field-hint">To confirm it is you. A new link replaces any sent before.</span></label>
			<button className="a-btn a-btn-primary a-wide" disabled={busy}><Send size={15} />{busy ? 'Sending...' : 'Send the verification link'}</button>
			{pending && <p className="a-login-alt"><button type="button" className="a-link" onClick={() => setEditing(false)}>Cancel</button></p>}
			{signOut}
		</AuthCard>
	);
}

export const emailRequiredRoute: RouteDef<EmailRequiredProps> = { Page: EmailRequired, head: () => ({ title: 'Add your email address - tgloggerd', robots: 'noindex' }) };
