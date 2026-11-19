import { describe, expect, it } from 'vitest';
import { contentLabel, serviceLabel, typeLabel } from './labels.ts';
import { CONTENT_TYPES } from './media.tsx';

describe('message type labels', () => {
	it('names every content type', () => {
		expect(contentLabel('video_note')).toBe('Video message');
		expect(contentLabel('paid_media')).toBe('Paid media');
		for (const t of CONTENT_TYPES) expect(contentLabel(t)).not.toMatch(/_/);
	});

	it('names a known service type and keeps an unknown one raw', () => {
		expect(serviceLabel('chat_add_members')).toBe('Members added');
		expect(serviceLabel('pin_message')).toBe('Message pinned');
		expect(serviceLabel('some_future_action')).toBe('some_future_action');
		// Not a key of the map's prototype either.
		expect(serviceLabel('constructor')).toBe('constructor');
	});

	it('names a service message by its action', () => {
		expect(typeLabel('service', 'chat_add_members')).toBe('Members added');
		expect(typeLabel('service', null)).toBe('Service message');
		expect(typeLabel('photo', 'chat_add_members')).toBe('Photo');
	});
});
