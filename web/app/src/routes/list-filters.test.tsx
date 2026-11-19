// @vitest-environment happy-dom
// The lists' kind filters: "Media" as the media kinds, one of a list
// (the `in` operator), service rows in private chats, the service and
// TDLib types as lists, and every file type on /files.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { MEDIA, type MessagesProps } from './messages.tsx';
import type { FilesProps } from './files.tsx';

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

describe('the service and TDLib type filters', () => {
	type Row = MessagesProps['rows'][number] & { service_type?: string };
	const services = fixture<MessagesProps>(withConds('/group_messages', [{ field: 'content_type', op: '=', value: 'service' }]) + '&per=1000').props.rows as Row[];
	const added = services.filter((r) => r.service_type === 'chat_add_members');
	const chat = added[0]!.chat.id;
	const inChat = withConds('/group_messages', [{ field: 'chat_id', op: '=', value: String(chat) }]);
	const pick = (key: string) => {
		fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
		fireEvent.change(screen.getByRole('combobox', { name: 'Field' }), { target: { value: key } });
	};

	it('offer the service types as a list with labels, and find "Members added"', async () => {
		const calls = fakeApi('superadmin');
		renderPage(inChat);
		pick('service_type');
		const opts = [...valueSelect().options].map((o) => [o.value, o.textContent]);
		expect(opts).toContainEqual(['chat_add_members', 'Members added']);
		expect(opts).toContainEqual(['pin_message', 'Message pinned']);
		fireEvent.change(valueSelect(), { target: { value: 'chat_add_members' } });
		fireEvent.click(screen.getByRole('button', { name: 'Apply' }));
		await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(1));
		const conds = lastConds(calls);
		expect(conds.at(-1)).toEqual({ field: 'service_type', op: '=', value: 'chat_add_members' });
		const rows = fixture<MessagesProps>(withConds('/group_messages', conds)).props.rows as Row[];
		expect(rows.map((r) => r.id).sort()).toEqual(added.filter((r) => r.chat.id === chat).map((r) => r.id).sort());
	});

	it('show a service type filter by its label', () => {
		renderPage(withConds('/group_messages', [{ field: 'chat_id', op: '=', value: String(chat) }, { field: 'service_type', op: '=', value: 'chat_add_members' }]));
		expect(document.querySelectorAll('.a-chip')[1]!.textContent).toContain('Members added');
	});

	it('offer the TDLib types as a list of their class names', () => {
		fakeApi('superadmin');
		renderPage(inChat);
		pick('tdlib_type');
		expect([...valueSelect().options].map((o) => o.textContent)).toContain('messageStakeDice');
	});
});

describe('the files list', () => {
	it('offers video notes and thumbnails in its type filter', () => {
		fakeApi('superadmin');
		renderPage('/files?view=list');
		fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
		fireEvent.change(screen.getByRole('combobox', { name: 'Field' }), { target: { value: 'file_type' } });
		const values = [...valueSelect().options].map((o) => o.value);
		expect(values).toContain('video_note');
		expect(values).toContain('thumbnail');
	});

	it('finds thumbnails, which it shows as photos', () => {
		const rows = fixture<FilesProps>(withConds('/files', [{ field: 'file_type', op: '=', value: 'thumbnail' }])).props.rows;
		expect(rows.length).toBeGreaterThan(0);
		expect(rows.every((f) => f.file_type === 'photo')).toBe(true);
	});

	for (const [name, type] of [['Voice', 'voice'], ['Audio', 'audio'], ['Stickers', 'sticker'], ['Animations', 'animation'], ['Video notes', 'video_note']]) {
		it(`filters ${type} files with its quick filter`, async () => {
			const calls = fakeApi('superadmin');
			renderPage('/files?view=list');
			fireEvent.click(screen.getByRole('button', { name: name! }));
			await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(1));
			const conds = lastConds(calls);
			expect(conds).toEqual([{ field: 'file_type', op: '=', value: type }]);
			const rows = fixture<FilesProps>(withConds('/files', conds)).props.rows;
			expect(rows.length).toBeGreaterThan(0);
			expect(rows.every((f) => f.file_type === type)).toBe(true);
		});
	}
});
