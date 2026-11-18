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
import { readdirSync, readFileSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
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

if (import.meta.main) {
	const root = fileURLToPath(new URL('..', import.meta.url));
	const build = buildId(root);
	const env = { ...process.env, TGD_BUILD: build };
	const vite = (args: string[]) => execFileSync('node_modules/.bin/vite', args, { cwd: root, env, stdio: 'inherit' });
	vite(['build']);
	vite(['build', '--ssr', 'src/entry-server.tsx', '--outDir', 'dist/server']);
	writeBuildFile(join(root, 'dist', 'client'), build);
	console.log(`build ${build}`);
}
