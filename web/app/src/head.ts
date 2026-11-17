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
