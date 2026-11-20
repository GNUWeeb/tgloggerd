// @vitest-environment happy-dom
// The message search's fields that need one chat (needs_chat): offered
// only once a chat id = filter, joined by AND, pins the list to a chat.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';
import { fakeApi, renderPage, setup } from '../test/page.tsx';
import { chatBound } from '../ui/list.tsx';
import type { Cond } from '../types.ts';

beforeEach(setup);
const fieldKeys = () => [...(screen.getByRole('combobox', { name: 'Field' }) as HTMLSelectElement).options].map((o) => o.value);
const withConds = (conds: unknown[]) => `/group_messages?f=${encodeURIComponent(JSON.stringify(conds))}`;

describe('fields that need one chat', () => {
	it('are left out, with a note, until a chat is pinned', () => {
		fakeApi('superadmin');
		renderPage('/group_messages');
		fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
		expect(fieldKeys()).not.toContain('tdlib_type');
		expect(fieldKeys()).not.toContain('service_type');
		expect(screen.getByText('TDLib type and Service type can be added once a group id = filter is set, joined by AND.')).toBeTruthy();
	});

	it('are offered beside a chat id = filter', () => {
		fakeApi('superadmin');
		renderPage(withConds([{ field: 'chat_id', op: '=', value: '-1001775585202' }]));
		fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
		expect(fieldKeys()).toContain('tdlib_type');
		expect(fieldKeys()).toContain('service_type');
	});

	it('need every filter joined by AND', () => {
		const chat: Cond = { field: 'chat_id', op: '=', value: '-1' };
		const other: Cond = { field: 'message_id', op: '>', value: '1' };
		expect(chatBound([chat, other])).toBe(true);
		expect(chatBound([other, { ...chat, n: 'OR' as const }])).toBe(true);
		expect(chatBound([{ ...chat, n: 'OR' as const }, other])).toBe(false);
		expect(chatBound([{ ...chat, op: '!=' }])).toBe(false);
		expect(chatBound([other])).toBe(false);
	});
});
