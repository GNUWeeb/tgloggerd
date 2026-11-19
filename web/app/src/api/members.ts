// A group's Telegram members: the real member count, the member list
// (GET /api/groups/{id}/members, and the group_members page) and a
// superadmin's switch that hides a channel's list (PATCH
// /api/groups/{id}).
//
// LOCAL TYPES: the API description does not have these fields and
// endpoints yet. They follow the agreed contract and live here, in one
// place; once webapi/openapi.yaml (and so the generated schema) has
// them, use the generated ones and drop these.
import type { Client } from 'openapi-fetch';
import { api, ok } from './client.ts';
import type { S } from './schema.ts';
import type { Viewer } from '../page.ts';

// Telegram's own member count, cached by the service: null until the
// archive has asked Telegram (or when Telegram does not say).
export interface MemberCount {
	member_count?: number | null;
	member_count_at?: string | null;
}

export type MemberStatus = 'creator' | 'administrator' | 'member' | 'restricted';

// Why a group's list is not available: the archive account may not
// read it (not_allowed: a channel it is no admin of; not_member: it
// left), a superadmin hid it (hidden), or Telegram asked to wait.
// unavailable: the daemon is off or does not answer.
export type MembersReason = '' | 'not_allowed' | 'hidden' | 'not_member' | 'flood_wait' | 'unavailable';

export interface GroupMember {
	// A member the archive has not stored yet comes with its id alone
	// (an empty name).
	user: S['UserSummary'];
	status: MemberStatus;
	// When the member joined, if Telegram says. A date-time, as the
	// service sends dates (a guess: the contract does not name the format).
	joined_at: string | null;
}

export interface MembersPage {
	available: boolean;
	reason: MembersReason;
	// Telegram's count, when known.
	total: number | null;
	members: GroupMember[];
	// The page answered, and how many seconds Telegram asked to wait
	// (flood_wait only).
	offset?: number;
	limit?: number;
	retry_after?: number;
}

export const MEMBERS_PAGE = 50;

// The group page's own fields for the list.
export interface MemberListFlags {
	hide_member_list?: boolean;
	// True for a superadmin on a channel.
	can_hide_member_list?: boolean;
}

// Who sees member lists: viewers who see users in full (superadmins
// and admins). The service answers 404 to anyone else.
export const seesMembers = (v: Viewer): boolean => v.signed_in && (v.account?.role === 'superadmin' || v.account?.role === 'admin');

// The two calls, typed as openapi-fetch takes them.
type Answer<R> = {
	200: { headers: { [name: string]: unknown }; content: { 'application/json': R } };
	default: { headers: { [name: string]: unknown }; content: { 'application/json': S['Error'] } };
};
type Path<Q> = { query?: Q; header?: never; path: { id: number }; cookie?: never };
type Item = { parameters: { query?: never; header?: never; path?: never; cookie?: never }; put?: never; post?: never; delete?: never; options?: never; head?: never; trace?: never };
interface LocalPaths {
	'/api/groups/{id}/members': Item & { patch?: never; get: { parameters: Path<{ offset?: number; limit?: number }>; requestBody?: never; responses: Answer<MembersPage> } };
	'/api/groups/{id}': Item & { get?: never; patch: { parameters: Path<never>; requestBody: { content: { 'application/json': { hide_member_list: boolean } } }; responses: Answer<unknown> } };
}
const local = api as unknown as Client<LocalPaths>;

export function fetchMembers(id: number, offset = 0, limit = MEMBERS_PAGE): Promise<MembersPage> {
	return ok(local.GET('/api/groups/{id}/members', { params: { path: { id }, query: { offset, limit } } }));
}

export function setHideMemberList(id: number, hide: boolean): Promise<unknown> {
	return ok(local.PATCH('/api/groups/{id}', { params: { path: { id } }, body: { hide_member_list: hide } }));
}

export function reasonText(r: MembersReason): string {
	if (r === 'hidden') return 'Hidden by a superadmin';
	if (r === 'flood_wait') return 'Telegram asked to wait; try again later';
	if (r === 'not_allowed' || r === 'not_member') return "Telegram doesn't show this group's members to the archive account";
	if (r === 'unavailable') return 'Member list not available right now';
	return 'Telegram did not give this group\'s members';
}
