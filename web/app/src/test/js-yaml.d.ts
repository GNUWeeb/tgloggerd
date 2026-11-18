// js-yaml ships no types; the tests use only load().
declare module 'js-yaml' {
	export function load(text: string): unknown;
}
