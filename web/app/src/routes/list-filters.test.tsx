// @vitest-environment happy-dom
// The message lists' kind filters: "Media" as the media kinds, one
// of a list (the `in` operator), and service rows in private chats.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { MEDIA, type MessagesProps } from './messages.tsx';

beforeEach(setup);

const withConds = (path: string, conds: unknown[]) => `${path}?f=${encodeURIComponent(JSON.stringify(conds))}`;
const valueSelect = () => screen.getAllByRole('combobox').find((x) => x.closest('label')?.textContent?.startsWith('Value')) as HTMLSelectElement;
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

describe('the private messages list', () => {
	it('filters by service', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/private_messages');
		fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
		fireEvent.change(screen.getByRole('combobox', { name: 'Field' }), { target: { value: 'content_type' } });
		expect([...valueSelect().options].map((o) => [o.value, o.textContent])).toContainEqual(['service', 'Service message']);
		fireEvent.change(valueSelect(), { target: { value: 'service' } });
		fireEvent.click(screen.getByRole('button', { name: 'Apply' }));
		await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(1));
		expect(lastConds(calls)).toEqual([{ field: 'content_type', op: '=', value: 'service' }]);
		const rows = fixture<MessagesProps>(withConds('/private_messages', lastConds(calls))).props.rows;
		expect(rows.length).toBeGreaterThan(0);
		expect(rows.every((r) => r.content_type === 'service')).toBe(true);
	});
});
