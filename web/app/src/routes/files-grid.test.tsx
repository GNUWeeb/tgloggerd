// @vitest-environment happy-dom
// The files grid: it scrolls for more instead of paging, in batches that
// always end on a full row; the list view keeps its pages.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { readFileSync } from 'node:fs';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { gridBatch, type FilesProps } from './files.tsx';
import type { PageData } from '../page.ts';

// happy-dom lays nothing out: where the grid's end is, below the top of
// the window, is set by each test.
let footTop = 10000;
beforeEach(() => {
	setup();
	footTop = 10000;
	const real = Element.prototype.getBoundingClientRect;
	vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(function (this: Element) {
		return this.classList.contains('a-filegrid-foot') ? new DOMRect(0, footTop, 100, 40) : real.call(this);
	});
});
afterEach(() => { vi.restoreAllMocks(); });

const cards = () => [...document.querySelectorAll<HTMLElement>('.a-filecard')];
const cardNames = () => cards().map((c) => c.querySelector('.a-uname')!.textContent);
const batchCalls = (calls: ReturnType<typeof fakeApi>) => callsTo(calls, 'GET', '/api/page/files');
const noAuto = (url: string): Partial<PageData> => {
	const v = fixture(url).viewer;
	return { viewer: { ...v, chat: { ...v.chat!, auto_load: false } } };
};

