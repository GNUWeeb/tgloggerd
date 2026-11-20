// Loaded before every test file (vitest setupFiles). Tests fake fetch
// with vi.stubGlobal, and vi.unstubAllGlobals() puts back whatever was
// there before: without this, the environment's real fetch. A request a
// timer, poll or effect starts after its test ended would then go to
// the network (happy-dom: ECONNREFUSED to localhost, printed after the
// run). With this, the fetch put back refuses every request: it never
// reaches the network, it rejects, and it prints a warning (once per URL
// per test file), so a stray request shows in the run output even when
// the app catches the rejection.
export class NetworkRefused extends Error {
	constructor(url: string) {
		super(`test made a real network request: ${url} (fake fetch in the test, or wait for the work it starts)`);
		this.name = 'NetworkRefused';
	}
}

export const refusedUrls: string[] = [];
const warned = new Set<string>();

const refuse = async (input: RequestInfo | URL) => {
	const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
	refusedUrls.push(url);
	if (!warned.has(url)) {
		warned.add(url);
		console.warn(`[no-network] refused a real request: ${url}`);
	}
	throw new NetworkRefused(url);
};

globalThis.fetch = refuse as typeof fetch;
if (typeof window !== 'undefined') window.fetch = refuse as typeof fetch;
