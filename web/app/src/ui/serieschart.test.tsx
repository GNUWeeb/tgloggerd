// @vitest-environment happy-dom
// The series chart's last period while it is still running (the end is
// Now): drawn as unfinished, so its partial count does not read as a
// drop, and named "so far".
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { SeriesChart, type Point } from './serieschart.tsx';

afterEach(cleanup);

const H = 3600;
const pts: Point[] = [20, 40, 30, 2].map((n, i) => ({ at: 1_700_000_000 + i * H, n }));
const draw = (variant: 'bars' | 'line' | 'area' | 'total', live: boolean) =>
	render(<div className="app"><SeriesChart points={pts} period="hour" variant={variant} tz="UTC" label="Messages per hour" live={live} /></div>).container;

describe('a running last period', () => {
	it('dashes the line into it, and only into it', () => {
		for (const v of ['line', 'area', 'total'] as const) {
			const c = draw(v, true);
			const dashed = c.querySelectorAll('.a-sc-line.partial');
			expect(dashed, v).toHaveLength(1);
			// From the period before the last to the last: one segment.
			expect(dashed[0]!.getAttribute('d')!.match(/[ML]/g), v).toEqual(['M', 'L']);
			// The solid line stops at the period before.
			const solid = c.querySelector('.a-sc-line:not(.partial)')!.getAttribute('d')!;
			expect(solid.match(/[ML]/g), v).toHaveLength(pts.length - 1);
			cleanup();
		}
	});

	it('draws the last bar lighter', () => {
		const bars = draw('bars', true).querySelectorAll('.a-sc-bar');
		expect([...bars].map((b) => b.classList.contains('partial'))).toEqual([false, false, false, true]);
	});

	it('says "so far" for the running period in the tooltip', () => {
		draw('line', true);
		fireEvent.keyDown(screen.getByRole('img', { name: 'Messages per hour' }), { key: 'End' });
		expect(screen.getByRole('status').textContent).toContain('2 messages so far');
		fireEvent.keyDown(screen.getByRole('img', { name: 'Messages per hour' }), { key: 'ArrowLeft' });
		expect(screen.getByRole('status').textContent).not.toContain('so far');
	});

	it('draws a fixed range as before: one solid line, no partial marks', () => {
		const c = draw('line', false);
		expect(c.querySelectorAll('.a-sc-line.partial')).toHaveLength(0);
		expect(c.querySelector('.a-sc-line')!.getAttribute('d')!.match(/[ML]/g)).toHaveLength(pts.length);
		expect(draw('bars', false).querySelectorAll('.a-sc-bar.partial')).toHaveLength(0);
	});
});
