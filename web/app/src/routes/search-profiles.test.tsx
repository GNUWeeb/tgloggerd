// @vitest-environment happy-dom
// The search engine on the users and groups lists: the options it names
// for the entity (never facets), the backend note in the list's words,
// highlights drawn in the bio or description cell, and the full-text
// matches operator on those fields.
import { beforeEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import type { EngineProps } from '../api/engine.ts';
import type { UsersProps } from './users.tsx';
import type { GroupsProps } from './groups.tsx';

beforeEach(setup);

const engine = (field: string): NonNullable<EngineProps['engine']> => ({ fields: [field], options: ['phrase', 'prefix', 'fuzzy', 'highlight', 'facets'], available: true, fallback: null });
const options = () => screen.queryByRole('group', { name: 'Search options' });
const conditions = () => [...(screen.getByRole('combobox', { name: 'Condition' }) as HTMLSelectElement).options].map((o) => o.value);

function users(extra: Partial<UsersProps>, url = '/users') {
	const calls = fakeApi('superadmin');
	const page = fixture<UsersProps>(url);
	renderPage(url, 'superadmin', { props: { ...page.props, ...extra } });
	return { calls, page };
}
function groups(extra: Partial<GroupsProps>, url = '/groups') {
	const calls = fakeApi('superadmin');
	const page = fixture<GroupsProps>(url);
	renderPage(url, 'superadmin', { props: { ...page.props, ...extra } });
	return { calls, page };
}

describe('search engine on the users list', () => {
	it('offers nothing without an engine, or while it is not available', () => {
		users({});
		expect(options()).toBeNull();
		cleanup();
		users({ engine: { ...engine('bio'), available: false, fallback: 'behind' } });
		expect(options()).toBeNull();
	});

	it('offers the options the engine names, but never the chats with the most hits', async () => {
		const { calls } = users({ engine: engine('bio') });
		const g = within(options()!);
		expect(g.getAllByRole('checkbox').map((c) => c.closest('label')!.textContent)).toEqual(['Allow typos', 'Highlight matches']);
		expect(g.getByText('Highlight matches').closest('label')!.title).toBe('Show the matching parts of each user.');
		fireEvent.click(screen.getByRole('button', { name: /Search syntax/ }));
		expect(document.querySelector('.a-engine-syntax dd')!.textContent).toBe('users with the word rank higher');
		fireEvent.click(g.getByRole('checkbox', { name: 'Highlight matches' }));
		await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\//).at(-1)?.query.get('highlight')).toBe('1'));
	});

	it('says when the database answered, in the list\'s words', () => {
		users({ engine: engine('bio'), backend: { used: 'mysql', fallback: 'too_many_hits', ignored: ['highlight'] } });
		expect(screen.getByRole('status').textContent).toBe('This search matches too many users for the search engine to list, so the database answered it. Highlighted matches are not available for this search.');
	});

	it('draws the matched fragments in the bio cell, and never other markup', () => {
		const rows = fixture<UsersProps>('/users').props.rows;
		users({ engine: engine('bio'), backend: { used: 'engine' }, highlights: rows.map((_, i) => (i === 0 ? ['a <mark>kernel</mark> hacker &amp; more'] : i === 1 ? ['<img src=x onerror=alert(1)><mark>x</mark>'] : [])) });
		const trs = document.querySelectorAll('.a-tablecard tbody tr');
		const first = trs[0]!.querySelector('.a-hl')!;
		expect(first.querySelector('mark')!.textContent).toBe('kernel');
		expect(first.textContent).toBe('a kernel hacker & more');
		expect(document.querySelector('img[src="x"]')).toBeNull();
		// The unsafe one keeps the plain bio.
		expect(trs[1]!.querySelector('.a-hl')).toBeNull();
		if (rows[1]!.bio) expect(trs[1]!.textContent).toContain(rows[1]!.bio.slice(0, 10));
	});

	it('filters a bio by the full-text matches when the search offers it', async () => {
		const { calls } = users({});
		fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
		fireEvent.change(screen.getByRole('combobox', { name: 'Field' }), { target: { value: 'bio' } });
		expect(conditions()).toEqual(['contains', '=', 'empty', 'matches']);
		fireEvent.change(screen.getByRole('combobox', { name: 'Condition' }), { target: { value: 'matches' } });
		fireEvent.change(screen.getByRole('textbox', { name: /Value/ }), { target: { value: 'kernel' } });
		fireEvent.click(screen.getByRole('button', { name: 'Apply' }));
		await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\//).at(-1)?.query.get('f')).toBe(JSON.stringify([{ field: 'bio', op: 'matches', value: 'kernel' }])));
	});

	it('keeps the bio\'s own operators when the search lacks matches', () => {
		const p = fixture<UsersProps>('/users').props;
		users({ fields: p.fields!.map((f) => (f.key === 'bio' ? { ...f, operators: f.operators.filter((o) => o !== 'matches') } : f)) });
		fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
		fireEvent.change(screen.getByRole('combobox', { name: 'Field' }), { target: { value: 'bio' } });
		expect(conditions()).toEqual(['contains', '=', 'empty']);
	});
});

describe('search engine on the groups list', () => {
	it('offers the options the engine names, but never the chats with the most hits', () => {
		groups({ engine: engine('description') });
		expect(within(options()!).getAllByRole('checkbox').map((c) => c.closest('label')!.textContent)).toEqual(['Allow typos', 'Highlight matches']);
	});

	it('says when the database answered', () => {
		groups({ engine: { ...engine('description'), available: false, fallback: 'behind' }, backend: { used: 'mysql', fallback: 'behind', ignored: ['fuzzy'] } });
		expect(screen.getByRole('status').textContent).toBe('The search engine is still catching up with the archive, so the database answered this search. Typo tolerance is not available for this search.');
	});

	it('draws the matched fragments in the description cell', () => {
		const rows = fixture<GroupsProps>('/groups').props.rows;
		groups({ engine: engine('description'), backend: { used: 'engine' }, highlights: rows.map((_, i) => (i === 0 ? ['no <mark>spam</mark> here'] : [])) });
		const hl = document.querySelector('.a-tablecard tbody tr')!.querySelector('.a-hl')!;
		expect(hl.querySelector('mark')!.textContent).toBe('spam');
		expect(document.querySelectorAll('.a-tablecard tbody .a-hl')).toHaveLength(1);
	});

	it('filters a description by the full-text matches', () => {
		groups({});
		fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
		fireEvent.change(screen.getByRole('combobox', { name: 'Field' }), { target: { value: 'description' } });
		expect(conditions()).toEqual(['contains', '=', 'empty', 'matches']);
	});
});
