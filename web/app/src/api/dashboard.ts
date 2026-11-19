// The dashboard's counts on their own (GET /api/dashboard/kpis): the
// same KPIs, in the same scope, as the dashboard's page data.
import { api, ok } from './client.ts';
import type { S } from './schema.ts';

export type KPI = S['KPI'];

export async function dashboardKpis(signal?: AbortSignal): Promise<KPI[]> {
	return (await ok(api.GET('/api/dashboard/kpis', { signal }))).kpis;
}

// The dashboard chart's messages per period (GET /api/dashboard/series):
// start in unix seconds, end in unix seconds or 'now', which makes the
// answer live.
export type Series = S['DashboardSeries'];
export async function dashboardSeries(q: { period: Series['period']; start: number; end: number | 'now' }, signal?: AbortSignal): Promise<Series> {
	return ok(api.GET('/api/dashboard/series', { params: { query: { period: q.period, start: q.start, end: String(q.end) } }, signal }));
}
