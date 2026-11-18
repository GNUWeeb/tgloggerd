// "Look up real name" (web/docs/contact-names.md): the logging account
// removes a contact, reads the name the user set and adds the contact
// back exactly as it was. A superadmin action, confirmed first.
import { useState } from 'react';
import { CircleCheck, LoaderCircle, ScanSearch, TriangleAlert } from 'lucide-react';
import { Modal } from '../ui/Modal.tsx';
import { Btn } from '../ui/primitives.tsx';
import { useNav } from '../nav.tsx';
import { api, ApiError, ok } from '../api/client.ts';

type Result = { first_name: string; last_name: string };

const wait = (s: number) => (s < 90 ? `${s} seconds` : `${Math.ceil(s / 60)} minutes`);

export function lookupError(x: unknown): string {
	if (!(x instanceof ApiError)) return 'The server could not be reached.';
	const again = x.retryAfter ? ` Try again in ${wait(x.retryAfter)}.` : '';
	return `${x.message}${x.message.endsWith('.') ? '' : '.'}${again}`;
}

export function LookupName({ id, name, contactName, onClose }: { id: number; name: string; contactName?: string; onClose: () => void }) {
	const { refresh } = useNav();
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState('');
	const [done, setDone] = useState<Result | null>(null);
	const run = async () => {
		setBusy(true); setErr('');
		try {
			const r = await ok(api.POST('/api/users/{id}/lookup-name', { params: { path: { id } } }));
			setDone(r);
			void refresh();
		} catch (x) { setErr(lookupError(x)); }
		setBusy(false);
	};
	const own = done ? `${done.first_name} ${done.last_name}`.trim() : '';
	return (
		<Modal title="Look up real name" icon={<ScanSearch size={16} />} onClose={onClose}
			footer={done
				? <><span className="a-grow" /><Btn primary onClick={onClose}>Close</Btn></>
				: <><span className="a-grow" /><Btn ghost onClick={onClose}>{busy ? 'Close' : 'Cancel'}</Btn><Btn primary icon={ScanSearch} onClick={() => void run()} disabled={busy}>Look up</Btn></>}>
			{done ? (
				<div className="a-banner ok" role="status"><CircleCheck size={15} /><span>The own name of this user is <b>{own || '(empty)'}</b>; it is now stored.</span></div>
			) : (
				<>
					<div className="a-prose">
					<p>Telegram shows the logging account this user as it saved the contact{contactName ? <>, <b>{contactName}</b></> : ''}, not by the name they set. To read that name, the account removes <b>{name}</b> from its contacts, reads the name Telegram then reports, and adds the contact back exactly as it was.</p>
					<p>Before you go ahead:</p>
					<ul className="a-bullets">
						<li>The user can briefly see a change: the mutual-contact status, and the account's phone number if it shares it with them.</li>
						<li>Settings Telegram does not expose, such as a custom photo set for the contact, are not restored.</li>
						<li>Only one lookup runs at a time, a few minutes apart.</li>
					</ul>
					</div>
					{busy && <div className="a-banner info" role="status"><LoaderCircle size={15} className="a-spin" /><span>Looking up the name. This can take up to two minutes; the contact is added back even if you close this.</span></div>}
					{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
				</>
			)}
		</Modal>
	);
}
