// @vitest-environment happy-dom
// What an earlier version of a message held besides its text (its
// detail), in the chat's edit list and the message page's edit history:
// a live location's positions with their times, a checklist's tasks,
// another kind's card; and a long list collapsed to its latest versions.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, within } from '@testing-library/react';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { KINDS_GROUP, sampleOf } from '../../standin/kinds.ts';
import { chatStream } from '../../../mockup/src/data/messages.js';
import { EDITS_SHOWN } from '../ui/editdetail.tsx';
import type { ChatProps } from './chat.tsx';
import type { MessageProps } from './message.tsx';
import type { ChatMessage } from '../types.ts';

beforeEach(setup);

const url = `/groups/${KINDS_GROUP.id}/chat`;
const chatPage = () => fixture<ChatProps>(url);
const liveOf = (msgs: ChatMessage[]) => msgs.find((m) => m.detail?.kind === 'location' && !!m.detail.live)!;
const checklistOf = (msgs: ChatMessage[]) => msgs.find((m) => m.detail?.kind === 'checklist')!;
const bubbleOf = (m: { message_id: number }) => document.getElementById(`m${m.message_id}`)!;
const openEdits = (m: { message_id: number }) => {
	const b = bubbleOf(m);
	fireEvent.click(within(b).getByRole('button', { name: /edited/ }));
	return b.querySelector('.c-edits') as HTMLElement;
};
const mapLinks = (el: Element) => [...el.querySelectorAll<HTMLAnchorElement>('.e-loc a')];
// The coordinates of a version, as the list shows them.
const coordsOf = (e: ChatMessage['edits'][number]) => {
	const d = e.detail;
	if (d?.kind !== 'location') throw new Error('not a location version');
	return `${d.lat.toFixed(5)}, ${d.lon.toFixed(5)}`;
};
const hm = (iso: string) => new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit', timeZone: chatPage().viewer.timezone || 'Asia/Jakarta' }).format(new Date(iso));

describe('the detail of an earlier version in the chat', () => {
	it('shows each earlier position of a live location with its time, linked to the map', () => {
		fakeApi('superadmin');
		const m = liveOf(chatPage().props.messages);
		renderPage(url);
		expect(m.edits.length).toBeGreaterThan(EDITS_SHOWN);
		const list = openEdits(m);
		// Collapsed to the latest versions, the earlier ones behind a button.
		const latest = m.edits.slice(-EDITS_SHOWN);
		expect(mapLinks(list).map((a) => a.textContent)).toEqual(latest.map(coordsOf));
		const row = mapLinks(list)[0]!.closest('.c-edits > div')!;
		expect(row.textContent).toContain(hm(latest[0]!.at));
		expect(mapLinks(list)[0]!.getAttribute('href')).toContain('openstreetmap.org');
		expect(list.textContent).not.toContain('(no text)');
		fireEvent.click(within(list).getByRole('button', { name: `Show ${m.edits.length - EDITS_SHOWN} more` }));
		expect(mapLinks(list).map((a) => a.textContent)).toEqual(m.edits.map(coordsOf));
		expect(within(list).queryByRole('button', { name: /Show/ })).toBeNull();
	});

	it('shows an earlier version of a checklist as its tasks', () => {
		fakeApi('superadmin');
		const m = checklistOf(chatPage().props.messages);
		renderPage(url);
		const list = openEdits(m);
		const tasks = [...list.querySelectorAll('.e-detail li')].map((li) => li.textContent);
		expect(tasks).toEqual(['Tag the release', 'Write the changelog']);
		expect(within(list).getAllByLabelText('not done')).toHaveLength(2);
		expect(within(list).queryByRole('button', { name: /Show/ })).toBeNull();
	});

	it('draws another kind\'s earlier version with its card', () => {
		const p = chatPage().props;
		const msgs = [...p.messages];
		const venue = { kind: 'venue' as const, lat: -6.1754, lon: 106.8272, title: 'Old meeting point', address: 'Somewhere', provider: '', venue_id: '', venue_type: '' };
		msgs[2] = { ...msgs[2]!, content_type: 'venue', edits: [{ at: '2026-11-17T09:00:00Z', text: '', entities: [], content_type: 'venue', detail: venue }], edited_at: '2026-11-17T09:05:00Z', deleted_at: null };
		fakeApi('superadmin');
		renderPage(url, 'superadmin', { props: { ...p, messages: msgs } });
		const list = openEdits(msgs[2]!);
		expect(within(list).getByText('Old meeting point')).toBeTruthy();
		expect(list.textContent).not.toContain('(no text)');
	});
});

type Raw = { id: number; message_id: number; chat: { id: number; kind: string } };
const sampleId = (pick: (s: NonNullable<ReturnType<typeof sampleOf>>) => boolean) => (chatStream(KINDS_GROUP) as Raw[]).find((m) => { const s = sampleOf(m); return !!s && pick(s); })!.id;
const openMessage = (id: number) => {
	fakeApi('superadmin');
	renderPage(`/messages/group/${id}`);
	return fixture<MessageProps>(`/messages/group/${id}`).props.message;
};
const history = () => screen.getByText('Edit history').closest('.a-card') as HTMLElement;

describe('the detail of an earlier version on the message page', () => {
	it('shows each earlier position of a live location with its time, the list collapsed', () => {
		const m = openMessage(sampleId((s) => s.content_type === 'location' && !!s.edits));
		const h = history();
		const latest = m.edits.slice(-EDITS_SHOWN);
		expect(mapLinks(h).map((a) => a.textContent)).toEqual(latest.map(coordsOf));
		const item = mapLinks(h)[0]!.closest('.a-tl')!;
		expect(item.querySelector('time')!.getAttribute('dateTime')).toBe(latest[0]!.at);
		expect(h.textContent).not.toContain('(no text)');
		fireEvent.click(within(h).getByRole('button', { name: `Show ${m.edits.length - EDITS_SHOWN} more` }));
		expect(mapLinks(h)).toHaveLength(m.edits.length);
	});

	it('shows an earlier version of a checklist as its tasks', () => {
		openMessage(sampleId((s) => s.content_type === 'checklist'));
		const tasks = [...history().querySelectorAll('.e-detail li')].map((li) => li.textContent);
		expect(tasks).toEqual(['Tag the release', 'Write the changelog']);
	});
});
