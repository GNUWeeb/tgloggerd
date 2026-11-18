// A federation's details (/settings/federations/{id}): its groups page by
// page, and changing its name and description.
import { api, ok } from './client.ts';
import type { S } from './schema.ts';

export type FederationDetailProps = S['FederationDetailProps'];
export type GroupsPage = S['FederationGroupsPage'];
export type FedGroup = S['GroupCounted'];
export type Person = S['FedPerson'];

export function federationGroups(id: number, q: { q?: string; cursor?: string | null; limit?: number }, signal?: AbortSignal): Promise<GroupsPage> {
	return ok(api.GET('/api/federations/{id}/groups', { params: { path: { id }, query: { q: q.q || undefined, cursor: q.cursor || undefined, limit: q.limit ?? 50 } }, signal }));
}

// The answer is the federation's id only: the page refetches its data.
export function updateFederation(id: number, body: S['FederationWrite']): Promise<unknown> {
	return ok(api.PATCH('/api/federations/{id}', { params: { path: { id } }, body }));
}
