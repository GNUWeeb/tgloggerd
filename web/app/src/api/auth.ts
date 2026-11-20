// Sign-in and sign-out against the Go service (webapi/README,
// "Authentication"). Signing in carries the pre-login token from GET
// /api/login in X-CSRF-Token (the loginCsrf scheme); every other change
// carries the session's token, which the client adds.
import type { S } from './schema.ts';
import { api } from './client.ts';

export { sessionCsrf } from './client.ts';

export type LoginError = S['LoginError'];

export async function loginToken(): Promise<{ signed_in: boolean; csrf?: string }> {
	const { data } = await api.GET('/api/login');
	return data ?? { signed_in: false };
}

export async function login(csrf: string, username: string, password: string): Promise<{ ok: true; landing?: string } | ({ ok: false; status: number } & Partial<LoginError> & { error: string })> {
	const { data, error, response } = await api.POST('/api/login', { body: { username, password }, headers: { 'x-csrf-token': csrf } });
	if (response.ok) return { ok: true, landing: data?.landing };
	return { ok: false, status: response.status, ...((error ?? { error: 'unknown' }) as LoginError) };
}

export async function logout(): Promise<void> {
	await api.POST('/api/logout');
}

// The signed-in account's landing page, for a sign-in page reached while
// already signed in.
export async function landing(): Promise<string | undefined> {
	const { data } = await api.GET('/api/session');
	return data?.landing;
}
