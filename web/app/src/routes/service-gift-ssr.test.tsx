// The server-rendered message page of a gift: no block element inside
// a <p> or another element that holds only phrasing content. A browser
// parsing such HTML closes the <p> before the block, so the page it
// builds no longer matches the client's tree and hydration fails.
import { describe, expect, it } from 'vitest';
import { render } from '../entry-server.tsx';
import { fixture } from '../test/page.tsx';
import { ALL, KINDS_GROUP, sampleOf } from '../../standin/kinds.ts';
import { chatStream } from '../../../mockup/src/data/messages.js';
import type { MessageProps } from './message.tsx';

// Each block element opened while a <p> is open, as a parser meets it.
const BLOCK = new Set(['address', 'article', 'aside', 'blockquote', 'details', 'div', 'dl', 'fieldset', 'figcaption', 'figure', 'footer', 'form', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'header', 'hr', 'main', 'menu', 'nav', 'ol', 'p', 'pre', 'section', 'table', 'ul']);
const VOID = new Set(['area', 'br', 'col', 'embed', 'hr', 'img', 'input', 'link', 'meta', 'source', 'track', 'wbr']);
function blocksInP(html: string): string[] {
	const open: string[] = [];
	const bad: string[] = [];
	for (const [, close, name] of html.matchAll(/<(\/?)([a-zA-Z][a-zA-Z0-9-]*)[^>]*>/g)) {
		const tag = name!.toLowerCase();
		if (close) { const at = open.lastIndexOf(tag); if (at >= 0) open.length = at; continue; }
		if (BLOCK.has(tag) && open.includes('p')) bad.push(`<${tag}> in <p>`);
		if (!VOID.has(tag)) open.push(tag);
	}
	return bad;
}

type Raw = { id: number; message_id: number; chat: { id: number; kind: string } };
const raw = (type: string) => (chatStream(KINDS_GROUP) as Raw[]).find((m) => sampleOf(m)?.service?.(ALL)?.type === type)!;

describe('a gift\'s server-rendered message page', () => {
	it('puts no block inside a <p>, with or without the service text', () => {
		for (const type of ['upgraded_gift', 'gift', 'gifted_premium']) {
			const page = fixture<MessageProps>(`/messages/group/${raw(type).id}`);
			for (const text of [page.props.message.text, '']) {
				const html = render({ ...page, props: { ...page.props, message: { ...page.props.message, text } } }).html;
				expect(html, `${type} ${text ? 'with' : 'without'} text`).toContain('c-gift');
				expect(blocksInP(html), `${type} ${text ? 'with' : 'without'} text`).toEqual([]);
			}
		}
	});

	it('the check finds a block inside a <p>', () => {
		expect(blocksInP('<p>a<span><div>b</div></span></p>')).toEqual(['<div> in <p>']);
		expect(blocksInP('<div><p>a</p><div>b</div></div>')).toEqual([]);
	});
});
