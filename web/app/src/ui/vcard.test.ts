import { describe, expect, it } from 'vitest';
import { parseVcard, sameNumber, vcfFilename } from './vcard.ts';

describe('parseVcard', () => {
	it('reads several numbers with their types, the e-mail addresses and the organisation', () => {
		const c = parseVcard('BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Dewi Lestari\r\nTEL;TYPE=CELL,VOICE:+62 812 0000 0101\r\nitem1.TEL;type=WORK;type=pref:+62 21 0000 0202\r\nEMAIL;TYPE=INTERNET:dewi@example.org\r\nORG:Example Co;Research\r\nEND:VCARD\r\n');
		expect(c.tels).toEqual([{ value: '+62 812 0000 0101', types: ['cell'] }, { value: '+62 21 0000 0202', types: ['work'] }]);
		expect(c.emails).toEqual([{ value: 'dewi@example.org', types: [] }]);
		expect(c.org).toBe('Example Co, Research');
	});

	it('takes LF line ends and unfolds continued lines', () => {
		const c = parseVcard('BEGIN:VCARD\nEMAIL:a.very.long.address.that.\n wraps@example.org\nORG:Two\n\t Words\nEND:VCARD');
		expect(c.emails.map((e) => e.value)).toEqual(['a.very.long.address.that.wraps@example.org']);
		expect(c.org).toBe('Two Words');
	});

	it('unescapes commas, semicolons, backslashes and newlines', () => {
		expect(parseVcard('ORG:Foo\\, Bar\\; Baz\\\\Qux\\nNext;Dept').org).toBe('Foo, Bar; Baz\\Qux\nNext, Dept');
	});

	it('reads vCard 2.1: bare types, quoted-printable with soft breaks, and 4.0 tel: URIs', () => {
		const c = parseVcard('BEGIN:VCARD\nVERSION:2.1\nTEL;CELL:0812\nTEL;VALUE=uri;TYPE=home:tel:+62-21-555\nORG;CHARSET=UTF-8;ENCODING=QUOTED-PRINTABLE:Caf=C3=A9 =\nNusantara\nEND:VCARD');
		expect(c.tels).toEqual([{ value: '0812', types: ['cell'] }, { value: '+62-21-555', types: ['home'] }]);
		expect(c.org).toBe('Café Nusantara');
	});

	it('drops repeated numbers and survives malformed input', () => {
		expect(parseVcard('TEL:1\nTEL;CELL:1').tels).toHaveLength(1);
		for (const bad of [null, undefined, '', 'garbage', ':nothing', 'TEL:', 'TEL;TYPE="unterminated:+1', 'ORG:=ZZ', '\n\n\r']) {
			const c = parseVcard(bad);
			expect(c.emails).toEqual([]);
			expect(Array.isArray(c.tels)).toBe(true);
		}
	});
});

describe('sameNumber', () => {
	it('ignores how a number is spaced', () => {
		expect(sameNumber('+62 812 0000 0101', '+6281200000101')).toBe(true);
		expect(sameNumber('+62 812 0000 0101', '+62 812 0000 0102')).toBe(false);
		expect(sameNumber('', '')).toBe(false);
	});
});

describe('vcfFilename', () => {
	it('names the file after the contact, without path or control characters', () => {
		expect(vcfFilename('Dewi Lestari')).toBe('Dewi Lestari.vcf');
		expect(vcfFilename('../../etc/passwd')).toBe('etc_passwd.vcf');
		expect(vcfFilename('a\u0000b\nc:d*e')).toBe('a_b_c_d_e.vcf');
		expect(vcfFilename('...')).toBe('contact.vcf');
		expect(vcfFilename('Ünïcødé 名前')).toBe('Ünïcødé 名前.vcf');
		expect(vcfFilename('x'.repeat(200))).toHaveLength(64 + 4);
	});
});
