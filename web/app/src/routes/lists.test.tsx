// @vitest-environment happy-dom
// The lists with the advanced search: its fields and operators, AND and
// OR, refused and timed-out searches, estimates, page sizes, long texts,
// and old (Drogon) list URLs shown as the current ones.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import type { UsersProps } from './users.tsx';
import type { MessagesProps } from './messages.tsx';

beforeEach(setup);
const pageCalls = (calls: ReturnType<typeof fakeApi>) => callsTo(calls, 'GET', /^\/api\/page\//);
const conds = (calls: ReturnType<typeof fakeApi>) => JSON.parse(pageCalls(calls).at(-1)!.query.get('f') ?? '[]');
const open = () => fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
const pickField = (key: string) => fireEvent.change(screen.getByRole('combobox', { name: 'Field' }), { target: { value: key } });
const pickOp = (op: string) => fireEvent.change(screen.getByRole('combobox', { name: 'Condition' }), { target: { value: op } });

describe('lists', () => {
	it('offers the search\'s fields with their own operators, after the list\'s', () => {
		renderPage('/users');
		open();
		const field = screen.getByRole('combobox', { name: 'Field' }) as HTMLSelectElement;
		const groups = [...field.querySelectorAll('optgroup')].map((g) => g.label);
		expect(groups).toEqual(['This list', 'Advanced search']);
		pickField('first_name');
		expect([...(screen.getByRole('combobox', { name: 'Condition' }) as HTMLSelectElement).options].map((o) => o.value)).toEqual(['=', '!=', 'LIKE', 'NOT LIKE', '%LIKE%', 'NOT %LIKE%']);
		pickOp('LIKE');
		expect(screen.getByText(/% matches any text/)).toBeTruthy();
	});

	it('sends a value-less filter without a value', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/users');
		open();
		pickField('bio');
		const ops = [...(screen.getAllByRole('combobox', { name: 'Condition' })[0] as HTMLSelectElement).options].map((o) => o.value);
		// The list's own bio filter comes first; the search's comes after.
		expect(ops).toContain('contains');
		pickField('phone');
		pickOp('IS NULL');
		expect(screen.queryByRole('textbox', { name: 'Value' })).toBeNull();
		fireEvent.click(screen.getByRole('button', { name: 'Apply' }));
		await waitFor(() => expect(pageCalls(calls)).toHaveLength(1));
		expect(conds(calls)).toEqual([{ field: 'phone', op: 'IS NULL', value: '' }]);
	});

	it('compares a search bool as 1 or 0', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/users');
		open();
		pickField('is_contact');
		const value = screen.getAllByRole('combobox').find((x) => x.closest('label')?.textContent?.startsWith('Value')) as HTMLSelectElement;
		expect([...value.options].map((o) => [o.value, o.textContent])).toEqual([['1', 'yes'], ['0', 'no']]);
		fireEvent.change(value, { target: { value: '0' } });
		fireEvent.click(screen.getByRole('button', { name: 'Apply' }));
		await waitFor(() => expect(pageCalls(calls)).toHaveLength(1));
		expect(conds(calls)).toEqual([{ field: 'is_contact', op: '=', value: '0' }]);
	});

	it('joins filters with AND or OR', async () => {
		const f = [{ field: 'id', op: '=', value: '1' }, { field: 'id', op: '=', value: '2' }];
		const calls = fakeApi('superadmin');
		renderPage(`/users?f=${encodeURIComponent(JSON.stringify(f))}`);
		fireEvent.click(screen.getByRole('button', { name: 'AND' }));
		await waitFor(() => expect(pageCalls(calls)).toHaveLength(1));
		expect(conds(calls)).toEqual([{ field: 'id', op: '=', value: '1', n: 'OR' }, { field: 'id', op: '=', value: '2' }]);
	});

	it('shows OR between filters joined so, and toggles it back to AND', async () => {
		const f = [{ field: 'id', op: '=', value: '1', n: 'OR' }, { field: 'id', op: '=', value: '2' }];
		const calls = fakeApi('superadmin');
		renderPage(`/users?f=${encodeURIComponent(JSON.stringify(f))}`);
		fireEvent.click(screen.getByRole('button', { name: 'OR' }));
		await waitFor(() => expect(pageCalls(calls)).toHaveLength(1));
		expect(conds(calls)).toEqual([{ field: 'id', op: '=', value: '1' }, { field: 'id', op: '=', value: '2' }]);
	});

	it('says why a search was refused and shows the list without it', () => {
		const f = [{ field: 'is_premium', op: 'LIKE', value: 'x' }];
		renderPage(`/users?f=${encodeURIComponent(JSON.stringify(f))}`);
		const b = screen.getByRole('alert');
		expect(b.textContent).toContain('This search cannot run');
		expect(b.textContent).toContain('Premium does not take LIKE');
		expect(document.querySelectorAll('.a-chip')).toHaveLength(0);
	});

	it('says when a search ran out of time', () => {
		renderPage('/group_messages?q=slowsearch');
		expect(screen.getByRole('alert').textContent).toContain('took too long');
	});

	it('marks an estimated count as about', () => {
		const p = fixture<MessagesProps>('/group_messages').props;
		expect(p.total_exact).toBe(false);
		renderPage('/group_messages');
		expect(screen.getAllByText(/of about /).length).toBeGreaterThan(0);
	});

	it('offers page sizes up to 1000', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/users');
		const per = screen.getAllByRole('combobox').find((s) => s.closest('.a-per')) as HTMLSelectElement;
		expect([...per.options].map((o) => o.value)).toEqual(['25', '50', '100', '200', '500', '1000']);
		fireEvent.change(per, { target: { value: '1000' } });
		await waitFor(() => expect(pageCalls(calls)).toHaveLength(1));
		expect(pageCalls(calls)[0]!.query.get('per')).toBe('1000');
	});

	it('sorts by a sortable search field', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/users');
		fireEvent.change(screen.getByRole('combobox', { name: 'Sort by' }), { target: { value: 'first_name' } });
		await waitFor(() => expect(pageCalls(calls)).toHaveLength(1));
		expect(pageCalls(calls)[0]!.query.get('sort')).toBe('first_name');
	});

	it('shows a long bio whole in a dialog', () => {
		const p = fixture<UsersProps>('/users').props;
		const bio = 'Line one of a long bio.\nLine two, with much more to say than fits in a table cell.';
		const rows = [{ ...p.rows[0]!, bio }, ...p.rows.slice(1)];
		renderPage('/users', 'superadmin', { props: { ...p, rows } });
		fireEvent.click(screen.getByRole('button', { name: `Show all of Bio of ${rows[0]!.name}` }));
		const d = screen.getByRole('dialog', { name: `Bio of ${rows[0]!.name}` });
		expect(within(d).getByText(/Line two/).textContent).toBe(bio);
	});

	it('shows a member only the users fields it may search', () => {
		renderPage('/users', 'member');
		open();
		const keys = [...(screen.getByRole('combobox', { name: 'Field' }) as HTMLSelectElement).options].map((o) => o.value);
		expect(keys).toContain('has_photo');
		expect(keys).not.toContain('first_name');
		expect(screen.queryByRole('columnheader', { name: 'Bio' })).toBeNull();
	});
});

describe('old list URLs', () => {
	it('shows the current URL for a Drogon list URL opened directly', () => {
		const old = `/users?search=${encodeURIComponent(JSON.stringify([{ c: 'first_name', o: 'LIKE', v: 'D%' }]))}&offset=50&limit=25&order=asc`;
		const { page } = renderPage(old);
		expect(page.url).not.toContain('search=');
		expect(location.pathname + location.search).toBe(page.url);
		expect(screen.getByText('LIKE')).toBeTruthy();
	});

	it('shows the current URL after going back to a Drogon list URL', async () => {
		fakeApi('superadmin');
		renderPage('/groups');
		const old = `/users?search=${encodeURIComponent(JSON.stringify([{ c: 'id', o: '=', v: '1' }]))}`;
		// Back or forward to an old URL loads it like a link does.
		history.pushState(null, '', old);
		window.dispatchEvent(new PopStateEvent('popstate'));
		await waitFor(() => expect(location.search).not.toContain('search='));
		expect(JSON.parse(new URLSearchParams(location.search).get('f')!)).toEqual([{ field: 'id', op: '=', value: '1' }]);
	});
});
