// Settings and the sign-in page.
import { useState } from 'react';
import { CircleCheck, LogIn, ShieldCheck } from 'lucide-react';
import { useRouter } from '../router.jsx';
import Shell, { ThemeButton } from '../ui/Shell.jsx';
import { PageHead, Btn, Card } from '../ui/primitives.jsx';
import { Field } from '../ui/forms.jsx';

export function Settings() {
	const [done, setDone] = useState(false);
	return (
		<Shell crumbs={[['Settings', null]]}>
			<PageHead title="Settings" />
			<div className="a-settings">
				<nav className="a-settingsnav" aria-label="Settings sections">
					<a href="#account" className="on">Account</a>
					<a href="#appearance">Appearance</a>
				</nav>
				<div className="a-stack">
					<Card title="Account" className="a-narrow" >
						<p className="a-muted" id="account">Signed in as <b>admin</b> . administrator</p>
					</Card>
					<Card title="Change password" className="a-narrow">
						{done && <div className="a-banner"><CircleCheck size={15} /><span>Password changed. Every other session has been signed out.</span></div>}
						<form className="a-form" onSubmit={(e) => { e.preventDefault(); setDone(true); }}>
							<Field label="Current password"><input type="password" autoComplete="current-password" required /></Field>
							<Field label="New password" hint="At least 8 characters."><input type="password" autoComplete="new-password" minLength={8} required /></Field>
							<Field label="Confirm new password"><input type="password" autoComplete="new-password" minLength={8} required /></Field>
							<p className="a-muted sm"><ShieldCheck size={13} /> Changing the password signs out every other browser and device.</p>
							<div><button className="a-btn a-btn-primary">Change password</button></div>
						</form>
					</Card>
					<Card title="Appearance" className="a-narrow">
						<div className="a-row" id="appearance"><span className="a-grow">Theme: follows the system until changed</span><ThemeButton /></div>
					</Card>
				</div>
			</div>
		</Shell>
	);
}

export function Login() {
	const { navigate } = useRouter();
	const [err, setErr] = useState(false);
	return (
		<div className="app a-loginpage">
			<form className="a-card a-login" onSubmit={(e) => { e.preventDefault(); if (e.target.user.value) navigate('/'); else setErr(true); }}>
				<div className="a-brand a-login-brand"><span className="a-logo">tg</span><span>tgloggerd</span></div>
				<h1>Sign in</h1>
				<p className="a-muted">Telegram archive. Sign in to browse it.</p>
				{err && <div className="a-banner warn"><span>Wrong username or password.</span></div>}
				<Field label="Username"><input name="user" autoComplete="username" autoFocus /></Field>
				<Field label="Password"><input name="pass" type="password" autoComplete="current-password" /></Field>
				<button className="a-btn a-btn-primary a-wide"><LogIn size={15} />Sign in</button>
				<div className="a-login-foot"><ThemeButton /></div>
			</form>
		</div>
	);
}
