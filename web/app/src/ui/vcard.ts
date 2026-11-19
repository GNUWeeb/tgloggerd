// What a shared contact's vCard holds beyond its name: the numbers, the
// e-mail addresses and the organisation. Telegram passes on whatever
// the sender's phone wrote (vCard 2.1, 3.0 or 4.0), so this reads all
// three loosely and skips what it cannot make sense of.

export type VcardValue = { value: string; types: string[] };
export type Vcard = { tels: VcardValue[]; emails: VcardValue[]; org: string };

// Types that say nothing a reader needs.
const QUIET = new Set(['pref', 'voice', 'internet', 'x-internet']);

// Split at a separator that is not escaped with a backslash.
function splitUnescaped(s: string, sep: string): string[] {
	const out: string[] = [];
	let cur = '';
	for (let i = 0; i < s.length; i++) {
		const c = s[i]!;
		if (c === '\\' && i + 1 < s.length) {
			cur += c + s[i + 1];
			i++;
		} else if (c === sep) {
			out.push(cur);
			cur = '';
		} else cur += c;
	}
	out.push(cur);
	return out;
}

const unescape = (s: string) => s.replace(/\\([nN,;:\\])/g, (_, c: string) => (c === 'n' || c === 'N' ? '\n' : c));

// vCard 2.1's quoted-printable, as UTF-8 bytes.
function decodeQP(s: string): string {
	const bytes: number[] = [];
	for (let i = 0; i < s.length; i++) {
		const hex = s[i] === '=' ? s.slice(i + 1, i + 3) : '';
		if (/^[0-9A-Fa-f]{2}$/.test(hex)) {
			bytes.push(parseInt(hex, 16));
			i += 2;
		} else for (const b of new TextEncoder().encode(s[i]!)) bytes.push(b);
	}
	return new TextDecoder().decode(new Uint8Array(bytes));
}

// The physical lines joined into content lines: a line that starts with
// a space or a tab continues the one before (RFC 6350 3.2), and in a
// quoted-printable value a line that ends in "=" goes on in the next.
function contentLines(text: string): string[] {
	const out: string[] = [];
	for (const line of text.replace(/\r\n?/g, '\n').split('\n')) {
		const last = out.length - 1;
		if (last >= 0 && /^[ \t]/.test(line)) out[last] += line.slice(1);
		else if (last >= 0 && /QUOTED-PRINTABLE/i.test(out[last]!.split(':')[0]!) && out[last]!.endsWith('=')) out[last] = out[last]!.slice(0, -1) + line;
		else out.push(line);
	}
	return out;
}

// The colon that ends the name and parameters, outside quoted values.
function valueStart(line: string): number {
	let quoted = false;
	for (let i = 0; i < line.length; i++) {
		if (line[i] === '"') quoted = !quoted;
		else if (line[i] === ':' && !quoted) return i;
	}
	return -1;
}

export function parseVcard(text: string | null | undefined): Vcard {
	const card: Vcard = { tels: [], emails: [], org: '' };
	if (!text) return card;
	for (const line of contentLines(text)) {
		const colon = valueStart(line);
		if (colon <= 0) continue;
		const [rawName = '', ...params] = line.slice(0, colon).split(';');
		// A group prefix ("item1.TEL") does not change what the line is.
		const name = rawName.slice(rawName.lastIndexOf('.') + 1).trim().toUpperCase();
		if (name !== 'TEL' && name !== 'EMAIL' && name !== 'ORG') continue;
		let raw = line.slice(colon + 1);
		const types: string[] = [];
		for (const p of params) {
			const eq = p.indexOf('=');
			const key = (eq < 0 ? 'TYPE' : p.slice(0, eq)).trim().toUpperCase();
			const val = (eq < 0 ? p : p.slice(eq + 1)).replace(/"/g, '');
			if (key === 'ENCODING' && val.toUpperCase() === 'QUOTED-PRINTABLE') raw = decodeQP(raw);
			// 2.1 writes a bare "CELL"; 3.0 "TYPE=cell,voice".
			else if (key === 'TYPE') for (const t of val.split(',')) if (t.trim()) types.push(t.trim().toLowerCase());
		}
		const shown = types.filter((t) => !QUIET.has(t));
		if (name === 'ORG') {
			if (!card.org) card.org = splitUnescaped(raw, ';').map((p) => unescape(p).trim()).filter(Boolean).join(', ');
			continue;
		}
		const value = unescape(raw).trim().replace(name === 'TEL' ? /^tel:/i : /^mailto:/i, '');
		if (!value) continue;
		const list = name === 'TEL' ? card.tels : card.emails;
		if (!list.some((x) => x.value === value)) list.push({ value, types: shown });
	}
	return card;
}

// A number as only its digits and a leading plus, to tell the same
// number apart from a different one however each is spaced.
export const sameNumber = (a: string, b: string) => {
	const n = (s: string) => s.replace(/[^\d+]/g, '');
	return n(a) !== '' && n(a) === n(b);
};

// A file name for the vCard from the contact's name: letters, digits,
// spaces, dots, dashes and underscores only, so no path or control
// character comes from the sender.
export function vcfFilename(name: string): string {
	const base = name.normalize('NFC').replace(/[^\p{L}\p{N} ._-]+/gu, '_').replace(/\s+/g, ' ').replace(/^[ ._]+|[ ._]+$/g, '').slice(0, 64).trim();
	return `${base || 'contact'}.vcf`;
}
