// The HTML document around the rendered markup, as the Go service
// assembles it (docs/design/web-ssr.md section 5). The stand-in uses it
// in development; it is also the reference for go-web-pages.
import type { PageData } from '../src/page.ts';

// JSON for an inline <script type="application/json">: the same escapes
// as Go's encoding/json with HTML escaping on, so no string can close the
// element or break JavaScript parsing.
export function scriptJson(value: unknown): string {
	return JSON.stringify(value)
		.replace(/</g, '\\u003c')
		.replace(/>/g, '\\u003e')
		.replace(/&/g, '\\u0026')
		.replace(/\u2028/g, '\\u2028')
		.replace(/\u2029/g, '\\u2029');
}

function attr(s: string): string {
	return s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;');
}

export interface Assets {
	css: string[];
	preload: string[];
	entry: string;
}

export function documentHtml(opts: { page: PageData; head: string | null; html: string | null; csrf?: string; assets: Assets }): string {
	const { page, head, html, csrf, assets } = opts;
	const theme = page.viewer.theme && page.viewer.theme !== 'system' ? ` data-theme="${attr(page.viewer.theme)}"` : '';
	const ssr = html !== null;
	return [
		'<!doctype html>',
		`<html lang="${attr(page.viewer.lang || 'en')}"${theme}>`,
		'<head>',
		'<meta charset="utf-8">',
		'<meta name="viewport" content="width=device-width, initial-scale=1">',
		csrf ? `<meta name="csrf-token" content="${attr(csrf)}">` : '',
		head ?? '<title>tgloggerd</title>',
		...assets.css.map((h) => `<link rel="stylesheet" href="${attr(h)}">`),
		...assets.preload.map((h) => `<link rel="modulepreload" href="${attr(h)}">`),
		'</head>',
		'<body>',
		`<div id="root" data-ssr="${ssr ? 1 : 0}">${html ?? ''}</div>`,
		`<script type="application/json" id="__tgd">${scriptJson(page)}</script>`,
		`<script type="module" src="${attr(assets.entry)}"></script>`,
		'</body>',
		'</html>',
	].filter(Boolean).join('\n');
}
