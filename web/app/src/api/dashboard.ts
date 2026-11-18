// The dashboard's counts on their own (GET /api/dashboard/kpis): the
// same KPIs, in the same scope, as the dashboard's page data.
import { api, ok } from './client.ts';
import type { S } from './schema.ts';

export type KPI = S['KPI'];

export async function dashboardKpis(signal?: AbortSignal): Promise<KPI[]> {
	return (await ok(api.GET('/api/dashboard/kpis', { signal }))).kpis;
}
