// @vitest-environment happy-dom
// A file's role in its message on /files: a link preview's picture, a
// video's cover and the like say so with a tag in the row, the card and
// the preview, which links to the message; the role filter lists only
// the files of that role.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import type { FilesProps } from './files.tsx';
import { roleOf, type FileRole } from '../ui/fileroles.tsx';

beforeEach(setup);

const LINK = [{ field: 'role', op: '=', value: 'link_preview' }];
const withConds = (path: string, conds: unknown[]) => `${path}${path.includes('?') ? '&' : '?'}f=${encodeURIComponent(JSON.stringify(conds))}`;
const rolesOf = (url: string) => fixture<FilesProps>(url).props.rows.map((r) => roleOf(r));
const valueSelect = () => screen.getAllByRole('combobox').find((x) => x.closest('label')?.textContent?.startsWith('Value')) as HTMLSelectElement;

describe('the stand-in sends FileRow.role', () => {
	it('as null for a message\'s own file and { role, message_kind } for the others, link previews among them', () => {
		const roles = rolesOf('/files?view=list&per=1000');
		expect(roles).toContain(null);
		const set = roles.filter((r): r is NonNullable<FileRole> => !!r);
		expect(new Set(set.map((r) => r.role))).toEqual(new Set(['link_preview', 'cover', 'invoice', 'game', 'paid', 'thumbnail']));
		expect(set).toContainEqual({ role: 'link_preview', message_kind: 'text' });
	});

	it('filters by role: only link previews, each with its message', () => {
		const rows = fixture<FilesProps>(withConds('/files?view=list&per=1000', LINK)).props.rows;
		expect(rows.length).toBeGreaterThan(0);
		expect(rows.every((r) => roleOf(r)?.role === 'link_preview' && r.message)).toBe(true);
		const other = fixture<FilesProps>(withConds('/files?view=list&per=1000', [{ field: 'role', op: '!=', value: 'link_preview' }])).props.rows;
		expect(other.some((r) => roleOf(r)?.role === 'link_preview')).toBe(false);
	});

	it('filters by several roles at once with in', () => {
		const rows = fixture<FilesProps>(withConds('/files?view=list&per=1000', [{ field: 'role', op: 'in', value: ['cover', 'game'] }])).props.rows;
		expect(new Set(rows.map((r) => roleOf(r)?.role))).toEqual(new Set(['cover', 'game']));
	});
});

describe('the files page shows the role', () => {
	const linkRow = () => fixture<FilesProps>(withConds('/files?view=list', LINK)).props.rows[0]!;

	it('as a tag in the list\'s row, and none on a message\'s own file', () => {
		const all = fixture<FilesProps>('/files?view=list&per=200').props.rows;
		renderPage('/files?view=list&per=200');
		const f = all.find((r) => roleOf(r)?.role === 'link_preview')!;
		const row = screen.getAllByRole('row').find((r) => r.textContent?.includes(f.name))!;
		const tag = row.querySelector('.a-roletag')!;
		expect(tag.textContent).toBe('Link preview');
		expect(tag.getAttribute('title')).toBe('Link preview of its message (Text)');
		const own = all.find((r) => roleOf(r) === null)!;
		const ownRow = screen.getAllByRole('row').find((r) => r.textContent?.includes(own.name))!;
		expect(ownRow.querySelector('.a-roletag')).toBeNull();
	});

	it('as a tag on the grid\'s card', () => {
		renderPage(withConds('/files', LINK));
		const cards = [...document.querySelectorAll('.a-filecard')];
		expect(cards.length).toBeGreaterThan(0);
		for (const c of cards) expect(c.querySelector('.a-roletag')!.textContent).toBe('Link preview');
	});

	it('in the preview, linked to its message', async () => {
		const f = linkRow();
		renderPage(withConds(`/files?view=list&file=${f.id}`, LINK));
		const d = await screen.findByRole('dialog', { name: f.name });
		const kv = [...d.querySelectorAll('dt')].find((x) => x.textContent === 'Role')!.nextElementSibling as HTMLElement;
		expect(kv.querySelector('.a-roletag')!.textContent).toBe('Link preview');
		expect(within(kv).getByRole('link', { name: 'of its message (Text)' }).getAttribute('href')).toBe(`/messages/${f.message!.scope}/${f.message!.id}`);
	});
});

describe('the role filter', () => {
	it('is a list of roles by their labels in the filter bar', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/files?view=list');
		fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
		fireEvent.change(screen.getByRole('combobox', { name: 'Field' }), { target: { value: 'role' } });
		const opts = [...valueSelect().options].map((o) => [o.value, o.textContent]);
		for (const o of [['link_preview', 'Link preview'], ['cover', 'Cover'], ['game', 'Game'], ['invoice', 'Invoice'], ['paid', 'Paid item'], ['thumbnail', 'Thumbnail']]) expect(opts).toContainEqual(o);
		fireEvent.change(valueSelect(), { target: { value: 'link_preview' } });
		fireEvent.click(screen.getByRole('button', { name: 'Apply' }));
		await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(1));
		expect(JSON.parse(callsTo(calls, 'GET', /^\/api\/page\//).at(-1)!.query.get('f')!)).toEqual(LINK);
	});

	it('has a "Link previews" quick filter, shown by its label', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/files?view=list');
		fireEvent.click(screen.getByRole('button', { name: 'Link previews' }));
		await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(1));
		expect(JSON.parse(callsTo(calls, 'GET', /^\/api\/page\//).at(-1)!.query.get('f')!)).toEqual(LINK);
	});
});
