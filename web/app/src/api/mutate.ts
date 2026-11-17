// JSON requests that change state, with the session's CSRF token in
// X-CSRF-Token (webapi/README, "Authentication"). Errors are
// {"error": "<code>", "message"?: "..."}.
import { sessionCsrf } from './auth.ts';

export class ApiError extends Error {
	constructor(public status: number, public code: string, message?: string) {
		super(message || code);
	}
}

export async function mutate<T = unknown>(method: 'POST' | 'PUT' | 'PATCH' | 'DELETE', path: string, body?: unknown): Promise<T> {
	const res = await fetch(path, {
		method,
		credentials: 'same-origin',
		headers: { 'x-csrf-token': sessionCsrf(), ...(body === undefined ? {} : { 'content-type': 'application/json' }) },
		body: body === undefined ? undefined : JSON.stringify(body),
	});
	if (res.status === 204) return undefined as T;
	const data = (await res.json().catch(() => ({}))) as T & { error?: string; message?: string };
	if (!res.ok) throw new ApiError(res.status, data.error || 'error', data.message);
	return data;
}
