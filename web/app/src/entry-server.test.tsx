import { describe, expect, it } from 'vitest';
import { render, BUILD } from './entry-server.tsx';
import { headHtml } from './head.ts';
import type { PageData } from './page.ts';

const page = (route: string): PageData => ({ v: 1, route, url: '/x', status: 404, props: {}, viewer: { signed_in: false, now: '2026-11-18T00:00:00Z' } });

describe('server entry', () => {
	it('carries the build id', () => {
		expect(BUILD).toBe('test');
	});
	it('renders a known route with its head', () => {
		const r = render(page('not_found'));
		expect(r.html).toContain('404');
		expect(r.head).toContain('<meta name="robots" content="noindex">');
	});
	it('throws UnknownRoute for an unknown route', () => {
		expect(() => render(page('no_such_route'))).toThrow(expect.objectContaining({ name: 'UnknownRoute' }));
	});
	it('escapes head values', () => {
		expect(headHtml({ title: '</title><script>x</script>', description: '"a" & b' }))
			.toBe('<title>&lt;/title&gt;&lt;script&gt;x&lt;/script&gt;</title><meta name="description" content="&quot;a&quot; &amp; b">');
	});
});
