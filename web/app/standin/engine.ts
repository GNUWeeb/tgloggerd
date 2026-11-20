// The stand-in's search engine (web/docs/search-api.md, "Search
// engine"): off by default, so the lists behave as without one; POST
// /api/_standin/engine {"state": "available" | "behind" | "off"}
// switches it. Available, it offers the options and answers them:
// highlighted fragments of the searched words in a row's full-text
// field (a message's text, a user's bio, a group's description) and, for
// the message lists, the chats with the most hits. Behind, it is listed
// but MySQL answers, and the options asked for are ignored. Matching
// itself stays the stand-in's own (fuzzy changes nothing here).

export type EngineState = 'off' | 'available' | 'behind';
let state: EngineState = 'off';
export const engineState = () => state;
export function setEngine(s: EngineState): void { state = s; }

const OPTIONS = ['phrase', 'proximity', 'prefix', 'fuzzy', 'highlight', 'facets'] as const;
type Option = 'fuzzy' | 'highlight' | 'facets';

// The kinds of list the engine answers, by their full-text field.
export type EngineKind = 'messages' | 'users' | 'groups';
const FIELD: Record<EngineKind, string> = { messages: 'text', users: 'bio', groups: 'description' };

// The capabilities entry of a list's entity; undefined without an engine.
// Facets (hits per chat) mean nothing for users and groups.
export function engineOf(kind: EngineKind = 'messages'): { fields: string[]; options: typeof OPTIONS[number][]; available: boolean; fallback: string | null } | undefined {
	if (state === 'off') return undefined;
	return { fields: [FIELD[kind]], options: OPTIONS.filter((o) => kind === 'messages' || o !== 'facets'), available: state === 'available', fallback: state === 'available' ? null : 'behind' };
}

const escape = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
// The plain words of a matches value: no operators, phrases split.
const words = (q: string) => q.replace(/@\d+/g, ' ').replace(/[+\-~*"()<>]/g, ' ').split(/\s+/).filter((w) => w.length > 1).map((w) => w.toLowerCase());

// Up to three fragments of a text around the searched words, the words
// in <mark>, the rest escaped; none when no word occurs.
export function fragments(text: string, q: string): string[] {
	const ws = words(q);
	if (!ws.length || !text) return [];
	const re = new RegExp(`(${ws.map((w) => w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')})`, 'gi');
	if (!re.test(text)) return [];
	const parts = text.split(re);
	return [parts.map((p, i) => (i % 2 ? `<mark>${escape(p)}</mark>` : escape(p))).join('')];
}

type Row = { content_type?: string; text?: string; chat: { id: number; title?: string; name?: string } };

// What the engine adds to a message list's page data.
export function engineAnswer(q: URLSearchParams, text: string, page: Row[], matched: () => Row[]) {
	return answer('messages', q, text, page, (r) => (r.content_type === 'service' ? '' : r.text ?? ''), matched);
}

// What the engine adds to the users' or groups' page data: text is the
// values of the bio (description) matches conditions.
export function profileAnswer(kind: 'users' | 'groups', q: URLSearchParams, text: string, page: Record<string, unknown>[]) {
	return answer(kind, q, text, page, (r) => String(r[FIELD[kind]] ?? ''));
}

function answer<R>(kind: EngineKind, q: URLSearchParams, text: string, page: R[], field: (r: R) => string, matched?: () => Row[]) {
	const engine = engineOf(kind);
	if (!engine) return {};
	const asked = (['fuzzy', 'highlight', 'facets'] as Option[]).filter((o) => q.get(o) === '1');
	// Nothing for the engine to answer without a text search.
	if (!text) return { engine };
	if (!engine.available) return { engine, backend: { used: 'mysql', fallback: 'behind', ...(asked.length ? { ignored: asked } : {}) } };
	// An option the entity lacks is left out, and the answer says so.
	const left = asked.filter((o) => !engine.options.includes(o));
	const out: Record<string, unknown> = { engine, backend: { used: 'engine', ...(left.length ? { ignored: left } : {}) } };
	if (asked.includes('highlight')) out.highlights = page.map((r) => fragments(field(r), text));
	if (matched && asked.includes('facets')) {
		const by = new Map<number, { chat_id: number; name: string; count: number }>();
		for (const r of matched()) {
			const f = by.get(r.chat.id) ?? { chat_id: r.chat.id, name: escape(r.chat.title ?? r.chat.name ?? String(r.chat.id)), count: 0 };
			f.count++;
			by.set(r.chat.id, f);
		}
		out.facets = [...by.values()].sort((a, b) => b.count - a.count).slice(0, 10);
	}
	return out;
}
