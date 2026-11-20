// The <head> elements the render service returns: title, meta and
// canonical link only, never scripts or styles (web-ssr.md 4.3).

export interface Head {
	title: string;
	description?: string;
	robots?: string;
	canonical?: string;
	og?: Record<string, string>;
}

export function escapeHtml(s: string): string {
	return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c] as string);
}

export function headHtml(h: Head): string {
	const out = [`<title>${escapeHtml(h.title)}</title>`];
	if (h.description) out.push(`<meta name="description" content="${escapeHtml(h.description)}">`);
	if (h.robots) out.push(`<meta name="robots" content="${escapeHtml(h.robots)}">`);
	if (h.canonical) out.push(`<link rel="canonical" href="${escapeHtml(h.canonical)}">`);
	for (const [k, v] of Object.entries(h.og || {})) out.push(`<meta property="og:${escapeHtml(k)}" content="${escapeHtml(v)}">`);
	return out.join('');
}

// The same head in the browser's document, for a page shown without a
// reload: the title, and the description, robots and canonical tags set
// to this page's, or removed when it has none. Open Graph tags are left
// as the server wrote them; only crawlers read them, and crawlers load
// each page whole.
export function applyHead(h: Head): void {
	document.title = h.title;
	const tag = (sel: string, make: () => HTMLElement, attr: string, value: string | undefined) => {
		let el = document.head.querySelector<HTMLElement>(sel);
		if (value === undefined) { el?.remove(); return; }
		if (!el) { el = make(); document.head.append(el); }
		el.setAttribute(attr, value);
	};
	const meta = (name: string) => () => Object.assign(document.createElement('meta'), { name });
	tag('meta[name="description"]', meta('description'), 'content', h.description);
	tag('meta[name="robots"]', meta('robots'), 'content', h.robots);
	tag('link[rel="canonical"]', () => Object.assign(document.createElement('link'), { rel: 'canonical' }), 'href', h.canonical);
}
