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

if (cluster.isPrimary && WORKERS > 1) {
	// A socket left by a crash would make listen() fail.
	if (existsSync(SOCKET)) unlinkSync(SOCKET);
	let chmodded = false;
	for (let i = 0; i < WORKERS; i++) cluster.fork();
	cluster.on('listening', () => {
		if (!chmodded) { chmodSync(SOCKET, 0o660); chmodded = true; log({ msg: 'listening', socket: SOCKET, workers: WORKERS }); }
	});
	cluster.on('exit', (w, code) => {
		log({ msg: 'worker exited, restarting', worker: w.process.pid, code });
		cluster.fork();
	});
	const stop = () => { for (const w of Object.values(cluster.workers || {})) w?.kill(); process.exit(0); };
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
