// The forwarding routes' content types as the app presents them: labels,
// groups, presets and the route list's summary (decision
// docs/design/decisions/2026-11-20-fwd-more-types.md). The service sends
// only the set and each direction's options; all of this is the app's.
import type { S } from '../api/schema.ts';

type FwdType = S['T2DType'] | S['D2TType'];

// Every type has a label: a new one fails to compile here.
const LABEL: Record<FwdType, string> = {
	text: 'Text', photo: 'Photos', video: 'Videos', animation: 'GIFs', video_note: 'Video messages', document: 'Files', sticker: 'Stickers',
	voice: 'Voice messages', audio: 'Music', poll: 'Polls', checklist: 'Checklists', dice: 'Dice', game: 'Games',
	location: 'Locations', contact: 'Contacts', story: 'Stories', paid_media: 'Paid media', invoice: 'Invoices',
	service: 'Service messages', other: 'Everything else',
	// Discord to Telegram only.
	embed: 'Embeds', system: 'System messages',
};

// A type the app does not know yet shows its raw name.
export const typeLabel = (t: string): string => (LABEL as Record<string, string>)[t] ?? t;

type GroupDef = { id: string; name: string; types: string[]; hint?: string };

const GROUPS: GroupDef[] = [
	{ id: 'text', name: 'Text', types: ['text'] },
	{ id: 'media', name: 'Media', types: ['photo', 'video', 'animation', 'video_note', 'document', 'sticker'] },
	{ id: 'audio', name: 'Audio', types: ['voice', 'audio'] },
	{ id: 'polls', name: 'Polls and games', types: ['poll', 'checklist', 'dice', 'game'] },
	{ id: 'places', name: 'Places and contacts', types: ['location', 'contact'] },
	{ id: 'extras', name: 'Telegram extras', types: ['story', 'paid_media', 'invoice'] },
	{ id: 'embeds', name: 'Embeds', types: ['embed'], hint: 'Link previews and the cards bots post.' },
	{ id: 'service', name: 'Service messages', types: ['service'], hint: 'Joins, pins, title changes and the like.' },
	{ id: 'system', name: 'System messages', types: ['system'], hint: 'Discord\'s joins, pins, boosts and the like.' },
	{ id: 'other', name: 'Everything else', types: ['other'], hint: 'Rich messages, calls, giveaways and kinds added later.' },
];
const UNKNOWN = 'unknown';

// list: draw the types even when there is only one (their names are
// all the app has for them).
export type Group = { id: string; name: string; hint?: string; types: string[]; list?: boolean };

// The direction's groups: only its options, empty groups left out, and
// the options the app does not know under "Other kinds".
export function groupsOf(options: readonly string[]): Group[] {
	const known = new Set(GROUPS.flatMap((g) => g.types));
	const out: Group[] = GROUPS.map((g) => ({ ...g, types: g.types.filter((t) => options.includes(t)) })).filter((g) => g.types.length);
	const rest = options.filter((t) => !known.has(t));
	if (rest.length) out.push({ id: UNKNOWN, name: 'Other kinds', types: rest, list: true });
	return out;
}

export type Preset = { id: string; name: string; types: string[] };

// Messages and media leave out the service and system messages, the
// Telegram extras and everything else.
const MESSAGES_AND_MEDIA = new Set(['text', 'media', 'audio', 'polls', 'places', 'embeds']);

// The presets, each limited to the direction's options; one that comes
// out the same as an earlier one or empty is left out.
export function presetsOf(options: readonly string[]): Preset[] {
	const gs = groupsOf(options);
	const all: Preset[] = [
		{ id: 'all', name: 'Everything', types: [...options] },
		{ id: 'media', name: 'Messages and media', types: gs.filter((g) => MESSAGES_AND_MEDIA.has(g.id)).flatMap((g) => g.types) },
		{ id: 'text', name: 'Text only', types: options.filter((t) => t === 'text') },
	];
	return all.filter((p, i) => p.types.length && !all.slice(0, i).some((q) => same(q.types, p.types)));
}

function same(a: readonly string[], b: readonly string[]) {
	return a.length === b.length && a.every((t) => b.includes(t));
}

// The preset a set equals, if any.
export function presetOf(options: readonly string[], set: readonly string[]): Preset | null {
	return presetsOf(options).find((p) => same(p.types, set)) ?? null;
}

// A set in option order, as the service stores it.
export function ordered<K extends string>(options: readonly K[], set: readonly K[]): K[] {
	return options.filter((t) => set.includes(t));
}

// The route list's one-line summary, and the full list for its title.
export function summaryOf(options: readonly string[], set: readonly string[]): { text: string; title: string } {
	const on = options.filter((t) => set.includes(t));
	const title = on.map(typeLabel).join(', ');
	if (!on.length) return { text: 'Nothing', title };
	if (on.length === options.length) return { text: 'All content', title };
	const p = presetOf(options, on);
	if (p) return { text: p.name, title };
	const off = options.filter((t) => !set.includes(t));
	if (off.length <= 3) return { text: `All except ${off.map(typeLabel).join(', ')}`, title };
	const parts = groupsOf(options).map((g) => {
		const n = g.types.filter((t) => set.includes(t)).length;
		return !n ? null : n === g.types.length ? g.name : `${g.name} (${n}/${g.types.length})`;
	});
	return { text: parts.filter(Boolean).join(', '), title };
}
