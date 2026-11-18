// One stand-in per worker, reset before every test.
import { spawn, type ChildProcess } from 'node:child_process';
import { test as base, expect } from '@playwright/test';

export type Persona = 'superadmin' | 'admin' | 'member' | 'noemail' | 'newpw' | 'public';

interface Standin { url: string; proc: ChildProcess }

export const test = base.extend<{ as: (p: Persona) => Promise<void>; emit: (e: Record<string, unknown>) => Promise<Record<string, unknown>>; standinUrl: string }, { standin: Standin }>({
	standin: [async ({}, use, info) => {
		const proc = spawn(process.execPath, ['standin/server.ts'], {
			env: { ...process.env, PORT: '0', TGD_STANDIN_TEST: String(info.workerIndex) },
			stdio: ['ignore', 'pipe', 'inherit'],
		});
		const port = await new Promise<number>((resolve, reject) => {
			const t = setTimeout(() => reject(new Error('the stand-in did not start within 60 s')), 60_000);
			let out = '';
			proc.stdout!.on('data', (d) => {
				out += String(d);
				const m = /stand-in on :(\d+)/.exec(out);
				if (m) { clearTimeout(t); resolve(Number(m[1])); }
			});
			proc.on('exit', (code) => { clearTimeout(t); reject(new Error(`the stand-in exited with ${code}`)); });
		});
		await use({ url: `http://127.0.0.1:${port}`, proc });
		proc.kill();
		await new Promise((r) => proc.once('exit', r));
	}, { scope: 'worker', timeout: 90_000 }],

	standinUrl: async ({ standin }, use) => use(standin.url),

	// A test fails on any uncaught error in the page.
	page: async ({ page }, use) => {
		const errors: string[] = [];
		page.on('pageerror', (e) => errors.push(e.message));
		await use(page);
		expect(errors, 'uncaught page errors').toEqual([]);
	},

	// Every test starts from the fixtures as loaded, signed in as the
	// superadmin unless it asks otherwise.
	baseURL: async ({ standin }, use) => {
		const r = await fetch(`${standin.url}/api/_standin/reset`, { method: 'POST' });
		if (!r.ok) throw new Error(`reset failed: ${r.status}`);
		await use(standin.url);
	},

	as: async ({ context, standin }, use) => {
		await use(async (p) => {
			await context.addCookies([{ name: 'tgd_as', value: p, url: standin.url }]);
		});
	},

	emit: async ({ standin }, use) => {
		await use(async (e) => {
			const r = await fetch(`${standin.url}/api/_standin/emit`, { method: 'POST', body: JSON.stringify(e) });
			if (!r.ok) throw new Error(`emit failed: ${r.status}`);
			return (await r.json()) as Record<string, unknown>;
		});
	},
});

export { expect };
