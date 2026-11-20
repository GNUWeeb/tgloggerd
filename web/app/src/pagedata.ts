import type { PageData } from './page.ts';

// The page data is a JSON script (type="application/json"), read with
// JSON.parse, never evaluated (web-ssr.md section 5).
export function readPageData(doc: Document): PageData | null {
	const el = doc.getElementById('__tgd');
	if (!el?.textContent) return null;
	return JSON.parse(el.textContent) as PageData;
}
