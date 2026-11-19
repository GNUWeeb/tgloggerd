// @vitest-environment happy-dom
// The shared page scroll lock: nested locks restore the page's own
// value only after the last release, in either order.
import { afterEach, describe, expect, it } from 'vitest';
import { lockScroll } from './scrollLock.ts';

const ov = () => document.body.style.overflow;
afterEach(() => { document.body.style.overflow = ''; });

describe('scroll lock', () => {
	it('hides the overflow and restores it on release', () => {
		const release = lockScroll();
		expect(ov()).toBe('hidden');
		release();
		expect(ov()).toBe('');
	});

	for (const order of ['first', 'last'] as const) {
		it(`keeps the page locked until the last release (${order} lock released first)`, () => {
			const a = lockScroll();
			const b = lockScroll();
			(order === 'first' ? a : b)();
			expect(ov()).toBe('hidden');
			(order === 'first' ? b : a)();
			expect(ov()).toBe('');
		});
	}

	it('ignores a second release of the same lock', () => {
		const a = lockScroll();
		const b = lockScroll();
		a();
		a();
		expect(ov()).toBe('hidden');
		b();
		expect(ov()).toBe('');
		const c = lockScroll();
		b();
		expect(ov()).toBe('hidden');
		c();
		expect(ov()).toBe('');
	});

	it('restores a value the page had set itself', () => {
		document.body.style.overflow = 'clip';
		const a = lockScroll();
		const b = lockScroll();
		b();
		a();
		expect(ov()).toBe('clip');
	});
});
