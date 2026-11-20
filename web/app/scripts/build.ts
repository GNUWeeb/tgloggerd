// Builds the client bundle (dist/client) and the render entry
// (dist/server) with one build id, and records that id in
// dist/client/build.json for the Go service (docs/design/web-ssr.md
// section 6). The render service refuses a render for another build, so
// both bundles must carry the same id; building them in one run is what
// guarantees it.
//
// The id is TGD_BUILD when set, else a hash of the inputs that make up
// the bundles: the same sources and lock file give the same id on any
// machine, without git (the build container has none).
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { copyFileSync, existsSync, mkdirSync, readdirSync, readFileSync, renameSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

// What the bundles are built from, relative to web/app; directories
// are taken whole.
export const INPUTS = ['package.json', 'package-lock.json', 'tsconfig.json', 'vite.config.ts', 'src'];

// The file in the client bundle's directory the Go service reads the
// build id from (webapi/internal/render, BuildFile).
export const BUILD_FILE = 'build.json';

// A build id goes into a JSON body and a log line; keep it plain.
const ID = /^[A-Za-z0-9._-]{1,64}$/;

// Tests live beside the sources but are never bundled; a change to one
// must not change the build id.
const NOT_BUNDLED = /(^|\/)test\/|\.test\.tsx?$/;

// The input files under root, as sorted paths with '/' separators.
export function inputFiles(root: string): string[] {
	const out: string[] = [];
	const walk = (rel: string) => {
		if (statSync(join(root, rel)).isDirectory()) {
			for (const name of readdirSync(join(root, rel)).sort()) walk(`${rel}/${name}`);
		} else if (!NOT_BUNDLED.test(rel)) out.push(rel);
	};
	for (const rel of INPUTS) walk(rel);
	return out.sort();
}

export function buildId(root: string, env: NodeJS.ProcessEnv = process.env): string {
	if (env.TGD_BUILD) {
		if (!ID.test(env.TGD_BUILD)) throw new Error(`TGD_BUILD ${JSON.stringify(env.TGD_BUILD)} is not 1 to 64 of [A-Za-z0-9._-]`);
		return env.TGD_BUILD;
	}
	const h = createHash('sha256');
	for (const rel of inputFiles(root)) {
		const data = readFileSync(join(root, rel));
		// Name and length first, so no two file sets hash alike.
		h.update(`${rel}\0${data.length}\0`);
		h.update(data);
	}
	return h.digest('hex').slice(0, 12);
}

// Written last and renamed into place: a build.json is only ever there
// for a complete build.
export function writeBuildFile(dir: string, build: string): void {
	const tmp = join(dir, `.${BUILD_FILE}.tmp`);
	writeFileSync(tmp, JSON.stringify({ build }) + '\n');
	renameSync(tmp, join(dir, BUILD_FILE));
}

// How many builds' assets a rebuild keeps: the new one and the ones
// before it, for pages loaded from them that are still open (their
// script asks for its own hashed files) until those reload.
export const KEEP_BUILDS = 3;

// The record of which assets each build used, newest first, next to
// build.json.
export const BUILDS_FILE = '.builds.json';

interface BuildRecord { build: string; files: string[] }

const listFiles = (dir: string, rel = ''): string[] => {
	if (!existsSync(join(dir, rel))) return [];
	return readdirSync(join(dir, rel), { withFileTypes: true }).flatMap((e) => {
		const r = rel ? `${rel}/${e.name}` : e.name;
		return e.isDirectory() ? listFiles(dir, r) : [r];
	});
};

// Merges a client bundle built into `staging` into `dist`. Vite empties
// its output directory on every build, so building straight into dist
// deleted the hashed files every page loaded earlier links; a page
// still open then lost its scripts and styles until it reloaded, and
// the Go service, which reads the manifest at start, linked missing
// files until it restarted. Here the new assets are added beside the
// old ones (their names carry a content hash, so they never clash),
// then the manifest is replaced, and build.json is written last, each
// by a rename. Assets that none of the last KEEP_BUILDS builds uses are
// removed; files from before this record existed count as one earlier
// build.
export function mergeClient(staging: string, dist: string, build: string, keep = KEEP_BUILDS): { removed: string[] } {
	mkdirSync(dist, { recursive: true });
	const assets = listFiles(staging).filter((f) => f.startsWith('assets/'));
	for (const f of assets) {
		const to = join(dist, f);
		if (existsSync(to)) continue; // same name, same content
		mkdirSync(dirname(to), { recursive: true });
		copyFileSync(join(staging, f), `${to}.tmp`);
		renameSync(`${to}.tmp`, to);
	}
	// Everything else (the manifest, anything Vite adds) is replaced.
	for (const f of listFiles(staging).filter((x) => !x.startsWith('assets/'))) {
		const to = join(dist, f);
		mkdirSync(dirname(to), { recursive: true });
		copyFileSync(join(staging, f), `${to}.tmp`);
		renameSync(`${to}.tmp`, to);
	}

	let builds: BuildRecord[] = [];
	try { builds = JSON.parse(readFileSync(join(dist, BUILDS_FILE), 'utf8')) as BuildRecord[]; } catch { /* first merge */ }
	const recorded = new Set(builds.flatMap((b) => b.files));
	const earlier = listFiles(dist).filter((f) => f.startsWith('assets/') && !recorded.has(f) && !assets.includes(f));
	if (earlier.length) builds.unshift({ build: 'earlier', files: earlier });
	builds = [{ build, files: assets }, ...builds.filter((b) => b.build !== build)].slice(0, keep);
	const kept = new Set(builds.flatMap((b) => b.files));
	const removed = listFiles(dist).filter((f) => f.startsWith('assets/') && !kept.has(f));
	for (const f of removed) rmSync(join(dist, f), { force: true });
	writeFileSync(join(dist, `${BUILDS_FILE}.tmp`), JSON.stringify(builds) + '\n');
	renameSync(join(dist, `${BUILDS_FILE}.tmp`), join(dist, BUILDS_FILE));
	writeBuildFile(dist, build);
	return { removed };
}

if (import.meta.main) {
	const root = fileURLToPath(new URL('..', import.meta.url));
	const build = buildId(root);
	const env = { ...process.env, TGD_BUILD: build };
	const vite = (args: string[]) => execFileSync('node_modules/.bin/vite', args, { cwd: root, env, stdio: 'inherit' });
	// The client goes to a staging directory first and is then merged
	// into dist/client, so the files of earlier builds stay for the pages
	// still open on them (mergeClient).
	const staging = join(root, 'dist', '.client-next');
	vite(['build', '--outDir', staging, '--emptyOutDir']);
	vite(['build', '--ssr', 'src/entry-server.tsx', '--outDir', 'dist/server']);
	const { removed } = mergeClient(staging, join(root, 'dist', 'client'), build);
	rmSync(staging, { recursive: true, force: true });
	console.log(`build ${build}${removed.length ? `; removed ${removed.length} files no kept build uses` : ''}`);
}
