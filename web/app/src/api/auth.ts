// Sign-in and sign-out against the Go service (webapi/README,
// "Authentication"). State-changing requests carry the CSRF token in
// X-CSRF-Token: the pre-login token from GET /api/login for signing in,
// the session's token (the csrf-token meta tag) otherwise.

export interface LoginError { error: string; message?: string; csrf?: string }

export function sessionCsrf(): string {
	return document.querySelector<HTMLMetaElement>('meta[name="csrf-token"]')?.content ?? '';
}

export async function loginToken(): Promise<{ signed_in: boolean; csrf?: string }> {
	const res = await fetch('/api/login', { credentials: 'same-origin' });
	return (await res.json()) as { signed_in: boolean; csrf?: string };
}

export async function login(csrf: string, username: string, password: string): Promise<{ ok: true } | ({ ok: false; status: number } & LoginError)> {
	const res = await fetch('/api/login', {
		method: 'POST',
		credentials: 'same-origin',
		headers: { 'content-type': 'application/json', 'x-csrf-token': csrf },
		body: JSON.stringify({ username, password }),
	});
	if (res.ok) return { ok: true };
	const body = (await res.json().catch(() => ({ error: 'unknown' }))) as LoginError;
	return { ok: false, status: res.status, ...body };
}

export async function logout(): Promise<void> {
	await fetch('/api/logout', { method: 'POST', credentials: 'same-origin', headers: { 'x-csrf-token': sessionCsrf() } });
}
