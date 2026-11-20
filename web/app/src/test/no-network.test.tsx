// @vitest-environment happy-dom
import { expect, it, vi } from 'vitest';
import { fakeApi, setup } from './page.tsx';

// A request started after its test ended (a timer or poll the test did
// not wait for) must not reach the network once page.tsx has removed
// its fake fetch.
let late: () => Promise<Response> = () => Promise.reject(new Error('first test did not run'));

it('answers a request while the fake is installed', async () => {
	setup();
	fakeApi('superadmin', { 'GET /api/late': () => ({ ok: true }) });
	// The app's client passes a Request, as the fake expects.
	late = () => fetch(new Request('http://localhost/api/late'));
	expect((await late()).status).toBe(200);
});

it('refuses and warns about a request made after the test that faked fetch ended', async () => {
	const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
	try {
		await expect(late()).rejects.toThrow(/real network request: http:\/\/localhost\/api\/late/);
		expect(warn).toHaveBeenCalledWith('[no-network] refused a real request: http://localhost/api/late');
	} finally {
		warn.mockRestore();
	}
});
