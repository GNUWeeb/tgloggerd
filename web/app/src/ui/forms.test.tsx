// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { ChatPicker, Field, MultiPicker } from './forms.tsx';

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe('pickers inside a field label', () => {
	it('name the chat picker\'s button after its state, not the label\'s whole text', () => {
		vi.stubGlobal('fetch', async () => new Response('{"items":[],"next_cursor":null}', { headers: { 'content-type': 'application/json' } }));
		const { rerender } = render(<Field label="Telegram chat" hint="A long hint about chats."><ChatPicker value={null} onChange={() => {}} search={{ kind: 'group' }} /></Field>);
		expect(screen.getByRole('button', { name: 'Choose a chat' })).toBeTruthy();
		rerender(<Field label="Telegram chat" hint="A long hint about chats."><ChatPicker value={{ id: -1, title: 'Rust', kind: 'group' }} onChange={() => {}} search={{ kind: 'group' }} /></Field>);
		expect(screen.getByRole('button', { name: 'Chat: Rust. Change' })).toBeTruthy();
	});

	it('name the multi-picker\'s input after what it adds, not after its chips', () => {
		render(<Field label="Groups" hint="Hint."><MultiPicker value={[{ id: -1, title: 'Rust', kind: 'group' }]} onChange={() => {}} options={[]} placeholder="Add a group..." /></Field>);
		expect(screen.getByRole('textbox', { name: 'Add a group...' })).toBeTruthy();
	});
});
