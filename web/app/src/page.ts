// The page data and render protocol shapes shared by the render service,
// the browser and (as JSON) the Go service. Specification:
// docs/design/web-ssr.md sections 4 and 5.

import type { S } from './api/schema.ts';

export const PROTOCOL = 1;

export type Role = S['Role'];

// Everything optional falls back to the site default: until per-account
// settings exist the server sends defaults or leaves fields out.
// pinned[].photo: each pinned group's photo (a /files/ URL), until the
// API description has it.
type Pinned = NonNullable<S['Viewer']['pinned']>[number] & { photo?: string | null };
export type Viewer = Omit<S['Viewer'], 'pinned'> & { pinned?: Pinned[] };

export interface PageData<P = unknown> {
	v: number;
	route: string;
	url: string;
	status: 200 | 404;
	props: P;
	viewer: Viewer;
}

export interface RenderRequest extends PageData {
	request_id?: string;
	build: string;
}

export interface RenderResult {
	head: string;
	html: string;
}

export interface RenderResponse extends RenderResult {
	v: number;
	build: string;
	ms: number;
}

export interface RenderError {
	v: number;
	error: 'bad_request' | 'unknown_version' | 'build_mismatch' | 'unknown_route' | 'render_failed' | 'too_large';
	detail: string;
}

export class UnknownRoute extends Error {
	override name = 'UnknownRoute';
}
