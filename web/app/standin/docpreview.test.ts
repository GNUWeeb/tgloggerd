// The stand-in previews documents as Go does: render by the lower-case
// extension, the sample documents in the kinds group's chat, its Files
// tab and the files list, and Go's headers on their bytes.
import { describe, expect, it } from 'vitest';
import { BIG_TEXT_SIZE, DOC_CSP, DOC_SAMPLES, docBytes, docHeaders, docRender, fileRender } from './docpreview.ts';
import { groupTabApi, pageData } from './fixtures.ts';
import { KINDS_GROUP } from './kinds.ts';

const WANT: Record<string, string> = { 'screenshot.png': 'image', 'diagram.svg': 'image', 'demo.mp4': 'video', 'talk.mp3': 'audio', 'build.log': 'text', 'page.html': 'text', 'manual.pdf': 'pdf', 'sources.zip': 'file', 'trace-big.log': 'text' };

describe('stand-in document previews', () => {
	it('renders a document by its lower-case extension', () => {
		expect(['PNG', 'svg', 'apng', 'ico'].map(docRender)).toEqual(['image', 'image', 'image', 'image']);
		expect(['mp4', 'MOV', 'ogv'].map(docRender)).toEqual(['video', 'video', 'video']);
		expect(['flac', 'opus'].map(docRender)).toEqual(['audio', 'audio']);
		expect(['html', 'htm', 'patch', 'kt', 'Log'].map(docRender)).toEqual(['text', 'text', 'text', 'text', 'text']);
		expect(docRender('pdf')).toBe('pdf');
		expect(['zip', 'gz', 'exe', '', undefined].map(docRender)).toEqual(['file', 'file', 'file', 'file', 'file']);
		expect(fileRender('photo', 'jpg')).toBe('image');
		expect(fileRender('sticker', 'tgs')).toBe('lottie');
		expect(fileRender('document', 'pdf')).toBe('pdf');
	});

	it('has the sample documents in the chat, the Files tab and the files list, each with its render', () => {
		const files = (groupTabApi('files', String(KINDS_GROUP.id), new URLSearchParams(), 'superadmin')[1] as { rows: { name: string; render?: string; url: string | null }[] }).rows;
		for (const d of DOC_SAMPLES) {
			const row = files.find((r) => r.name === d.name);
			expect(row?.render, d.name).toBe(WANT[d.name]);
			const list = (pageData(`/files?view=list&q=${d.name}`, 'superadmin').props as { rows: { name: string; render?: string; message: { scope: string; id: number } | null }[] }).rows;
			const f = list.find((r) => r.name === d.name)!;
			expect(f.render, d.name).toBe(WANT[d.name]);
			const msg = (pageData(`/messages/${f.message!.scope}/${f.message!.id}`, 'superadmin').props as { message: { content_type: string; media: { render: string; name: string; url: string } } }).message;
			expect(msg.content_type).toBe('document');
			expect(msg.media.name).toBe(d.name);
			expect(msg.media.render, d.name).toBe(WANT[d.name]);
		}
	});

	it('serves the text and PDF samples by their own URLs with Go\'s headers', () => {
		expect(docBytes(995016, 'html')).toContain('<script>');
		expect(docBytes(995019, 'log').length).toBe(BIG_TEXT_SIZE);
		expect(BIG_TEXT_SIZE).toBeGreaterThan(512 * 1024);
		const text = docHeaders(995015, 'log', false);
		expect(text['content-type']).toBe('text/plain; charset=utf-8');
		expect(text['x-content-type-options']).toBe('nosniff');
		expect(text['content-security-policy']).toBe(DOC_CSP);
		expect(text['content-disposition']).toBe('inline; filename="build.log"');
		expect(docHeaders(995016, 'html', false)['content-type']).toBe('text/plain; charset=utf-8');
		const pdf = docHeaders(995017, 'pdf', false);
		expect(pdf['content-type']).toBe('application/pdf');
		expect(pdf['content-security-policy']).toBeUndefined();
		expect(docHeaders(995012, 'svg', false)['content-security-policy']).toBe(DOC_CSP);
		expect(docHeaders(995015, 'log', true)['content-disposition']).toBe('attachment; filename="build.log"');
	});
});