describe('files grid', () => {
	it('maps the rows per page setting to a batch that every column count divides', () => {
		expect([10, 25, 50, 100, 200, 1000].map(gridBatch)).toEqual([24, 24, 48, 96, 96, 96]);
		for (const b of [24, 48, 96]) for (const n of [2, 3, 4, 6, 8]) expect(b % n).toBe(0);
	});

	it('shows no pager, and a first batch of 48 for 50 rows a page, 24 for 25', () => {
		fakeApi('superadmin');
		const { unmount } = renderPage('/files?per=50');
		expect(cards()).toHaveLength(48);
		expect(document.querySelector('.a-pager')).toBeNull();
		expect(screen.queryByRole('button', { name: 'Next page' })).toBeNull();
		unmount();
		renderPage('/files?per=25');
		expect(cards()).toHaveLength(24);
	});

	it('appends the next batch when scrolled near its end, fetched as the same list\'s page data', async () => {
		const calls = fakeApi('superadmin');
		const url = '/files?per=50&sort=size&dir=asc';
		renderPage(url);
		expect(batchCalls(calls)).toHaveLength(0);
		// Far from the end, a scroll loads nothing.
		fireEvent.scroll(window);
		expect(batchCalls(calls)).toHaveLength(0);
		footTop = innerHeight + 300;
		fireEvent.scroll(window);
		// The next batch moves the end down, out of reach again.
		footTop = 10000;
		await waitFor(() => expect(cards()).toHaveLength(96));
		expect(batchCalls(calls)).toHaveLength(1);
		const c = batchCalls(calls)[0]!;
		expect(Object.fromEntries(c.query)).toEqual({ per: '48', page: '2', sort: 'size', dir: 'asc' });
		// The two batches are the list's first 96 files, in order.
		const all = fixture<FilesProps>('/files?per=96&sort=size&dir=asc').props.rows;
		expect(all).toHaveLength(96);
		expect(cardNames()).toEqual(all.map((r) => r.name));
		// Nothing navigated: the address is the list's own.
		expect(location.pathname + location.search).toBe(url);
	});

	it('with loading while scrolling off, loads the next batch only on Load more', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/files?per=25', 'superadmin', noAuto('/files?per=25'));
		footTop = 0;
		fireEvent.scroll(window);
		expect(batchCalls(calls)).toHaveLength(0);
		fireEvent.click(screen.getByRole('button', { name: 'Load more' }));
		await waitFor(() => expect(cards()).toHaveLength(48));
		expect(batchCalls(calls)[0]!.query.get('per')).toBe('24');
		expect(batchCalls(calls)[0]!.query.get('page')).toBe('2');
		expect(batchCalls(calls)).toHaveLength(1);
	});

	it('says when every file is shown, with nothing more to load', () => {
		const calls = fakeApi('superadmin');
		const p = fixture<FilesProps>('/files').props;
		renderPage('/files', 'superadmin', { props: { ...p, rows: p.rows.slice(0, 5), total: 5 } });
		footTop = 0;
		fireEvent.scroll(window);
		expect(screen.getByRole('status').textContent).toBe('All 5 files shown');
		expect(screen.queryByRole('button', { name: 'Load more' })).toBeNull();
		expect(batchCalls(calls)).toHaveLength(0);
	});

	it('offers Load more again when a batch fails', async () => {
		let fail = true;
		const calls = fakeApi('superadmin', { 'GET /api/page/files': () => { if (fail) throw new TypeError('offline'); return fixture('/files?per=48&page=2'); } });
		renderPage('/files?per=50');
		footTop = 0;
		fireEvent.scroll(window);
		await waitFor(() => expect(screen.getByRole('button', { name: 'Load more' }).hasAttribute('disabled')).toBe(false));
		expect(batchCalls(calls)).toHaveLength(1);
		// No retry by itself after a failure.
		fireEvent.scroll(window);
		expect(batchCalls(calls)).toHaveLength(1);
		fail = false;
		fireEvent.click(screen.getByRole('button', { name: 'Load more' }));
		await waitFor(() => expect(cards()).toHaveLength(96));
	});

	it('opens the preview of a file from a later batch in place', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/files?per=25', 'superadmin', noAuto('/files?per=25'));
		fireEvent.click(screen.getByRole('button', { name: 'Load more' }));
		await waitFor(() => expect(cards()).toHaveLength(48));
		const card = cards()[30]!;
		const name = card.querySelector('.a-uname')!.textContent!;
		fireEvent.click(card);
		await screen.findByRole('dialog', { name });
		expect(new URLSearchParams(location.search).get('file')).toBeTruthy();
		expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(1);
	});

	it('uses only 2, 3, 4, 6 or 8 columns, chosen by the width it has', () => {
		const css = readFileSync('src/styles/app.css', 'utf8');
		// Every rule that sets the grid's columns, with the width it needs.
		const rules: [number, string][] = [];
		for (const m of css.matchAll(/(?:@container \(min-width: (\d+)px\) \{\s*)?\.app \.a-filegrid \{[^}]*grid-template-columns: ([^;]+);/g)) rules.push([Number(m[1] ?? 0), m[2]!]);
		expect(rules.length).toBeGreaterThan(0);
		for (const [, v] of rules) expect(v).toMatch(/^repeat\((2|3|4|6|8), minmax\(0, 1fr\)\)$/);
		const at = (w: number) => Number(rules.filter(([min]) => w >= min).sort((a, b) => a[0] - b[0]).at(-1)![1].match(/\d+/)![0]);
		expect([320, 500, 700, 1000, 1270].map(at)).toEqual([2, 3, 4, 6, 8]);
		// The widths are the grid's card's own.
		expect(css).toMatch(/\.app \.a-filegrid-card \{ container-type: inline-size; \}/);
	});

	it('keeps the list view\'s pages and page sizes', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/files?view=list');
		expect(document.querySelector('.a-filegrid')).toBeNull();
		const pager = document.querySelector<HTMLElement>('.a-pager')!;
		expect(within(pager).getByRole('combobox', { name: 'Rows' })).toBeTruthy();
		fireEvent.click(within(pager).getByRole('button', { name: 'Next page' }));
		await waitFor(() => expect(batchCalls(calls)).toHaveLength(1));
		expect(batchCalls(calls)[0]!.query.get('page')).toBe('2');
		expect(batchCalls(calls)[0]!.query.get('view')).toBe('list');
	});
});
