// Fails when src/api/generated/ is not what `npm run gen:api` makes
// from webapi/openapi.yaml, so the client cannot drift from the
// description. Runs the same pinned generator; needs no network.
import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { join } from 'node:path';

const committed = 'src/api/generated/schema.d.ts';
const dir = mkdtempSync(join('node_modules', '.check-api-'));
try {
	const fresh = join(dir, 'schema.d.ts');
	execFileSync('node_modules/.bin/openapi-typescript', ['../../webapi/openapi.yaml', '-o', fresh], { stdio: ['ignore', 'ignore', 'inherit'] });
	if (readFileSync(fresh, 'utf8') !== readFileSync(committed, 'utf8')) {
		console.error(`${committed} is out of date with webapi/openapi.yaml; run npm run gen:api and commit the result.`);
		process.exitCode = 1;
	} else console.log(`${committed} matches webapi/openapi.yaml.`);
} finally {
	rmSync(dir, { recursive: true, force: true });
}
