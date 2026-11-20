// The render service: serves protocol version 1 on a Unix socket for the
// Go service (docs/design/web-ssr.md sections 4 and 8).
//
//   RENDER_SOCKET   socket path (default /run/tgloggerd-render/render.sock)
//   RENDER_WORKERS  worker processes (default 2)
//   RENDER_ENTRY    built render entry (default ../dist/server/entry-server.js)
import cluster from 'node:cluster';
import { chmodSync, existsSync, unlinkSync } from 'node:fs';
import { createServer } from 'node:http';
import { fileURLToPath } from 'node:url';
import { makeHandler, type Renderer } from './handler.ts';

const SOCKET = process.env.RENDER_SOCKET || '/run/tgloggerd-render/render.sock';
const WORKERS = Math.max(1, Number(process.env.RENDER_WORKERS) || 2);
const ENTRY = process.env.RENDER_ENTRY || fileURLToPath(new URL('../dist/server/entry-server.js', import.meta.url));

const log = (e: Record<string, unknown>) => process.stdout.write(JSON.stringify({ t: new Date().toISOString(), pid: process.pid, ...e }) + '\n');

// Without a built entry every worker would die at its import and be
// forked again at once; exit instead and let the container's restart
// policy retry, with its back-off, until the build is there.
if (cluster.isPrimary && !existsSync(ENTRY)) {
	log({ msg: 'no render entry; run npm run build', entry: ENTRY });
	process.exit(1);
}

if (cluster.isPrimary && WORKERS > 1) {
	// A socket left by a crash would make listen() fail.
	if (existsSync(SOCKET)) unlinkSync(SOCKET);
	let chmodded = false;
	for (let i = 0; i < WORKERS; i++) cluster.fork();
	cluster.on('listening', () => {
		if (!chmodded) { chmodSync(SOCKET, 0o660); chmodded = true; log({ msg: 'listening', socket: SOCKET, workers: WORKERS }); }
	});
	// A worker that dies is replaced; workers that keep dying (an entry
	// that is there but fails to load, a bad build) would be replaced in
	// a busy loop, so after CRASHES deaths within CRASH_WINDOW_MS the
	// primary gives up and exits 1, and the container's restart policy
	// retries with its back-off.
	const CRASHES = Math.max(3, 2 * WORKERS);
	const CRASH_WINDOW_MS = 10_000;
	const deaths: number[] = [];
	let stopping = false;
	cluster.on('exit', (w, code, signal) => {
		if (stopping) return;
		const now = Date.now();
		deaths.push(now);
		while (deaths.length && now - deaths[0]! > CRASH_WINDOW_MS) deaths.shift();
		if (deaths.length >= CRASHES) {
			log({ msg: 'workers keep exiting; giving up', deaths: deaths.length, window_ms: CRASH_WINDOW_MS, code, signal });
			stopping = true;
			for (const x of Object.values(cluster.workers || {})) x?.kill();
			process.exit(1);
		}
		log({ msg: 'worker exited, restarting', worker: w.process.pid, code, signal });
		cluster.fork();
	});
	// Stop: no more replacements; each worker finishes the renders it has
	// (it closes its server on SIGTERM), then the primary exits.
	const stop = () => {
		if (stopping) return;
		stopping = true;
		const left = new Set(Object.values(cluster.workers || {}).filter((w) => w !== undefined));
		if (!left.size) process.exit(0);
		cluster.on('exit', (w) => { left.delete(w); if (!left.size) process.exit(0); });
		for (const w of left) w.process.kill('SIGTERM');
		setTimeout(() => process.exit(0), 10_000).unref();
	};
	process.on('SIGTERM', stop);
	process.on('SIGINT', stop);
} else {
	const mod = (await import(ENTRY)) as Renderer & { BUILD: string };
	const renderer: Renderer = { build: mod.BUILD, render: mod.render };
	if (WORKERS === 1 && existsSync(SOCKET)) unlinkSync(SOCKET);
	const server = createServer(makeHandler(renderer, log));
	server.listen(SOCKET, () => {
		if (WORKERS === 1) { chmodSync(SOCKET, 0o660); log({ msg: 'listening', socket: SOCKET, build: renderer.build }); }
	});
	process.on('SIGTERM', () => server.close(() => process.exit(0)));
}
