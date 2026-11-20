// A chat background's fill as CSS: only colours built from integers.
import { describe, expect, it } from 'vitest';
import { fillCss, rgb } from './background.tsx';

describe('a background fill as CSS', () => {
	it('turns an integer into a colour, and nothing else', () => {
		expect(rgb(0x2b5278)).toBe('#2b5278');
		expect(rgb(0)).toBe('#000000');
		for (const bad of [-1, 0x1000000, 1.5, '#fff', 'red;x', null, NaN]) expect(rgb(bad)).toBeNull();
	});

	it('draws a solid colour, a gradient at its angle, and a freeform blend', () => {
		expect(fillCss({ type: 'solid', colors: [0xffffff], rotation_angle: 0 })).toBe('#ffffff');
		expect(fillCss({ type: 'gradient', colors: [0x000000, 0xffffff], rotation_angle: 0 })).toBe('linear-gradient(180deg, #000000, #ffffff)');
		expect(fillCss({ type: 'gradient', colors: [0x000000, 0xffffff], rotation_angle: 315 })).toBe('linear-gradient(135deg, #000000, #ffffff)');
		const ff = fillCss({ type: 'freeform_gradient', colors: [0x110000, 0x002200, 0x000033], rotation_angle: 0 })!;
		expect(ff.match(/radial-gradient/g)).toHaveLength(3);
		expect(ff).toContain('#110000');
		expect(ff.endsWith(', #000033')).toBe(true);
		expect(fillCss({ type: 'freeform_gradient', colors: [1, 2, 3, 4], rotation_angle: 0 })!.match(/radial-gradient/g)).toHaveLength(4);
	});

	it('drops what is not a colour, and draws no fill without one', () => {
		expect(fillCss({ type: 'gradient', colors: [0x123456, 'red' as unknown as number], rotation_angle: 0 })).toBe('#123456');
		expect(fillCss({ type: 'solid', colors: [], rotation_angle: 0 })).toBeNull();
		expect(fillCss(null)).toBeNull();
	});
});
