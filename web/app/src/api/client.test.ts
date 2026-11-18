import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError, ok, setSessionCsrf } from './client.ts';

afterEach(() => vi.unstubAllGlobals());

const json = (status: number, body: unknown) => new Response(status === 204 ? null : JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });

describe('api client', () => {
	it('adds the session token to changes only, and keeps a token the call sets', async () => {
		const seen: Request[] = [];
		vi.stubGlobal('fetch', vi.fn(async (r: Request) => { seen.push(r); return json(204, null); }));
		setSessionCsrf('sess');
		await api.POST('/api/logout');
		await api.GET('/api/login');
		await api.POST('/api/login', { body: { username: 'u', password: 'p' }, headers: { 'x-csrf-token': 'pre' } });
		expect(seen.map((r) => [r.method, r.headers.get('x-csrf-token')])).toEqual([['POST', 'sess'], ['GET', null], ['POST', 'pre']]);
	});

	it('turns an error answer into an ApiError with its message', async () => {
		vi.stubGlobal('fetch', vi.fn(async () => json(409, { error: 'last_superadmin', message: 'The last active superadmin cannot be disabled.' })));
		const e = await ok(api.POST('/api/accounts/{id}/disable', { params: { path: { id: 1 } } })).catch((x: unknown) => x);
		expect(e).toBeInstanceOf(ApiError);
		expect(e).toMatchObject({ status: 409, code: 'last_superadmin', message: 'The last active superadmin cannot be disabled.' });
	});

	it('returns the data of a success', async () => {
		vi.stubGlobal('fetch', vi.fn(async () => json(201, { id: 7, token: 'tok' })));
		await expect(ok(api.POST('/api/mcp/tokens', { body: { name: 'ci' } }))).resolves.toEqual({ id: 7, token: 'tok' });
	});
});

describe('gates', () => {
	it('calls back on a 403 for a gate, and not on other 403s', async () => {
		const { onGate } = await import('./client.ts');
		const cb = vi.fn();
		onGate(cb);
		vi.stubGlobal('fetch', vi.fn(async () => json(403, { error: 'csrf' })));
		await api.POST('/api/logout');
		expect(cb).not.toHaveBeenCalled();
		vi.stubGlobal('fetch', vi.fn(async () => json(403, { error: 'email_required' })));
		const e = await ok(api.POST('/api/mcp/tokens', { body: { name: 'x' } })).catch((x: unknown) => x);
		expect(cb).toHaveBeenCalledTimes(1);
		expect(e).toMatchObject({ status: 403, code: 'email_required' });
		vi.stubGlobal('fetch', vi.fn(async () => json(403, { error: 'password_change_required' })));
		await api.POST('/api/logout');
		expect(cb).toHaveBeenCalledTimes(2);
		onGate(null);
	});
});
