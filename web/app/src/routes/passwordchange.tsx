// The step an account that signed in with a temporary password must
// take first (the spec's Gates): until it chooses its own password the
// server answers every page with this route, at the URL asked for
// (`next`), and other calls with 403 password_change_required. Once
// changed, fetching the page data again lands on that page.
import { useState, type FormEvent } from 'react';
import { KeyRound, LogOut } from 'lucide-react';
import { useNav } from '../nav.tsx';
import { logout } from '../api/auth.ts';
import { ApiError, api, ok, setSessionCsrf } from '../api/client.ts';
import { AuthCard, Alert } from './password.tsx';
import type { PageData } from '../page.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

export type PasswordChangeProps = S['PasswordChangeProps'];

const MESSAGES: Record<string, string> = {
	wrong_password: 'The temporary password is not right.',
	weak_password: 'Choose a longer password: at least 8 characters.',
	rate_limited: 'Too many tries. Wait a while before trying again.',
};

function PasswordChange({ page }: { page: PageData<PasswordChangeProps> }) {
	const { refresh } = useNav();
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState('');
	const submit = async (e: FormEvent<HTMLFormElement>) => {
		e.preventDefault();
		const f = new FormData(e.currentTarget);
		const pw = String(f.get('new_password') ?? '');
		if (pw !== f.get('confirm_password')) { setErr('The new passwords do not match.'); return; }
		if (pw === f.get('current_password')) { setErr('Choose a password of your own, not the temporary one.'); return; }
		setBusy(true); setErr('');
		try {
			const r = await ok(api.POST('/api/account/password', { body: { current_password: String(f.get('current_password') ?? ''), new_password: pw } }));
			// The session moved to a new cookie; the old token no longer works.
			setSessionCsrf(r.csrf);
			await refresh();
		} catch (x) {
			setErr(x instanceof ApiError ? MESSAGES[x.code] ?? x.message : 'The server could not be reached.');
			setBusy(false);
		}
	};
	return (
		<AuthCard page={page} title="Choose your own password" sub={<>You signed in as <b>{page.props.username}</b> with a temporary password. Choose your own before you go on; the temporary one stops working.</>} onSubmit={(e) => void submit(e)}>
			{err && <Alert text={err} />}
			<input type="text" name="username" autoComplete="username" value={page.props.username} readOnly hidden />
			<label className="a-field"><span className="a-field-label">Temporary password</span><input name="current_password" type="password" autoComplete="current-password" autoFocus required /></label>
			<label className="a-field"><span className="a-field-label">New password</span><input name="new_password" type="password" autoComplete="new-password" minLength={8} required /><span className="a-field-hint">At least 8 characters. A long passphrase is best.</span></label>
			<label className="a-field"><span className="a-field-label">New password again</span><input name="confirm_password" type="password" autoComplete="new-password" minLength={8} required /></label>
			<button className="a-btn a-btn-primary a-wide" disabled={busy}><KeyRound size={15} />{busy ? 'Saving...' : 'Set my password'}</button>
			<p className="a-login-alt"><button type="button" className="a-link" onClick={() => void logout().finally(() => location.assign('/login'))}><LogOut size={13} /> Sign out</button></p>
		</AuthCard>
	);
}

export const passwordChangeRoute: RouteDef<PasswordChangeProps> = { Page: PasswordChange, head: () => ({ title: 'Choose your own password - tgloggerd', robots: 'noindex' }) };
