// The search engine's options on the lists it answers (the messages'
// text, users' bio, groups' description), offered only when the engine
// is configured and available for the list (EngineProps.engine): typo
// tolerance, highlighted matches, the chats with the most hits (message
// lists only), and a short help on the search syntax it reads. Each
// option is a query parameter of the list's URL (fuzzy=1, highlight=1,
// facets=1), so it survives a reload and a link. Without an engine, or
// while it is unavailable, nothing here shows and the lists behave as
// before.
import { useState } from 'react';
import { CircleHelp, Info, MessagesSquare } from 'lucide-react';
import { setQuery, useNav } from '../nav.tsx';
import { fmt } from '../format.ts';
import { safeFragment, unescapeHtml, type Backend, type EngineOption, type EngineProps, type Facet } from '../api/engine.ts';
import type { Cond } from '../types.ts';

// What a list's rows are called in the hints: each message, ...
export type Noun = { one: string; many: string };
const MESSAGES: Noun = { one: 'message', many: 'messages' };

const TOGGLES: [EngineOption & ('fuzzy' | 'highlight' | 'facets'), string, (n: Noun) => string][] = [
	['fuzzy', 'Allow typos', () => 'Words also match with a letter or two wrong, as word~ does for one word.'],
	['highlight', 'Highlight matches', (n) => `Show the matching parts of each ${n.one}.`],
	['facets', 'Chats with the most hits', () => 'List the chats this search finds most often.'],
];

// The syntax the engine reads, by the option that offers it.
const SYNTAX: [EngineOption | 'always', string, string | ((n: Noun) => string)][] = [
	['always', 'word', (n) => `${n.many} with the word rank higher`],
	['always', '+word  -word', 'must have it / must not have it'],
	['phrase', '"a phrase"', 'the words in this order, next to each other'],
	['proximity', '"a b" @3', 'the words within 3 positions of each other'],
	['prefix', 'word*', 'a word starting so'],
	['fuzzy', 'word~', 'the word, or one a letter or two off'],
];

// The users and groups lists' search box is not the engine's; only a
// bio or description matches filter is.
export const MATCHES_HINT = 'works with a matches filter';

export const offered = (p: EngineProps) => (p.engine?.available ? p.engine.options : []);

// facets: false on the lists without chats to count hits by (users,
// groups), whatever the capabilities say. hint: how to reach the engine
// on a list whose search box does not.
export function EngineOptions({ p, noun = MESSAGES, facets = true, hint }: { p: EngineProps; noun?: Noun; facets?: boolean; hint?: string }) {
	const { page, navigate } = useNav();
	const [help, setHelp] = useState(false);
	const opts = offered(p).filter((o) => facets || o !== 'facets');
	if (!opts.length) return null;
	const q = new URL(page.url, 'http://x').searchParams;
	const toggles = TOGGLES.filter(([k]) => opts.includes(k));
	const set = (k: string, on: boolean) => navigate(setQuery(page.url, { [k]: on ? '1' : undefined, ...(k === 'fuzzy' ? { page: undefined } : {}) }), { replace: true, scroll: false });
	return (
		<div className="a-engine">
			<div className="a-engine-row" role="group" aria-label="Search options">
				{toggles.map(([k, label, hint]) => (
					<label key={k} className={`a-checkchip ${q.get(k) === '1' ? 'on' : ''}`} title={hint(noun)}>
						<input type="checkbox" checked={q.get(k) === '1'} onChange={(e) => set(k, e.target.checked)} />{label}
					</label>
				))}
				{hint && <span className="a-muted sm">{hint}</span>}
				<button type="button" className="a-btn a-btn-ghost a-engine-help" aria-expanded={help} onClick={() => setHelp((x) => !x)}><CircleHelp size={14} />Search syntax</button>
			</div>
			{help && (
				<dl className="a-engine-syntax">
					{SYNTAX.filter(([o]) => o === 'always' || opts.includes(o)).map(([, form, meaning]) => (
						<div key={form}><dt className="mono">{form}</dt><dd>{typeof meaning === 'string' ? meaning : meaning(noun)}</dd></div>
					))}
				</dl>
			)}
		</div>
	);
}

const FALLBACK: Record<NonNullable<Backend['fallback']>, (n: Noun) => string> = {
	asked: () => 'This search ran on the database, as asked.',
	unavailable: () => 'The search engine cannot be reached just now, so the database answered this search.',
	timeout: () => 'The search engine took too long, so the database answered this search.',
	behind: () => 'The search engine is still catching up with the archive, so the database answered this search.',
	too_many_hits: (n) => `This search matches too many ${n.many} for the search engine to list, so the database answered it.`,
};
const OPTION_NAME = { fuzzy: 'typo tolerance', highlight: 'highlighted matches', facets: 'the chats with the most hits' } as const;
// Typo tolerance is one thing; the others are plural.
const cap = (s: string) => s[0]!.toUpperCase() + s.slice(1);

// Which backend answered, when that changes what the page shows: the
// database instead of the engine, or options left out.
export function BackendNote({ b, noun = MESSAGES }: { b?: Backend; noun?: Noun }) {
	if (!b || (!b.fallback && !b.ignored?.length)) return null;
	const ignored = b.ignored?.map((o) => OPTION_NAME[o]) ?? [];
	return (
		<div className="a-banner" role="status">
			<Info size={15} />
			<span>
				{b.fallback ? FALLBACK[b.fallback](noun) : null}
				{ignored.length > 0 && <>{b.fallback ? ' ' : ''}{cap(ignored.length > 1 ? `${ignored.slice(0, -1).join(', ')} and ${ignored.at(-1)}` : ignored[0]!)} {ignored.length > 1 || b.ignored![0] !== 'fuzzy' ? 'are' : 'is'} not available for this search.</>}
			</span>
		</div>
	);
}

// The chats with the most hits; one opens the search in that chat.
export function Facets({ facets, conds, onConds }: { facets?: Facet[]; conds: Cond[]; onConds: (c: Cond[]) => void }) {
	if (!facets?.length) return null;
	const inChat = conds.some((c) => c.field === 'chat_id' && c.op === '=');
	return (
		<div className="a-facets" aria-label="Chats with the most hits">
			<span className="a-muted sm"><MessagesSquare size={13} /> Most hits in</span>
			{facets.map((f) => (
				<button key={f.chat_id} type="button" className="a-facet" disabled={inChat} onClick={() => onConds([...conds, { field: 'chat_id', op: '=', value: String(f.chat_id) }])}
					title={inChat ? undefined : 'Search only in this chat'}>
					<span className="a-ellipsis">{unescapeHtml(f.name)}</span><span className="num">{fmt(f.count)}</span>
				</button>
			))}
		</div>
	);
}

// A row's matched fragments, as the server marked them; null when it
// sent none, or none that is only text and <mark>.
export function Highlighted({ fragments }: { fragments?: string[] }) {
	const safe = (fragments ?? []).map(safeFragment).filter((x): x is string => x !== null);
	if (!safe.length) return null;
	return <span className="a-hl" dangerouslySetInnerHTML={{ __html: safe.join(' <span class="a-hl-gap">...</span> ') }} />;
}
