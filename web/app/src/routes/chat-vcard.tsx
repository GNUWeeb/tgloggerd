// What a contact card adds from the contact's vCard: the other numbers,
// the e-mail addresses, the organisation, and the vCard as a .vcf file.
// Go sends the vCard only to viewers who may see users' phones, so with
// none there is nothing to add.
import { useEffect, useMemo, useState } from 'react';
import { Building2, Download, Mail, Phone } from 'lucide-react';
import { parseVcard, sameNumber, vcfFilename } from '../ui/vcard.ts';

export function VcardExtra({ vcard, phone, name }: { vcard: string | null; phone: string | null; name: string }) {
	const card = useMemo(() => parseVcard(vcard), [vcard]);
	const [href, setHref] = useState('');
	useEffect(() => {
		if (!vcard) return;
		// A .vcf file uses CRLF line ends (RFC 6350 3.2).
		const url = URL.createObjectURL(new Blob([vcard.replace(/\r?\n/g, '\r\n')], { type: 'text/vcard;charset=utf-8' }));
		setHref(url);
		return () => URL.revokeObjectURL(url);
	}, [vcard]);
	if (!vcard) return null;
	// The number Go already shows is not listed twice.
	const tels = card.tels.filter((t) => !phone || !sameNumber(t.value, phone));
	return (
		<>
			{tels.map((t) => <div key={t.value} className="k-line"><Phone size={13} aria-hidden="true" /><span className="mono">{t.value}</span>{t.types.length > 0 && <span className="c-sub">{t.types.join(', ')}</span>}</div>)}
			{card.emails.map((e) => <div key={e.value} className="k-line"><Mail size={13} aria-hidden="true" /><a href={`mailto:${e.value}`} className="a-link">{e.value}</a></div>)}
			{card.org && <div className="k-line"><Building2 size={13} aria-hidden="true" /><span>{card.org}</span></div>}
			{href && <a href={href} download={vcfFilename(name)} className="k-line a-link"><Download size={13} aria-hidden="true" />Save as vCard</a>}
		</>
	);
}
