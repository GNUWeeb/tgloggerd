// The route list's summary of a forwarding route's content types.
import { describe, expect, it } from 'vitest';
import { summaryOf } from './fwdtypes.ts';

const T2D = ['text', 'photo', 'video', 'animation', 'video_note', 'document', 'sticker', 'voice', 'audio', 'poll', 'checklist', 'dice', 'game', 'location', 'contact', 'story', 'paid_media', 'invoice', 'service', 'other'];
const D2T = ['text', 'photo', 'video', 'animation', 'document', 'sticker', 'audio'];
const but = (opts: string[], ...off: string[]) => opts.filter((t) => !off.includes(t));
const text = (opts: string[], set: string[]) => summaryOf(opts, set).text;

describe('forwarding type summary', () => {
	it('says All content for every option, and a preset by its name', () => {
		expect(text(T2D, T2D)).toBe('All content');
		expect(text(D2T, D2T)).toBe('All content');
		expect(text(T2D, ['text'])).toBe('Text only');
		expect(text(T2D, but(T2D, 'story', 'paid_media', 'invoice', 'service', 'other'))).toBe('Messages and media');
	});

	it('names what is off when at most three are', () => {
		expect(text(T2D, but(T2D, 'sticker', 'service'))).toBe('All except Stickers, Service messages');
		expect(text(T2D, but(T2D, 'sticker', 'voice', 'other'))).toBe('All except Stickers, Voice messages, Everything else');
		expect(text(D2T, but(D2T, 'audio'))).toBe('All except Music');
	});

	it('lists the groups on otherwise, a partial one with its count', () => {
		expect(text(T2D, ['text', 'photo', 'video', 'document', 'sticker', 'voice', 'audio'])).toBe('Text, Media (4/6), Audio');
		expect(text(T2D, ['photo', 'service'])).toBe('Media (1/6), Service messages');
	});

	it('gives the full list in the title, an unknown type by its name', () => {
		expect(summaryOf(T2D, ['text', 'photo']).title).toBe('Text, Photos');
		expect(summaryOf([...D2T, 'hologram'], ['text', 'hologram']).title).toBe('Text, hologram');
		expect(text([...D2T, 'hologram'], ['text', 'photo', 'hologram'])).toBe('Text, Media (1/5), Other kinds');
	});
});
