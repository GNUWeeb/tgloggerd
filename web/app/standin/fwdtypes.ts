// The forwarding content types each direction's bridge honours, in the
// order the route form lists them (T2DProps / D2TProps type_options),
// as the Go service sends them.
import type { S } from '../src/api/schema.ts';

export const T2D_OPTIONS: S['T2DType'][] = ['text', 'photo', 'video', 'animation', 'video_note', 'document', 'sticker', 'voice', 'audio', 'poll', 'checklist', 'dice', 'game', 'location', 'contact', 'story', 'paid_media', 'invoice', 'service', 'other'];
export const D2T_OPTIONS: S['D2TType'][] = ['text', 'photo', 'video', 'animation', 'document', 'sticker', 'audio'];

// The six types a route had before; each new one copies the flag it
// followed until then, as the migration does: every new Telegram to
// Discord type followed other, Discord to Telegram audio followed
// document and animation followed photo.
const T2D_OLD = new Set(['text', 'photo', 'video', 'document', 'sticker', 'other']);
const D2T_FROM: Record<string, string> = { audio: 'document', animation: 'photo' };

export function migrated(dir: 't2d' | 'd2t', types: string[]): string[] {
	const has = new Set(types);
	if (dir === 't2d') return T2D_OPTIONS.filter((t) => has.has(t) || (!T2D_OLD.has(t) && has.has('other')));
	return D2T_OPTIONS.filter((t) => has.has(t) || (t in D2T_FROM && has.has(D2T_FROM[t]!)));
}

// A written set: unknown types and an empty set are refused; the stored
// set is in option order. Returns the error answer or the set.
export function checkTypes(dir: 't2d' | 'd2t', types: unknown): { error: [number, unknown] } | { types: string[] } {
	const opts: string[] = dir === 't2d' ? T2D_OPTIONS : D2T_OPTIONS;
	if (!Array.isArray(types)) return { error: [400, { error: 'bad_request', message: 'types must be a list.' }] };
	const bad = types.find((t) => !opts.includes(t));
	if (bad !== undefined) return { error: [400, { error: 'bad_request', message: `Unknown content type: ${String(bad)}.` }] };
	if (!types.length) return { error: [400, { error: 'bad_request', message: 'Choose at least one kind of message.' }] };
	return { types: opts.filter((t) => types.includes(t)) };
}
