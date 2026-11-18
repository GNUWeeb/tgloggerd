import { afterEach, describe, expect, it } from 'vitest';
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { BUILD_FILE, buildId, INPUTS, inputFiles, writeBuildFile } from './build.ts';

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
