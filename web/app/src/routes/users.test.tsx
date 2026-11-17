// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, renderPage, setup } from '../test/page.tsx';

beforeEach(setup);

describe('users list', () => {
	it('shows the fixture rows with names and counts', () => {
		const { page } = renderPage('/users');
		const rows = (page.props as { rows: { name: string }[] }).rows;
		expect(screen.getByRole('heading', { level: 1 }).textContent).toBe('Users');
		expect(screen.getAllByText(rows[0]!.name).length).toBeGreaterThan(0);
	});

	it('searches through the URL, so the page data is fetched for it', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/users');
		fireEvent.change(screen.getByRole('textbox', { name: 'Search' }), { target: { value: 'dewi' } });
		fireEvent.submit(screen.getByRole('search'));
		await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(1));
		const c = callsTo(calls, 'GET', /^\/api\/page\//)[0]!;
		expect(c.path).toBe('/api/page/users');
		expect(c.query.get('q')).toBe('dewi');
		await waitFor(() => expect(location.search).toBe('?q=dewi'));
	});

	it('saves the current search under a title it suggests', async () => {
		const calls = fakeApi('superadmin', { 'POST /api/account/saved-searches': () => ({ id: 9 }) });
		renderPage('/users?q=dewi');
		fireEvent.click(screen.getByRole('button', { name: /Save search/ }));
		const dialog = screen.getByRole('dialog');
		expect((within(dialog).getByRole('textbox') as HTMLInputElement).value).toBe('"dewi" in Users');
		fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }));
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/account/saved-searches')).toHaveLength(1));
		expect(callsTo(calls, 'POST', '/api/account/saved-searches')[0]!.body).toEqual({ title: '"dewi" in Users', url: '/users?q=dewi' });
		await screen.findByText(/Saved\. Your saved searches/);
	});

	it('offers no save without a search', () => {
		renderPage('/users');
		expect(screen.queryByRole('button', { name: /Save search/ })).toBeNull();
	});

	it('shows message counts to an admin but not to a member', () => {
		renderPage('/users');
		expect(screen.getAllByRole('columnheader').map((h) => h.textContent)).toContain('Messages');
	});

	it('shows a member only the columns it may see', () => {
		renderPage('/users', 'member');
		expect(screen.getAllByRole('columnheader').map((h) => h.textContent)).not.toContain('Messages');
	});
});
