// A group's Telegram members: the member list (GET
// /api/groups/{id}/members, and the group_members page) and a
// superadmin's switch that hides a channel's list (PATCH
// /api/groups/{id}).

import { api, ok } from './client.ts';
import type { S } from './schema.ts';
import type { Viewer } from '../page.ts';

export type GroupMember = S['GroupMember'];
export type MembersPage = S['GroupMembersPage'];

export const MEMBERS_PAGE = 50;
// The longest search the service takes (?q=, a leading @ ignored).
export const MEMBERS_Q_MAX = 64;

// Who sees member lists: viewers who see users in full (superadmins
// and admins). The service answers 404 to anyone else.
export const seesMembers = (v: Viewer): boolean => v.signed_in && (v.account?.role === 'superadmin' || v.account?.role === 'admin');

// q searches the members (by name, @username or id); an empty one
// lists them all.
export function fetchMembers(id: number, offset = 0, limit = MEMBERS_PAGE, q = ''): Promise<MembersPage> {
	return ok(api.GET('/api/groups/{id}/members', { params: { path: { id }, query: { offset, limit, ...(q ? { q } : {}) } } }));
}

export function setHideMemberList(id: number, hide: boolean): Promise<unknown> {
	return ok(api.PATCH('/api/groups/{id}', { params: { path: { id } }, body: { hide_member_list: hide } }));
}

// A reason the client does not know reads as not available.
export function reasonText(r: string): string {
	if (r === 'hidden') return 'Hidden by a superadmin';
	if (r === 'flood_wait') return 'Telegram asked to wait; try again later';
	if (r === 'not_allowed' || r === 'not_member') return "Telegram doesn't show this group's members to the archive account";
	if (r === 'unavailable') return 'Member list not available right now';
	return 'Telegram did not give this group\'s members';
}
