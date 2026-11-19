// @vitest-environment happy-dom
// Message kinds by their human labels, not the archive's stored names:
// the message page's pill and rows.
import { beforeEach, describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { fixture, renderPage, setup } from '../test/page.tsx';
import type { MessageProps } from './message.tsx';
import type { MessagesProps } from './messages.tsx';

beforeEach(setup);

const kv = (key: string) => screen.getAllByText(key).map((dt) => dt.nextElementSibling?.textContent).find(Boolean);

describe('the message page', () => {
	const url = () => `/messages/group/${fixture<MessagesProps>('/group_messages').props.rows[0]!.id}`;
	const render = (patch: Partial<MessageProps['message']>) => {
		const p = fixture<MessageProps>(url()).props;
		return renderPage(url(), 'superadmin', { props: { message: { ...p.message, ...patch } } });
	};
	const pill = () => document.querySelector('.a-msgflagsrow')!.firstElementChild!.textContent;

	it('names a video message in its pill and Type row', () => {
		render({ content_type: 'video_note', text: 'hi', media: null });
		expect(pill()).toBe('Video message');
		expect(kv('Type')).toBe('Video message');
		expect(screen.queryByText('video_note')).toBeNull();
	});

	it('names a service message by its action, keeping the raw name as a hint', () => {
		render({ content_type: 'service', text: '', media: null, service_type: 'chat_add_members', service: undefined });
		expect(pill()).toBe('Members added');
		expect(kv('Type')).toBe('Service message');
		expect(kv('Service action')).toBe('Members added chat_add_members');
		expect(document.querySelector('.a-kv .mono.a-muted')!.textContent).toBe('chat_add_members');
	});

	it('shows an action it has no label for by its raw name', () => {
		render({ content_type: 'service', text: '', media: null, service_type: 'some_future_action', service: undefined });
		expect(pill()).toBe('some_future_action');
	});
});
