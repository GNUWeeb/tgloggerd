// Numbers and dates, in the viewer's timezone and format. "now" comes
// from the server, so the first browser render matches the server's.
import type { Viewer } from './page.ts';

export const SITE_DEFAULTS = { timezone: 'Asia/Jakarta', date_format: '24h', lang: 'en', rows_per_page: 25 } as const;

export function fmt(n: number): string {
	return n.toLocaleString('en-US');
}

export function compact(n: number): string {
	if (n >= 1e6) return (n / 1e6).toFixed(n >= 1e7 ? 0 : 1) + 'M';
	if (n >= 1e3) return (n / 1e3).toFixed(n >= 1e4 ? 0 : 1) + 'k';
	return String(n);
}

export function bytes(n: number): string {
	if (n >= 2 ** 40) return (n / 2 ** 40).toFixed(1) + ' TB';
	if (n >= 2 ** 30) return (n / 2 ** 30).toFixed(1) + ' GB';
	if (n >= 2 ** 20) return (n / 2 ** 20).toFixed(1) + ' MB';
	if (n >= 2 ** 10) return (n / 2 ** 10).toFixed(0) + ' KB';
	return n + ' B';
}

// Timestamps are UTC ISO strings from the server.
export function ago(iso: string, v: Viewer): string {
	const ms = Date.parse(v.now) - Date.parse(iso);
	if (ms < 3600e3) return Math.max(1, Math.floor(ms / 60e3)) + 'm ago';
	if (ms < 86400e3) return Math.floor(ms / 3600e3) + 'h ago';
	const d = Math.floor(ms / 86400e3);
	if (d < 30) return d + 'd ago';
	if (d < 365) return Math.floor(d / 30) + 'mo ago';
	return Math.floor(d / 365) + 'y ago';
}

export function dateTime(iso: string, v: Viewer): string {
	const tz = v.timezone || SITE_DEFAULTS.timezone;
	const f = v.date_format || SITE_DEFAULTS.date_format;
	const d = new Date(iso);
	if (f === 'iso') {
		const p = new Intl.DateTimeFormat('en-CA', { timeZone: tz, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).formatToParts(d);
		const g = (t: string) => p.find((x) => x.type === t)?.value ?? '';
		return `${g('year')}-${g('month')}-${g('day')} ${g('hour')}:${g('minute')}`;
	}
	return new Intl.DateTimeFormat('en-GB', { timeZone: tz, day: 'numeric', month: 'short', year: 'numeric', hour: 'numeric', minute: '2-digit', hour12: f === '12h' }).format(d);
}
