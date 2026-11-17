// The page data and render protocol shapes shared by the render service,
// the browser and (as JSON) the Go service. Specification:
// docs/design/web-ssr.md sections 4 and 5.

export const PROTOCOL = 1;

export type Role = 'superadmin' | 'admin' | 'member';

// Everything optional falls back to the site default: until per-account
// settings exist the server sends defaults or leaves fields out.
export interface Viewer {
	signed_in: boolean;
	account?: { id: number; username: string; name: string; role: Role };
	now: string;
	timezone?: string;
	date_format?: '24h' | '12h' | 'iso';
	lang?: 'en' | 'id';
	theme?: 'system' | 'light' | 'dark';
	sidebar?: 'open' | 'collapsed';
	rows_per_page?: number;
	nav?: string[];
	pinned?: { id: number; title: string }[];
	chat?: {
		auto_load?: boolean;
		live?: boolean;
		media?: 'auto' | 'click';
		show_deleted?: boolean;
		show_edited?: boolean;
		show_service?: boolean;
	};
}

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
