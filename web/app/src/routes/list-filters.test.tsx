// @vitest-environment happy-dom
// The message lists' kind filters: "Media" as the media kinds, one
// of a list (the `in` operator).
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { MEDIA, type MessagesProps } from './messages.tsx';

beforeEach(setup);

const withConds = (path: string, conds: unknown[]) => `${path}?f=${encodeURIComponent(JSON.stringify(conds))}`;
const lastConds = (calls: ReturnType<typeof fakeApi>) => JSON.parse(callsTo(calls, 'GET', /^\/api\/page\//).at(-1)!.query.get('f')!);

describe('the Media quick filter', () => {
	const media = [{ field: 'content_type', op: 'in', value: MEDIA }];

	it('asks for the media kinds, as one of a list', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/group_messages');
		fireEvent.click(screen.getByRole('button', { name: 'Media' }));
		await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(1));
		expect(lastConds(calls)).toEqual(media);
		expect(MEDIA).toEqual(['photo', 'video', 'video_note', 'document', 'audio', 'voice', 'sticker', 'animation', 'paid_media']);
	});

	it('returns media rows only: no service, poll or location', () => {
		const all = fixture<MessagesProps>('/group_messages?per=1000').props.rows.map((r) => r.content_type);
		// The sample has the kinds the old "not text" filter let through.
		for (const t of ['service', 'poll', 'location'] as const) expect(all).toContain(t);
		const got = fixture<MessagesProps>(withConds('/group_messages', media) + '&per=1000').props.rows.map((r) => r.content_type);
		expect(got.length).toBeGreaterThan(0);
		expect(got.every((t) => (MEDIA as string[]).includes(t))).toBe(true);
	});

	it('is shown on, with its kinds by their labels', () => {
		renderPage(withConds('/group_messages', media));
		expect(screen.getByRole('button', { name: 'Media' }).className).toContain('on');
		expect(document.querySelector('.a-chip')!.textContent).toContain('Photo, Video, Video message');
	});
});
