// The render entry the render service loads: page data in, markup and
// head out (web-ssr.md 4.2 and 4.3).
import { renderToString } from 'react-dom/server';
import App, { routeOf } from './App.tsx';
import { headHtml } from './head.ts';
import type { PageData, RenderResult } from './page.ts';

export const BUILD: string = __TGD_BUILD__;

export function render(page: PageData): RenderResult {
	const route = routeOf(page); // throws UnknownRoute
	return { head: headHtml(route.head(page)), html: renderToString(<App page={page} />) };
}
