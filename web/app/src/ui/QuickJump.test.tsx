// @vitest-environment happy-dom
// "Search everything": Ctrl-K or the top-bar button, pages and chats,
// keys to move and open, Esc to close.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, renderPage, setup } from '../test/page.tsx';

beforeEach(setup);
const chats = { 'GET /api/chats/search': (c: { query: URLSearchParams }) => ({
	items: c.query.get('q') === 'rust' ? [{ id: -1001, title: 'Rust Indonesia', kind: 'group', sub: 'supergroup', photo: null }, { id: 42, title: 'Rustacean Ayu', kind: 'user', sub: 'private chat', photo: null }] : [],
	next_cursor: null,
}) };
const dialog = () => screen.queryByRole('dialog', { name: 'Search everything' });
const pageCalls = (calls: ReturnType<typeof fakeApi>) => callsTo(calls, 'GET', /^\/api\/page\//);

describe('quick jump', () => {
	it('opens with Ctrl-K and Cmd-K, and closes with Esc', () => {
		fakeApi('superadmin', chats);
		renderPage('/');
		fireEvent.keyDown(window, { key: 'k', ctrlKey: true });
		expect(dialog()).toBeTruthy();
		fireEvent.keyDown(screen.getByRole('combobox'), { key: 'Escape' });
		expect(dialog()).toBeNull();
		fireEvent.keyDown(window, { key: 'K', metaKey: true });
		expect(dialog()).toBeTruthy();
	});

	it('opens from the top-bar button', () => {
		fakeApi('superadmin', chats);
		renderPage('/');
		fireEvent.click(screen.getByRole('button', { name: 'Search everything' }));
		expect(document.activeElement).toBe(screen.getByRole('combobox'));
	});

	it('finds chats and pages, and opens the one chosen with the keys', async () => {
		const calls = fakeApi('superadmin', chats);
		renderPage('/');
		fireEvent.click(screen.getByRole('button', { name: 'Search everything' }));
		fireEvent.change(screen.getByRole('combobox'), { target: { value: 'rust' } });
		const list = screen.getByRole('listbox');
		await within(list).findByText('Rust Indonesia');
		expect(callsTo(calls, 'GET', '/api/chats/search').at(-1)!.query.get('q')).toBe('rust');
		expect(within(list).getByText('Rustacean Ayu')).toBeTruthy();
		fireEvent.keyDown(screen.getByRole('combobox'), { key: 'ArrowDown' });
		expect(within(list).getAllByRole('option')[1]!.getAttribute('aria-selected')).toBe('true');
		fireEvent.keyDown(screen.getByRole('combobox'), { key: 'Enter' });
		await waitFor(() => expect(pageCalls(calls).at(-1)?.path).toBe('/api/page/users/42'));
		expect(dialog()).toBeNull();
	});

	it('matches the menu\'s pages by name', async () => {
		// Only the request for the files page matters here: building and
		// drawing the stand-in's whole files list took seconds, too long
		// under load, so its answer never comes.
		const calls = fakeApi('superadmin', { ...chats, 'GET /api/page/files': () => new Promise<never>(() => {}) });
		renderPage('/');
		fireEvent.click(screen.getByRole('button', { name: 'Search everything' }));
		fireEvent.change(screen.getByRole('combobox'), { target: { value: 'file' } });
		const files = within(screen.getByRole('listbox')).getByText('Files');
		fireEvent.mouseDown(files.closest('li')!);
		await waitFor(() => expect(pageCalls(calls).at(-1)?.path).toBe('/api/page/files'));
	});

	it('offers a member only its own pages', () => {
		fakeApi('member', chats);
		renderPage('/', 'member');
		fireEvent.click(screen.getByRole('button', { name: 'Search everything' }));
		fireEvent.change(screen.getByRole('combobox'), { target: { value: 'accounts' } });
		expect(within(screen.getByRole('listbox')).queryByText('Accounts')).toBeNull();
	});

	it('says when nothing matches', async () => {
		fakeApi('superadmin', chats);
		renderPage('/');
		fireEvent.click(screen.getByRole('button', { name: 'Search everything' }));
		fireEvent.change(screen.getByRole('combobox'), { target: { value: 'zzzz' } });
		expect(await screen.findByText('Nothing matches.')).toBeTruthy();
	});
});
