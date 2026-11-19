// The dashboard's chart of one series, messages per period, drawn by hand
// in SVG: bars with 2px gaps and 4px rounded tops on the baseline. One
// y axis, a recessive grid, labels in the text colours and the marks in
// the accent, so light and dark both work. The pointer anywhere over the
// plot (or the arrow keys, once focused) picks the nearest period: a
// crosshair marks it and the tooltip gives its start and count.
import { useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from 'react';
import { fmt } from '../format.ts';
import { axisLabel, periodLabel, type Period } from '../periods.ts';

export interface Point { at: number; n: number }

const H = 200;
const M = { top: 12, right: 8, bottom: 26 };
// useLayoutEffect warns in the server render; it measures nothing there.
const useIso = typeof document === 'undefined' ? useEffect : useLayoutEffect;

// A round top for the y axis and its step, about four gridlines high.
export function niceScale(max: number): { top: number; step: number } {
	if (max <= 0) return { top: 4, step: 1 };
	const raw = max / 4;
	const mag = 10 ** Math.floor(Math.log10(raw));
	const step = [1, 2, 2.5, 5, 10].map((k) => k * mag).find((s) => s >= raw && Number.isInteger(s)) ?? Math.ceil(raw);
	return { top: Math.ceil(max / step) * step, step };
}

const yLabel = (v: number) => (v >= 1e6 ? `${+(v / 1e6).toFixed(1)}M` : v >= 1e4 ? `${+(v / 1e3).toFixed(1)}k` : fmt(v));

// A bar from the baseline up to y, its top corners rounded.
function barPath(x: number, w: number, y: number, base: number): string {
	const h = base - y;
	const r = Math.min(4, w / 2, h);
	if (r <= 0.5) return `M${x},${base}V${y}h${w}V${base}Z`;
	return `M${x},${base}V${y + r}a${r},${r} 0 0 1 ${r},${-r}h${w - 2 * r}a${r},${r} 0 0 1 ${r},${r}V${base}Z`;
}

export interface SeriesChartProps {
	points: Point[];
	period: Period;
	tz: string;
	h12?: boolean;
	noun?: string;
	// The chart's accessible name, summarising the range and the total.
	label: string;
}

export function SeriesChart({ points, period, tz, h12 = false, noun = 'messages', label }: SeriesChartProps) {
	const box = useRef<HTMLDivElement>(null);
	const [width, setWidth] = useState(720);
	const [at, setAt] = useState<number | null>(null);
	useIso(() => {
		const el = box.current;
		if (!el) return;
		const measure = () => { if (el.clientWidth > 0) setWidth(el.clientWidth); };
		measure();
		if (typeof ResizeObserver === 'undefined') return;
		const ro = new ResizeObserver(measure);
		ro.observe(el);
		return () => ro.disconnect();
	}, []);
	const n = points.length;
	const values = points.map((p) => p.n);
	const { top, step } = niceScale(Math.max(0, ...values));
	const ticks: number[] = [];
	for (let v = 0; v <= top + step / 2; v += step) ticks.push(v);
	const left = Math.max(28, 8 + 7 * yLabel(top).length);
	const plotW = Math.max(1, width - left - M.right);
	const base = H - M.bottom;
	const plotH = base - M.top;
	const slot = plotW / Math.max(1, n);
	const xc = (i: number) => left + (i + 0.5) * slot;
	const y = (v: number) => base - (v / top) * plotH;
	// Bars keep a 2px gap while there is room for one; denser, they touch.
	const barW = slot >= 4 ? slot - 2 : slot;
	// About six labels, counted back from the last period so it has one.
	const every = Math.max(1, Math.ceil(n / Math.max(2, Math.floor(plotW / 90))));
	const xticks = points.map((_, i) => i).filter((i) => (n - 1 - i) % every === 0);
	const pick = (e: PointerEvent<SVGRectElement>) => {
		const r = e.currentTarget.getBoundingClientRect();
		const scale = r.width > 0 ? plotW / r.width : 1;
		const i = Math.floor(((e.clientX - r.left) * scale) / slot);
		setAt(Math.min(n - 1, Math.max(0, i)));
	};
	const key = (e: KeyboardEvent) => {
		const k = e.key;
		if (!n || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(k)) return;
		e.preventDefault();
		setAt((c) => (k === 'Home' ? 0 : k === 'End' ? n - 1 : Math.min(n - 1, Math.max(0, (c ?? (k === 'ArrowLeft' ? n : -1)) + (k === 'ArrowLeft' ? -1 : 1)))));
	};
	const shown = at !== null && at < n ? at : null;
	const tipX = shown === null ? 0 : xc(shown);
	return (
		<div className="a-sc" ref={box}>
			<div className="a-sc-plot" role="img" aria-label={label} tabIndex={0} onKeyDown={key} onBlur={() => setAt(null)}>
				<svg width={width} height={H} viewBox={`0 0 ${width} ${H}`} aria-hidden="true">
					<g className="a-sc-grid">
						{ticks.map((v) => <line key={v} x1={left} x2={width - M.right} y1={y(v)} y2={y(v)} />)}
					</g>
					<g className="a-sc-ylab">
						{ticks.map((v) => <text key={v} x={left - 6} y={y(v)} dy="0.32em" textAnchor="end">{yLabel(v)}</text>)}
					</g>
					<g className="a-sc-xlab">
						{xticks.map((i) => (
							<text key={i} x={xc(i)} y={H - 8} textAnchor={xc(i) < left + 30 ? 'start' : xc(i) > width - 40 ? 'end' : 'middle'}>{axisLabel(points[i]!.at, period, tz, h12)}</text>
						))}
					</g>
					<line className="a-sc-base" x1={left} x2={width - M.right} y1={base} y2={base} />
					<g className="a-sc-marks">
						{points.map((p, i) => (p.n > 0 ? <path key={i} className={`a-sc-bar ${i === shown ? 'on' : ''}`} d={barPath(xc(i) - barW / 2, barW, y(p.n), base)} /> : null))}
					</g>
					{shown !== null && <line className="a-sc-cross" x1={tipX} x2={tipX} y1={M.top} y2={base} />}
					{/* The whole plot is the hit area, so a thin bar need not be hit. */}
					<rect className="a-sc-hit" x={left} y={0} width={plotW} height={base} onPointerMove={pick} onPointerDown={pick} onPointerLeave={() => setAt(null)} />
				</svg>
			</div>
			{shown !== null && (
				<div className="a-sc-tip" role="status" style={{ left: tipX, top: Math.max(0, y(values[shown]!) - 8), transform: `translate(${tipX < width * 0.15 ? '0' : tipX > width * 0.85 ? '-100%' : '-50%'}, -100%)` }}>
					<b className="num">{fmt(values[shown]!)}</b> <span className="a-muted">{noun}</span>
					<div className="a-muted sm">{periodLabel(points[shown]!.at, period, tz, h12)}</div>
				</div>
			)}
		</div>
	);
}
