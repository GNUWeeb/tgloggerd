// The search engine's part of a message list (web/docs/search-api.md,
// "Search engine"): what it offers for the list (engine), which backend
// answered (backend), and with the options asked for, the matched
// fragments of each row (highlights) and the chats with the most hits
// (facets). All absent without an engine.
import type { S } from './schema.ts';

export type EngineEntity = S['SearchEngineEntity'];
export type Backend = S['SearchBackend'];
export type Facet = S['SearchFacet'];
export type EngineOption = EngineEntity['options'][number];

export type EngineProps = Pick<S['MessagesProps'], 'engine' | 'backend' | 'highlights' | 'facets'>;

// A highlight fragment as the server sends it: HTML-escaped text with
// the matches in <mark>. Anything else in it is not trusted as markup:
// the fragment is shown as plain text then.
export function safeFragment(html: string): string | null {
	const stripped = html.replace(/<\/?mark>/g, '');
	return /[<>]/.test(stripped) ? null : html;
}

// A name the server HTML-escaped (a facet's), for React to escape again.
export function unescapeHtml(s: string): string {
	return s.replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&quot;/g, '"').replace(/&#39;/g, "'").replace(/&amp;/g, '&');
}
