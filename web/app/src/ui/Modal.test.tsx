// @vitest-environment happy-dom
// A dialog locks the page's scrolling once while open and unlocks it
// on close, however often its onClose changes and whatever viewer
// opened inside it closed first.
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { useState } from 'react';
import { Modal } from './Modal.tsx';
import { Lightbox } from '../routes/chat-media.tsx';

const noop = () => {};
const ov = () => document.body.style.overflow;
afterEach(() => { cleanup(); document.body.style.overflow = ''; });

// As the files preview does it: a new onClose on every render, and none
// while its bigger view is open.
function Page() {
	const [open, setOpen] = useState(true);
	const [zoom, setZoom] = useState(false);
	const [, tick] = useState(0);
	if (!open) return <p>closed</p>;
	return (
		<Modal title="Preview" onClose={zoom ? noop : () => setOpen(false)}>
			<button type="button" onClick={() => tick((n) => n + 1)}>Render</button>
			<button type="button" onClick={() => setZoom(true)}>View bigger</button>
			{zoom && <Lightbox m={{ url: '/files/x', name: 'x.jpg' }} kind="picture" onClose={() => setZoom(false)} />}
		</Modal>
	);
}

describe('modal scroll lock', () => {
	it('unlocks the page on close when onClose changes every render', () => {
		render(<Page />);
		expect(ov()).toBe('hidden');
		fireEvent.click(screen.getByRole('button', { name: 'Render' }));
		fireEvent.click(screen.getByRole('button', { name: 'Render' }));
		expect(ov()).toBe('hidden');
		fireEvent.click(screen.getByRole('button', { name: 'Close' }));
		expect(screen.getByText('closed')).toBeTruthy();
		expect(ov()).toBe('');
	});

	it('unlocks the page on close after a viewer opened inside it closed', () => {
		render(<Page />);
		fireEvent.click(screen.getByRole('button', { name: 'View bigger' }));
		const viewer = screen.getByRole('dialog', { name: 'Media viewer' });
		fireEvent.click(viewer.querySelector('button[aria-label="Close"]')!);
		expect(screen.queryByRole('dialog', { name: 'Media viewer' })).toBeNull();
		expect(ov()).toBe('hidden');
		fireEvent.keyDown(window, { key: 'Escape' });
		expect(screen.getByText('closed')).toBeTruthy();
		expect(ov()).toBe('');
	});
});
