// @vitest-environment happy-dom
// The search engine's options on the message lists: offered only when
// the engine is available, and what the answer adds (which backend
// answered, highlights, the chats with the most hits).
import { beforeEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { safeFragment, unescapeHtml, type EngineProps } from '../api/engine.ts';
import type { MessagesProps } from '../routes/messages.tsx';

beforeEach(setup);

const URL = '/group_messages?q=kernel';
const ENGINE = { fields: ['text'], options: ['phrase', 'proximity', 'prefix', 'fuzzy', 'highlight', 'facets'], available: true, fallback: null } satisfies EngineProps['engine'];
function show(extra: EngineProps, url = URL) {
	const calls = fakeApi('superadmin');
	const page = fixture<MessagesProps>(url);
	renderPage(url, 'superadmin', { props: { ...page.props, ...extra } });
	return { calls, page };
}
const options = () => screen.queryByRole('group', { name: 'Search options' });

describe('search engine options', () => {
	it('shows none without an engine, or while it is not available', () => {
		show({});
		expect(options()).toBeNull();
		expect(screen.queryByRole('button', { name: /Search syntax/ })).toBeNull();
		cleanup();
		show({ engine: { ...ENGINE, available: false, fallback: 'behind' } });
		expect(options()).toBeNull();
	});

	it('offers the options the engine names, as URL parameters', async () => {
		const { calls } = show({ engine: { ...ENGINE, options: ['phrase', 'fuzzy', 'highlight'] } });
		const g = within(options()!);
		expect(g.getAllByRole('checkbox').map((c) => c.closest('label')!.textContent)).toEqual(['Allow typos', 'Highlight matches']);
		// The search box reaches the engine here, so no matches hint.
		expect(g.queryByText('works with a matches filter')).toBeNull();
		fireEvent.click(g.getByRole('checkbox', { name: 'Allow typos' }));
		await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\//).at(-1)?.query.get('fuzzy')).toBe('1'));
	});

	it('explains the syntax it reads, and only that', () => {
		show({ engine: { ...ENGINE, options: ['phrase', 'prefix'] } });
		fireEvent.click(screen.getByRole('button', { name: /Search syntax/ }));
		const forms = [...document.querySelectorAll('.a-engine-syntax dt')].map((d) => d.textContent);
		expect(forms).toEqual(['word', '+word  -word', '"a phrase"', 'word*']);
	});

	// Four renders of the list: under a loaded full run this takes longer
	// than the default five seconds.
	it('says when the database answered instead, and what it left out', () => {
		show({ engine: { ...ENGINE, available: false, fallback: 'behind' }, backend: { used: 'mysql', fallback: 'behind', ignored: ['highlight', 'facets'] } });
		expect(screen.getByRole('status').textContent).toBe('The search engine is still catching up with the archive, so the database answered this search. Highlighted matches and the chats with the most hits are not available for this search.');
		cleanup();
		show({ engine: ENGINE, backend: { used: 'engine', ignored: ['highlight'] } });
		expect(screen.getByRole('status').textContent).toBe('Highlighted matches are not available for this search.');
		cleanup();
		show({ engine: ENGINE, backend: { used: 'mysql', fallback: 'too_many_hits', ignored: ['fuzzy'] } });
		expect(screen.getByRole('status').textContent).toMatch(/so the database answered it\. Typo tolerance is not available for this search\.$/);
		cleanup();
		show({ engine: ENGINE, backend: { used: 'engine' } });
		expect(screen.queryByText(/database answered/)).toBeNull();
	}, 20_000);

	it('shows the matched fragments, and never markup other than <mark>', () => {
		const page = fixture<MessagesProps>(URL);
		const rows = page.props.rows;
		show({ engine: ENGINE, backend: { used: 'engine' }, highlights: rows.map((_, i) => (i === 0 ? ['the <mark>kernel</mark> rc &amp; more'] : i === 1 ? ['<img src=x onerror=alert(1)><mark>kernel</mark>'] : [])) });
		const first = document.querySelector('.a-tablecard tbody tr .a-hl')!;
		expect(first.querySelector('mark')!.textContent).toBe('kernel');
		expect(first.textContent).toBe('the kernel rc & more');
		expect(document.querySelector('img[src="x"]')).toBeNull();
		// The unsafe one shows the plain snippet instead.
		const second = document.querySelectorAll('.a-tablecard tbody tr')[1]!;
		expect(second.querySelector('.a-hl')).toBeNull();
		expect(second.textContent).toContain(rows[1]!.snippet.slice(0, 10));
	});

	it('lists the chats with the most hits; one narrows the search to it', async () => {
		const { calls } = show({ engine: ENGINE, backend: { used: 'engine' }, facets: [{ chat_id: -1001, name: 'Rust &amp; friends', count: 1200 }, { chat_id: -1002, name: 'Other', count: 3 }] });
		const f = document.querySelector<HTMLElement>('.a-facets')!;
		expect(within(f).getAllByRole('button').map((b) => b.textContent)).toEqual(['Rust & friends1,200', 'Other3']);
		fireEvent.click(within(f).getByRole('button', { name: /Rust & friends/ }));
		await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\//).at(-1)?.query.get('f')).toBe(JSON.stringify([{ field: 'chat_id', op: '=', value: '-1001' }])));
	});
});

// The message lists' own text filter, with the search's full-text
// matches added when the list's registry offers it.
describe('text matches on the message lists', () => {
	const conditions = () => [...(screen.getByRole('combobox', { name: 'Condition' }) as HTMLSelectElement).options].map((o) => o.value);
	function filterText(url: string, extra: Partial<MessagesProps> = {}) {
		const calls = fakeApi('superadmin');
		const page = fixture<MessagesProps>(url);
		renderPage(url, 'superadmin', { props: { ...page.props, ...extra } });
		fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
		fireEvent.change(screen.getByRole('combobox', { name: 'Field' }), { target: { value: 'text' } });
		return { calls, page };
	}
	const apply = (op: string, value: string) => {
		fireEvent.change(screen.getByRole('combobox', { name: 'Condition' }), { target: { value: op } });
		fireEvent.change(screen.getByRole('textbox', { name: /Value/ }), { target: { value } });
		fireEvent.click(screen.getByRole('button', { name: 'Apply' }));
	};
	const lastF = (calls: ReturnType<typeof fakeApi>) => callsTo(calls, 'GET', /^\/api\/page\//).at(-1)?.query.get('f');

	for (const url of ['/group_messages', '/private_messages']) {
		it(`offers and applies text matches on ${url}`, async () => {
			const { calls } = filterText(url);
			expect(conditions()).toEqual(['contains', '=', 'empty', 'matches']);
			apply('matches', 'kernel rc');
			await waitFor(() => expect(lastF(calls)).toBe(JSON.stringify([{ field: 'text', op: 'matches', value: 'kernel rc' }])));
		});

		it(`keeps the plain text filter on ${url}`, async () => {
			const { calls } = filterText(url);
			apply('contains', 'kernel');
			await waitFor(() => expect(lastF(calls)).toBe(JSON.stringify([{ field: 'text', op: 'contains', value: 'kernel' }])));
		});
	}

	it('keeps the text field\'s own operators when the search lacks matches', () => {
		const p = fixture<MessagesProps>('/group_messages').props;
		filterText('/group_messages', { fields: p.fields!.map((f) => (f.key === 'text' ? { ...f, operators: f.operators.filter((o) => o !== 'matches') } : f)) });
		expect(conditions()).toEqual(['contains', '=', 'empty']);
	});
});

describe('engine helpers', () => {
	it('trust only text and <mark> in a fragment', () => {
		expect(safeFragment('a <mark>b</mark> &lt;c&gt;')).toBe('a <mark>b</mark> &lt;c&gt;');
		expect(safeFragment('<b>x</b>')).toBeNull();
		expect(safeFragment('<mark onclick=x>y</mark>')).toBeNull();
		expect(unescapeHtml('a &amp;lt; &lt;b&gt; &quot;c&quot; &#39;d&#39;')).toBe('a &lt; <b> "c" \'d\'');
	});
});
