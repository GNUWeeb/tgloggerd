// The pages email links lead to (web-accounts.md section 8), and the
// forgotten-password form. They need no session: a link may be opened
// in another browser. Link tokens travel after '#', so they reach
// neither the server's nor a proxy's logs nor a Referer header; the page
// reads the token, takes it out of the address bar, and sends it only
// when the reader acts, so a mail scanner that opens the link uses
// nothing up.
import { useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { KeyRound, MailCheck, Send, TriangleAlert, CircleCheck } from 'lucide-react';
import { ThemeButton } from '../ui/Shell.tsx';
import { Link } from '../nav.tsx';
import { loginToken } from '../api/auth.ts';
import { ApiError, api, ok } from '../api/client.ts';
import type { PageData } from '../page.ts';
import type { RouteDef } from './index.tsx';

export type NoProps = Record<string, never>;

// The token from #token=..., removed from the address bar once read.
function useLinkToken(): string | null | undefined {
	const [token, setToken] = useState<string | null | undefined>(undefined); // undefined: not read yet
	// Read once: the address bar no longer has it when an effect runs again.
	const read = useRef<string | null | undefined>(undefined);
	useEffect(() => {
		if (read.current === undefined) {
			read.current = new URLSearchParams(location.hash.slice(1)).get('token') || null;
			if (location.hash) history.replaceState(history.state, '', location.pathname + location.search);
		}
		setToken(read.current);
	}, []);
	return token;
}

const LINK_MESSAGES: Record<string, string> = {
	bad_link: 'This link is not valid any more: it was used already, it expired, or it was replaced by a newer one.',
	weak_password: 'Choose a longer password: at least 8 characters.',
	rate_limited: 'Too many requests. Wait a while before trying again.',
	csrf: 'The form expired. Reload the page and try again.',
};
const errText = (x: unknown) => (x instanceof ApiError ? LINK_MESSAGES[x.code] ?? x.message : 'The server could not be reached.');

function AuthCard({ page, title, sub, children, onSubmit }: { page: PageData; title: string; sub: ReactNode; children: ReactNode; onSubmit?: (e: FormEvent<HTMLFormElement>) => void }) {
	return (
		<div className="app a-loginpage">
			<form className="a-card a-login" onSubmit={onSubmit ?? ((e) => e.preventDefault())}>
				<div className="a-brand a-login-brand"><span className="a-logo">tg</span><span>tgloggerd</span></div>
				<h1>{title}</h1>
				<p className="a-muted">{sub}</p>
				{children}
				<div className="a-login-foot"><ThemeButton viewer={page.viewer} /></div>
			</form>
		</div>
	);
}

const Alert = ({ text }: { text: string }) => <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{text}</span></div>;
const Done = ({ children }: { children: ReactNode }) => <div className="a-banner ok" role="status"><CircleCheck size={15} /><span>{children}</span></div>;
const BadLink = () => <Alert text="This link is incomplete. Open it from the email again, or ask for a new one." />;

function ForgotPassword({ page }: { page: PageData<NoProps> }) {
	const [csrf, setCsrf] = useState('');
	const [signedIn, setSignedIn] = useState(false);
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState('');
	const [sent, setSent] = useState(false);
	useEffect(() => {
		void loginToken().then((r) => { setSignedIn(r.signed_in); setCsrf(r.csrf ?? ''); }).catch(() => setErr('The server could not be reached.'));
	}, []);
	const submit = async (e: FormEvent<HTMLFormElement>) => {
		e.preventDefault();
		const username = String(new FormData(e.currentTarget).get('username') ?? '').trim();
		setBusy(true); setErr('');
		try {
			await ok(api.POST('/api/password/forgot', { body: { username }, headers: { 'x-csrf-token': csrf } }));
			setSent(true);
		} catch (x) { setErr(errText(x)); } finally { setBusy(false); }
	};
	if (signedIn) {
		return <AuthCard page={page} title="Forgot your password?" sub="You are signed in."><p>Change your password in <Link to="/settings" className="a-link">Settings</Link>.</p></AuthCard>;
	}
	return (
		<AuthCard page={page} title="Forgot your password?" sub="Enter your username or email address. If the account has a verified address, a link to choose a new password is sent to it." onSubmit={(e) => void submit(e)}>
			{err && <Alert text={err} />}
			{sent ? (
				<Done>If that account has a verified email address, a link is on its way. It works once and expires; check your spam folder if it does not arrive.</Done>
			) : (
				<>
					<label className="a-field"><span className="a-field-label">Username or email address</span><input name="username" autoComplete="username" autoFocus required /></label>
					<button className="a-btn a-btn-primary a-wide" disabled={busy || !csrf}><Send size={15} />{busy ? 'Sending...' : 'Send the link'}</button>
				</>
			)}
			<p className="a-login-alt"><Link to="/login" className="a-link">Back to sign in</Link></p>
		</AuthCard>
	);
}

function SetPassword({ page }: { page: PageData<NoProps> }) {
	const token = useLinkToken();
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState('');
	const [done, setDone] = useState(false);
	const submit = async (e: FormEvent<HTMLFormElement>) => {
		e.preventDefault();
		const f = new FormData(e.currentTarget);
		const pw = String(f.get('new_password') ?? '');
		if (pw !== f.get('confirm_password')) { setErr('The passwords do not match.'); return; }
		if (pw.length < 8) { setErr(LINK_MESSAGES.weak_password as string); return; }
		setBusy(true); setErr('');
		try {
			await ok(api.POST('/api/password/set', { body: { token: token as string, new_password: pw } }));
			setDone(true);
		} catch (x) { setErr(errText(x)); } finally { setBusy(false); }
	};
	return (
		<AuthCard page={page} title="Choose a password" sub="For your tgloggerd account. Every session the account had is signed out." onSubmit={(e) => void submit(e)}>
			{token === null && <BadLink />}
			{err && <Alert text={err} />}
			{done ? (
				<>
					<Done>Your password is set.</Done>
					<Link to="/login" className="a-btn a-btn-primary a-wide">Sign in</Link>
				</>
			) : token && (
				<>
					<label className="a-field"><span className="a-field-label">New password</span><input name="new_password" type="password" autoComplete="new-password" minLength={8} autoFocus required /><span className="a-field-hint">At least 8 characters. A long passphrase is best.</span></label>
					<label className="a-field"><span className="a-field-label">New password again</span><input name="confirm_password" type="password" autoComplete="new-password" minLength={8} required /></label>
					<button className="a-btn a-btn-primary a-wide" disabled={busy}><KeyRound size={15} />{busy ? 'Saving...' : 'Set password'}</button>
				</>
			)}
		</AuthCard>
	);
}

function VerifyEmail({ page }: { page: PageData<NoProps> }) {
	const token = useLinkToken();
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState('');
	const [done, setDone] = useState(false);
	const verify = async (e: FormEvent<HTMLFormElement>) => {
		e.preventDefault();
		setBusy(true); setErr('');
		try {
			await ok(api.POST('/api/account/email/verify', { body: { token: token as string } }));
			setDone(true);
		} catch (x) { setErr(errText(x)); } finally { setBusy(false); }
	};
	return (
		<AuthCard page={page} title="Verify your email address" sub="Confirm that this address is yours, so it can be used for password resets and security notices." onSubmit={(e) => void verify(e)}>
			{token === null && <BadLink />}
			{err && <Alert text={err} />}
			{done ? (
				<>
					<Done>Your email address is verified.</Done>
					<Link to={page.viewer.signed_in ? '/settings' : '/login'} className="a-btn a-btn-primary a-wide">{page.viewer.signed_in ? 'Back to Settings' : 'Sign in'}</Link>
				</>
			) : token && <button className="a-btn a-btn-primary a-wide" disabled={busy}><MailCheck size={15} />{busy ? 'Verifying...' : 'Verify this address'}</button>}
		</AuthCard>
	);
}

export const passwordForgotRoute: RouteDef<NoProps> = { Page: ForgotPassword, head: () => ({ title: 'Forgot your password - tgloggerd', robots: 'noindex' }) };
export const passwordSetRoute: RouteDef<NoProps> = { Page: SetPassword, head: () => ({ title: 'Choose a password - tgloggerd', robots: 'noindex' }) };
export const emailVerifyRoute: RouteDef<NoProps> = { Page: VerifyEmail, head: () => ({ title: 'Verify your email address - tgloggerd', robots: 'noindex' }) };
