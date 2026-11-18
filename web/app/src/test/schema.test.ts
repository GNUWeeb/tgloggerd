import { describe, expect, it } from 'vitest';
import { validate, type Spec } from './schema.ts';

const spec: Spec = {
	paths: {},
	components: {
		schemas: {
			Base: { type: 'object', required: ['id'], properties: { id: { type: 'integer', minimum: 1 } } },
			Thing: { allOf: [{ $ref: '#/components/schemas/Base' }, { type: 'object', required: ['kind'], properties: { kind: { enum: ['a', 'b'] }, note: { type: ['string', 'null'] } } }] },
			Closed: { type: 'object', additionalProperties: false, properties: { x: { const: 1 } } },
			Pair: { type: 'array', prefixItems: [{ type: 'string' }, { type: 'integer' }], minItems: 2, maxItems: 2 },
			Cat: { type: 'object', required: ['kind', 'meow'], properties: { kind: { enum: ['cat'] }, meow: { type: 'boolean' } } },
			Dog: { type: 'object', required: ['kind', 'bark'], properties: { kind: { enum: ['dog', 'puppy'] }, bark: { type: 'integer' } } },
			Pet: { oneOf: [{ $ref: '#/components/schemas/Cat' }, { $ref: '#/components/schemas/Dog' }], discriminator: { propertyName: 'kind', mapping: { cat: '#/components/schemas/Cat', dog: '#/components/schemas/Dog', puppy: '#/components/schemas/Dog' } } },
			Either: { oneOf: [{ type: 'object', required: ['a'], properties: { a: { type: 'string' } } }, { type: 'object', required: ['b'], properties: { b: { type: 'string' } } }] },
		},
	},
};
const v = (name: string, value: unknown) => validate(spec, { $ref: `#/components/schemas/${name}` }, value);

describe('schema validator', () => {
	it('accepts conforming data', () => {
		expect(v('Thing', { id: 3, kind: 'a', note: null })).toEqual([]);
		expect(v('Pair', ['x', 2])).toEqual([]);
		expect(v('Either', { a: 'y' })).toEqual([]);
	});

	it('names each mismatch with its path', () => {
		expect(v('Thing', { id: 0, kind: 'c', note: 5 })).toEqual(['$.id: 0 < 1', '$.kind: "c" is not one of ["a","b"]', '$.note: is integer, wants string or null']);
		expect(v('Thing', { kind: 'a' })).toEqual(['$: lacks id']);
		expect(v('Closed', { x: 1, y: 2 })).toEqual(['$: has y, which the description does not']);
		expect(v('Pair', ['x'])).toEqual(['$: 1 items, wants at least 2']);
		expect(v('Either', { c: 1 })[0]).toMatch(/matches none of oneOf/);
		expect(v('Either', { a: 'x', b: 'y' })[0]).toMatch(/matches 2 of oneOf/);
	});

	it('checks a discriminated oneOf against the branch its property names', () => {
		expect(v('Pet', { kind: 'puppy', bark: 2 })).toEqual([]);
		expect(v('Pet', { kind: 'fox', bark: 2 })[0]).toBe('$.kind: "fox" is not a discriminator value');
		// The mapped branch's own complaint, not only "matches none".
		expect(v('Pet', { kind: 'cat', bark: 2 })).toContain('$: lacks meow');
	});

	it('refuses a keyword it does not check', () => {
		expect(() => validate(spec, { pattern: '^a' }, 'a')).toThrow(/pattern is not supported/);
	});
});
