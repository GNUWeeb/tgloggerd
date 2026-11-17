import { describe, expect, it } from 'vitest';
import { documentHtml, scriptJson } from './document.ts';
import type { PageData } from '../src/page.ts';

const page: PageData = { v: 1, route: 'users', url: '/users', status: 200, props: { name: '</script><script>alert(1)</script> & \u2028' }, viewer: { signed_in: true, now: '2026-11-18T00:00:00Z', theme: 'dark' } };

describe('document assembly', () => {
	it('escapes the page data like encoding/json', () => {
		const s = scriptJson(page.props);
		expect(s).not.toContain('<');
		expect(s).not.toContain('>');
		expect(s).not.toContain('&');
		expect(s).toContain('\\u003c/script\\u003e');
		expect(s).toContain('\\u2028');
		expect(JSON.parse(s)).toEqual(page.props);
	});
	it('marks the fallback with data-ssr="0" and keeps the data', () => {
		const d = documentHtml({ page, head: null, html: null, assets: { css: [], preload: [], entry: '/app/e.js' } });
		expect(d).toContain('<div id="root" data-ssr="0"></div>');
		expect(d).toContain('id="__tgd"');
		expect(d).toContain('data-theme="dark"');
	});
	it('embeds the rendered markup and head', () => {
		const d = documentHtml({ page, head: '<title>x</title>', html: '<p>y</p>', csrf: 't"k', assets: { css: ['/app/a.css'], preload: [], entry: '/app/e.js' } });
		expect(d).toContain('<div id="root" data-ssr="1"><p>y</p></div>');
		expect(d).toContain('<meta name="csrf-token" content="t&quot;k">');
		expect(d).toContain('<link rel="stylesheet" href="/app/a.css">');
	});
});
