import { afterEach, describe, expect, it } from 'vitest';
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { BUILD_FILE, BUILDS_FILE, buildId, INPUTS, inputFiles, mergeClient, writeBuildFile } from './build.ts';

const dirs: string[] = [];
afterEach(() => {
	for (const d of dirs.splice(0)) rmSync(d, { recursive: true, force: true });
});

// A source tree with every input and a file that is not one.
function tree(): string {
	const d = mkdtempSync(join(tmpdir(), 'tgd-build-'));
	dirs.push(d);
	for (const f of INPUTS.filter((f) => f !== 'src')) writeFileSync(join(d, f), `${f}\n`);
	mkdirSync(join(d, 'src', 'routes'), { recursive: true });
	writeFileSync(join(d, 'src', 'App.tsx'), 'app\n');
	writeFileSync(join(d, 'src', 'routes', 'login.tsx'), 'login\n');
	writeFileSync(join(d, 'README'), 'not an input\n');
	return d;
}

describe('buildId', () => {
	it('hashes the inputs, the same for the same tree', () => {
		const a = tree(), b = tree();
		expect(buildId(a, {})).toMatch(/^[0-9a-f]{12}$/);
		expect(buildId(a, {})).toBe(buildId(b, {}));
		expect(inputFiles(a)).toEqual(['package-lock.json', 'package.json', 'src/App.tsx', 'src/routes/login.tsx', 'tsconfig.json', 'vite.config.ts']);
	});

	it('changes with any input and with nothing else', () => {
		const d = tree();
		const before = buildId(d, {});
		writeFileSync(join(d, 'README'), 'edited\n');
		expect(buildId(d, {})).toBe(before);
		// Tests are not bundled.
		mkdirSync(join(d, 'src', 'test'), { recursive: true });
		writeFileSync(join(d, 'src', 'test', 'page.tsx'), 'helper\n');
		writeFileSync(join(d, 'src', 'routes', 'login.test.tsx'), 'test\n');
		expect(buildId(d, {})).toBe(before);
		writeFileSync(join(d, 'src', 'routes', 'login.tsx'), 'login 2\n');
		const src = buildId(d, {});
		expect(src).not.toBe(before);
		writeFileSync(join(d, 'package-lock.json'), '{}\n');
		expect(buildId(d, {})).not.toBe(src);
	});

	it('takes TGD_BUILD when set, and refuses a malformed one', () => {
		const d = tree();
		expect(buildId(d, { TGD_BUILD: 'v1.2-rc.3' })).toBe('v1.2-rc.3');
		expect(() => buildId(d, { TGD_BUILD: 'a b' })).toThrow(/TGD_BUILD/);
		expect(() => buildId(d, { TGD_BUILD: '"><script>' })).toThrow(/TGD_BUILD/);
	});
});

describe('writeBuildFile', () => {
	it('writes the id as the Go service reads it, leaving no temporary file', () => {
		const d = tree();
		writeBuildFile(d, 'abc123');
		writeBuildFile(d, 'def456');
		expect(JSON.parse(readFileSync(join(d, BUILD_FILE), 'utf8'))).toEqual({ build: 'def456' });
		expect(readdirSync(d).filter((f) => f.includes('.tmp'))).toEqual([]);
	});
});

describe('mergeClient', () => {
	// A client bundle as Vite writes it: hashed assets and a manifest.
	const bundle = (files: string[], manifest: string) => {
		const d = mkdtempSync(join(tmpdir(), 'tgd-client-'));
		dirs.push(d);
		mkdirSync(join(d, 'assets'));
		mkdirSync(join(d, '.vite'));
		for (const f of files) writeFileSync(join(d, 'assets', f), f);
		writeFileSync(join(d, '.vite', 'manifest.json'), manifest);
		return d;
	};
	const dist = () => { const d = mkdtempSync(join(tmpdir(), 'tgd-dist-')); dirs.push(d); return d; };
	const assets = (d: string) => readdirSync(join(d, 'assets')).sort();

	it('adds a build beside the one before, and replaces the manifest and build id', () => {
		const d = dist();
		mergeClient(bundle(['entry-a1.js', 'style-s1.css'], 'm1'), d, 'b1');
		mergeClient(bundle(['entry-a2.js', 'style-s1.css'], 'm2'), d, 'b2');
		expect(assets(d)).toEqual(['entry-a1.js', 'entry-a2.js', 'style-s1.css']);
		expect(readFileSync(join(d, '.vite', 'manifest.json'), 'utf8')).toBe('m2');
		expect(JSON.parse(readFileSync(join(d, BUILD_FILE), 'utf8'))).toEqual({ build: 'b2' });
	});

	it('removes assets once no kept build uses them', () => {
		const d = dist();
		mergeClient(bundle(['a1.js', 'shared.css'], 'm'), d, 'b1', 2);
		mergeClient(bundle(['a2.js', 'shared.css'], 'm'), d, 'b2', 2);
		const { removed } = mergeClient(bundle(['a3.js', 'shared.css'], 'm'), d, 'b3', 2);
		expect(removed).toEqual(['assets/a1.js']);
		expect(assets(d)).toEqual(['a2.js', 'a3.js', 'shared.css']);
		expect(JSON.parse(readFileSync(join(d, BUILDS_FILE), 'utf8')).map((b: { build: string }) => b.build)).toEqual(['b3', 'b2']);
	});

	it('counts files from before the record as one earlier build', () => {
		const d = dist();
		mkdirSync(join(d, 'assets'));
		writeFileSync(join(d, 'assets', 'old.js'), 'old');
		mergeClient(bundle(['new1.js'], 'm'), d, 'b1', 2);
		expect(assets(d)).toEqual(['new1.js', 'old.js']);
		mergeClient(bundle(['new2.js'], 'm'), d, 'b2', 2);
		expect(assets(d)).toEqual(['new1.js', 'new2.js']);
	});

	it('rebuilding the same id keeps one record of it, and leaves no temporary file', () => {
		const d = dist();
		mergeClient(bundle(['a1.js'], 'm'), d, 'b1', 2);
		mergeClient(bundle(['a1.js'], 'm'), d, 'b1', 2);
		expect(JSON.parse(readFileSync(join(d, BUILDS_FILE), 'utf8'))).toEqual([{ build: 'b1', files: ['assets/a1.js'] }]);
		const all = (x: string): string[] => readdirSync(x, { withFileTypes: true }).flatMap((e) => (e.isDirectory() ? all(join(x, e.name)) : [e.name]));
		expect(all(d).filter((f) => f.endsWith('.tmp'))).toEqual([]);
	});
});
