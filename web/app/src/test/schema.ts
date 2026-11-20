// A small JSON Schema validator for the keywords webapi/openapi.yaml
// uses (type, enum, const, properties, required, additionalProperties,
// items, prefixItems, minItems, maxItems, minimum, maximum, allOf,
// oneOf with its discriminator, $ref), so tests can check data against
// the API description
// without a validator package. format is not checked. Unknown keywords
// fail loudly, so a new one in the description cannot be ignored;
// extensions (x-...) carry no validation and are skipped.
import { readFileSync } from 'node:fs';
import { load } from 'js-yaml';

export type Schema = Record<string, unknown>;
export interface Spec { paths: Record<string, Record<string, unknown>>; components: { schemas: Record<string, Schema> } }

const KNOWN = new Set(['type', 'enum', 'const', 'properties', 'required', 'additionalProperties', 'items', 'prefixItems', 'minItems', 'maxItems', 'minimum', 'maximum', 'allOf', 'oneOf', 'discriminator', '$ref', 'format', 'description', 'default', 'example', 'examples', 'title', 'deprecated', 'readOnly', 'writeOnly']);

export function loadSpec(path = new URL('../../../../webapi/openapi.yaml', import.meta.url)): Spec {
	return load(readFileSync(path, 'utf8')) as Spec;
}

const typeOf = (v: unknown) => (v === null ? 'null' : Array.isArray(v) ? 'array' : Number.isInteger(v) ? 'integer' : typeof v);
const fits = (t: string, v: unknown) => (t === 'number' ? typeof v === 'number' : t === 'integer' ? Number.isInteger(v) : typeOf(v) === t);

// The errors of `value` against `schema`, as "path: what" lines; empty
// when it conforms.
export function validate(spec: Spec, schema: Schema, value: unknown, path = '$'): string[] {
	for (const k of Object.keys(schema)) if (!KNOWN.has(k) && !k.startsWith('x-')) throw new Error(`${path}: schema keyword ${k} is not supported by src/test/schema.ts`);
	const errs: string[] = [];
	if (typeof schema.$ref === 'string') {
		const name = /^#\/components\/schemas\/(.+)$/.exec(schema.$ref)?.[1];
		const target = name ? spec.components.schemas[name] : undefined;
		if (!target) throw new Error(`${path}: unknown $ref ${schema.$ref}`);
		errs.push(...validate(spec, target, value, path));
	}
	if (schema.type !== undefined) {
		const types = Array.isArray(schema.type) ? (schema.type as string[]) : [schema.type as string];
		if (!types.some((t) => fits(t, value))) return [...errs, `${path}: is ${typeOf(value)}, wants ${types.join(' or ')}`];
	}
	if (schema.enum && !(schema.enum as unknown[]).some((e) => e === value)) errs.push(`${path}: ${JSON.stringify(value)} is not one of ${JSON.stringify(schema.enum)}`);
	if ('const' in schema && schema.const !== value) errs.push(`${path}: ${JSON.stringify(value)} is not ${JSON.stringify(schema.const)}`);
	if (typeof value === 'number') {
		if (typeof schema.minimum === 'number' && value < schema.minimum) errs.push(`${path}: ${value} < ${schema.minimum}`);
		if (typeof schema.maximum === 'number' && value > schema.maximum) errs.push(`${path}: ${value} > ${schema.maximum}`);
	}
	if (value && typeof value === 'object' && !Array.isArray(value)) {
		const obj = value as Record<string, unknown>;
		const props = (schema.properties ?? {}) as Record<string, Schema>;
		for (const r of (schema.required ?? []) as string[]) if (!(r in obj)) errs.push(`${path}: lacks ${r}`);
		for (const [k, v] of Object.entries(obj)) {
			if (props[k]) errs.push(...validate(spec, props[k]!, v, `${path}.${k}`));
			else if (schema.additionalProperties === false) errs.push(`${path}: has ${k}, which the description does not`);
			else if (schema.additionalProperties && typeof schema.additionalProperties === 'object') errs.push(...validate(spec, schema.additionalProperties as Schema, v, `${path}.${k}`));
		}
	}
	if (Array.isArray(value)) {
		const pre = (schema.prefixItems ?? []) as Schema[];
		value.forEach((v, i) => {
			const s = pre[i] ?? (schema.items as Schema | undefined);
			if (s) errs.push(...validate(spec, s, v, `${path}[${i}]`));
		});
		if (typeof schema.minItems === 'number' && value.length < schema.minItems) errs.push(`${path}: ${value.length} items, wants at least ${schema.minItems}`);
		if (typeof schema.maxItems === 'number' && value.length > schema.maxItems) errs.push(`${path}: ${value.length} items, wants at most ${schema.maxItems}`);
	}
	for (const s of (schema.allOf ?? []) as Schema[]) errs.push(...validate(spec, s, value, path));
	// A discriminator names the branch: its property must be one the
	// mapping knows, and the value must be that branch (oneOf still
	// checks that it is only that one).
	const disc = schema.discriminator as { propertyName: string; mapping?: Record<string, string> } | undefined;
	if (disc && value !== null && typeof value === 'object' && !Array.isArray(value)) {
		const key = (value as Record<string, unknown>)[disc.propertyName];
		const to = typeof key === 'string' ? disc.mapping?.[key] : undefined;
		if (!to) errs.push(`${path}.${disc.propertyName}: ${JSON.stringify(key)} is not a discriminator value`);
		else errs.push(...validate(spec, { $ref: to }, value, path));
	}
	if (schema.oneOf) {
		const results = (schema.oneOf as Schema[]).map((s) => validate(spec, s, value, path));
		const ok = results.filter((r) => !r.length).length;
		if (ok !== 1) errs.push(ok ? `${path}: matches ${ok} of oneOf, wants exactly 1` : `${path}: matches none of oneOf; closest: ${results.sort((a, b) => a.length - b.length)[0]!.slice(0, 3).join('; ')}`);
	}
	return errs;
}

// The schema of a JSON answer: paths[path][method].responses[status].
export function responseSchema(spec: Spec, path: string, method: string, status = '200'): Schema {
	const op = spec.paths[path]?.[method] as { responses?: Record<string, { content?: Record<string, { schema?: Schema }> }> } | undefined;
	const s = op?.responses?.[status]?.content?.['application/json']?.schema;
	if (!s) throw new Error(`no ${status} JSON answer for ${method.toUpperCase()} ${path}`);
	return s;
}
