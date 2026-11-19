// @vitest-environment happy-dom
// A shared contact's vCard under its card: the other numbers, the
// address, the organisation and the .vcf download, only for a viewer
// who may see phones.
import { beforeEach, describe, expect, it } from 'vitest';
import { within } from '@testing-library/react';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { KINDS_GROUP } from '../../standin/kinds.ts';
import { vcfFilename } from '../ui/vcard.ts';
import type { ChatProps } from './chat.tsx';

beforeEach(setup);

const url = `/groups/${KINDS_GROUP.id}/chat`;
const contactIn = (persona: 'superadmin' | 'member') => {
	fakeApi(persona);
	renderPage(url, persona);
	const m = fixture<ChatProps>(url, persona).props.messages.find((x) => x.detail?.kind === 'contact')!;
	if (m.detail?.kind !== 'contact') throw new Error('no contact sample');
	return { d: m.detail, card: document.getElementById(`m${m.message_id}`)!.querySelector<HTMLElement>('.k-contact')! };
};

describe('a contact\'s vCard', () => {
	it('shows both numbers, the address and the organisation, and downloads as a .vcf', () => {
		const { d, card } = contactIn('superadmin');
		const c = within(card);
		expect(c.getByText('+62 812 0000 0101')).toBeTruthy();
		expect(c.getByText('+62 21 0000 0202')).toBeTruthy();
		// The number the card already shows is not listed again.
		expect(c.queryByText('+6281200000101')).toBeNull();
		expect(c.getByRole('link', { name: 'contact.sample@example.org' }).getAttribute('href')).toBe('mailto:contact.sample@example.org');
		expect(c.getByText('Sample Works, Ltd, Field Team')).toBeTruthy();
		const save = c.getByRole('link', { name: 'Save as vCard' });
		expect(save.getAttribute('href')).toMatch(/^blob:/);
		expect(save.getAttribute('download')).toBe(vcfFilename(`${d.first_name} ${d.last_name}`.trim()));
		expect(save.getAttribute('download')).toMatch(/\.vcf$/);
	});

	it('shows a viewer who may not see phones neither the numbers nor the download', () => {
		const { d, card } = contactIn('member');
		expect(d.vcard).toBeNull();
		const c = within(card);
		expect(c.queryByText('+62 21 0000 0202')).toBeNull();
		expect(c.queryByText('contact.sample@example.org')).toBeNull();
		expect(c.queryByText('Sample Works, Ltd, Field Team')).toBeNull();
		expect(c.queryByRole('link', { name: 'Save as vCard' })).toBeNull();
	});
});
