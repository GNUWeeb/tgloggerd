// @vitest-environment happy-dom
// The per-day bars: the date and count of every day, on hover and on
// keyboard focus, the date axis, and the table for screen readers.
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { DayBars } from './daybars.tsx';
import { renderPage, setup } from '../test/page.tsx';

afterEach(cleanup);
// 30 days ending on Tue 17 Nov 2026 (UTC), today's count last.
const data = Array.from({ length: 30 }, (_, i) => (i === 10 ? 900 : 100 + i));
const now = '2026-11-17T16:00:00Z';

describe('day bars', () => {
	it('names every day and its count', () => {
		render(<DayBars data={data} now={now} />);
		const bars = screen.getAllByRole('img');
		expect(bars).toHaveLength(30);
		expect(bars[29]!.getAttribute('aria-label')).toBe('Today, 17 Nov: 129 messages');
		expect(bars[0]!.getAttribute('aria-label')).toBe('Mon 19 Oct: 100 messages');
		expect(bars[10]!.getAttribute('aria-label')).toBe('Thu 29 Oct: 900 messages');
	});

	it('shows the count and the day on hover, and hides it on leaving', () => {
		const r = render(<DayBars data={data} now={now} />);
		const bars = screen.getAllByRole('img');
		fireEvent.mouseEnter(bars[10]!);
		const tip = screen.getByRole('status');
		expect(tip.textContent).toContain('900');
		expect(tip.textContent).toContain('Thu 29 Oct');
		fireEvent.mouseLeave(r.container.querySelector('.a-daybars')!);
		expect(screen.queryByRole('status')).toBeNull();
	});

	it('shows the same on keyboard focus', () => {
		render(<DayBars data={data} now={now} />);
		const bars = screen.getAllByRole('img');
		expect(bars[29]!.getAttribute('tabindex')).toBe('0');
		fireEvent.focus(bars[29]!);
		expect(screen.getByRole('status').textContent).toContain('Today, 17 Nov');
		fireEvent.blur(bars[29]!);
		expect(screen.queryByRole('status')).toBeNull();
	});

	it('labels every seventh day back from today, and the peak', () => {
		const r = render(<DayBars data={data} now={now} />);
		const ticks = [...r.container.querySelectorAll('.a-daybars-axis span')].map((s) => s.textContent);
		expect(ticks).toEqual(['20 Oct', '27 Oct', '3 Nov', '10 Nov', 'Today']);
		expect(r.container.querySelector('.a-daybars-peak')!.textContent).toBe('900');
	});

	it('gives screen readers every value in a table', () => {
		render(<DayBars data={data} now={now} />);
		const rows = within(screen.getByRole('table')).getAllByRole('row');
		expect(rows).toHaveLength(31);
		expect(rows[30]!.textContent).toBe('Today, 17 Nov129');
	});

	it('is the dashboard\'s messages-per-day chart', () => {
		setup();
		renderPage('/');
		expect(screen.getByText('last 30 days, UTC')).toBeTruthy();
		expect(screen.getAllByRole('img', { name: /^Today, 17 Nov: [\d,]+ messages$/ })).toHaveLength(1);
	});
});
