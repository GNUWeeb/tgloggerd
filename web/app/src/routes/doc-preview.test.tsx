// @vitest-environment happy-dom
// Documents previewed by their extension (web/docs/media.md,
// "Documents"), on the stand-in's sample documents: a picture opens in
// the photo viewer (an svg only through an <img>), a video and a song
// play with the chat's players, a text file shows its first 512 KiB as
// text on demand (an HTML page as its source), a PDF opens in a new
// tab, a zip stays a file, and each keeps a download (?download=1).
import { beforeEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { readFileSync } from 'node:fs';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { KINDS_GROUP, docSampleMessages } from '../../standin/kinds.ts';
import { docBytes } from '../../standin/docpreview.ts';
import { downloadHref } from '../ui/docpreview.tsx';
import type { ChatProps } from './chat.tsx';
import type { MessageProps } from './message.tsx';
import type { FilesProps } from './files.tsx';
import type { GroupFilesPage } from '../api/groupfiles.ts';

beforeEach(setup);

// The files' bytes as the stand-in serves them.
const files = () => fakeApi('superadmin', {
	'GET /files/:token': (c) => {
		const [, id, ext] = /standin-(\d+)\.(\w+)$/.exec(c.path)!;
		return new Response(docBytes(Number(id), ext!), { headers: { 'content-type': 'text/plain; charset=utf-8' } });
	},
});
const fileCalls = (calls: ReturnType<typeof fakeApi>) => callsTo(calls, 'GET', /^\/files\//);
const viewer = () => screen.queryByRole('dialog', { name: 'Media viewer' });
// Nothing a file holds may become live markup.
const NO_LIVE = 'script, iframe, object, embed, [srcdoc]';

describe('a document in the chat', () => {
	const open = (name: string) => {
		const { m } = docSampleMessages<{ chat: { id: number; kind: string }; message_id: number }>().find((d) => d.doc.name === name)!;
		const url = `/groups/${KINDS_GROUP.id}/chat?at=${m.message_id}`;
		const media = fixture<ChatProps>(url).props.messages.find((x) => x.message_id === m.message_id)!.media!;
		const calls = files();
		renderPage(url);
		const row = document.getElementById(`m${m.message_id}`)!;
		return { row, media, calls, c: within(row) };
	};

	it('draws a PNG as a photo that opens in the viewer, with its download', () => {
		const { c, media } = open('screenshot.png');
		const pic = c.getByRole('link', { name: 'Photo' });
		const img = pic.querySelector('img')!;
		expect(img.getAttribute('src')).toBe(media.thumb ?? media.url);
		// Its thumb is the whole file: loaded lazily, drawn small.
		expect([img.getAttribute('loading'), img.getAttribute('decoding')]).toEqual(['lazy', 'async']);
		expect(pic.closest('.c-docprev')).toBeTruthy();
		expect(c.getByRole('link', { name: 'Download' }).getAttribute('href')).toBe(`${media.url}?download=1`);
		fireEvent.click(pic);
		expect(viewer()!.querySelector('img')!.getAttribute('src')).toBe(media.url);
	});

	it('draws an svg only through an <img>', () => {
		const { row, c, media } = open('diagram.svg');
		expect(media.url).toMatch(/\.svg$/);
		const imgs = [...row.querySelectorAll(`[src="${media.url}"], [data="${media.url}"], [href="${media.url}"]`)];
		expect(imgs.length).toBeGreaterThan(0);
		for (const el of imgs) expect(['IMG', 'A']).toContain(el.tagName);
		expect(imgs.some((el) => el.tagName === 'IMG')).toBe(true);
		expect(row.querySelector(NO_LIVE)).toBeNull();
		expect(c.getByRole('link', { name: 'Download' }).getAttribute('href')).toBe(`${media.url}?download=1`);
	});

	it('plays an mp4 as a video and an mp3 with the audio player', () => {
		const v = open('demo.mp4');
		expect(v.c.getByRole('link', { name: 'Video' })).toBeTruthy();
		expect(v.c.getByRole('link', { name: 'Download' }).getAttribute('href')).toBe(`${v.media.url}?download=1`);
		cleanup();
		const a = open('talk.mp3');
		expect(a.row.querySelector('audio')!.getAttribute('src')).toBe(a.media.url);
		expect(a.row.querySelector('a.c-doc')!.getAttribute('href')).toBe(`${a.media.url}?download=1`);
	});

	it('shows a log\'s text only when asked, from the first 512 KiB of the file', async () => {
		const { c, calls, media, row } = open('build.log');
		expect(fileCalls(calls)).toHaveLength(0);
		fireEvent.click(c.getByRole('button', { name: 'Show text' }));
		const pre = await waitFor(() => { const el = row.querySelector('pre'); expect(el).toBeTruthy(); return el!; });
		expect(pre.textContent).toBe(docBytes(995015, 'log'));
		const got = fileCalls(calls);
		expect(got).toHaveLength(1);
		expect(got[0]!.path).toBe(media.url);
		expect(got[0]!.headers.get('range')).toBe(`bytes=0-${512 * 1024}`);
		expect(c.queryByText(/Showing the first 512 KiB/)).toBeNull();
		expect(row.querySelector('a.c-doc')!.getAttribute('href')).toBe(`${media.url}?download=1`);
	});

	it('shows an HTML page as its source, never as markup', async () => {
		const { c, row } = open('page.html');
		fireEvent.click(c.getByRole('button', { name: 'Show text' }));
		const pre = await waitFor(() => { const el = row.querySelector('pre'); expect(el).toBeTruthy(); return el!; });
		expect(pre.textContent).toContain('<script>document.title = "doc ran"</script>');
		expect(row.querySelector(NO_LIVE)).toBeNull();
		expect(row.querySelector('h1')).toBeNull();
		expect(document.title).not.toBe('doc ran');
	});

	it('cuts a text over 512 KiB, with a note and the download', async () => {
		const { c, row, media } = open('trace-big.log');
		fireEvent.click(c.getByRole('button', { name: 'Show text' }));
		const note = await c.findByText(/Showing the first 512 KiB/);
		expect(row.querySelector('pre')!.textContent!.length).toBe(512 * 1024);
		expect(within(note).getByRole('link', { name: 'Download' }).getAttribute('href')).toBe(`${media.url}?download=1`);
	});

	it('opens a PDF in a new tab, never embedded', () => {
		const { c, row, media } = open('manual.pdf');
		const a = c.getByRole('link', { name: 'Open' });
		expect(a.getAttribute('href')).toBe(media.url);
		expect(a.getAttribute('target')).toBe('_blank');
		expect(a.getAttribute('rel')).toBe('noopener');
		expect(row.querySelector(NO_LIVE)).toBeNull();
	});

	it('leaves a zip a plain file to download', () => {
		const { c, row, media } = open('sources.zip');
		expect(media.render).toBe('file');
		expect(c.queryByRole('button', { name: 'Show text' })).toBeNull();
		expect(c.queryByRole('link', { name: 'Open' })).toBeNull();
		expect(row.querySelector('a.c-doc')!.getAttribute('href')).toBe(`${media.url}?download=1`);
	});
});

describe('a document on its message page', () => {
	const open = (name: string) => {
		const { m } = docSampleMessages<{ id: number; chat: { id: number; kind: string }; message_id: number }>().find((d) => d.doc.name === name)!;
		const url = `/messages/group/${m.id}`;
		const media = fixture<MessageProps>(url).props.message.media!;
		const calls = files();
		renderPage(url);
		const box = document.querySelector<HTMLElement>('.a-msgmedia')!;
		return { box, media, calls, c: within(box) };
	};

	it('opens a PNG in the photo viewer, with its download', () => {
		const { c, media } = open('screenshot.png');
		fireEvent.click(c.getByRole('link', { name: 'Photo' }));
		expect(viewer()!.querySelector('img')!.getAttribute('src')).toBe(media.url);
		expect(c.getByRole('link', { name: 'Download' }).getAttribute('href')).toBe(`${media.url}?download=1`);
	});

	it('plays an mp4 document, with its download', () => {
		const { c, media } = open('demo.mp4');
		expect(c.getByRole('link', { name: 'Download' }).getAttribute('href')).toBe(`${media.url}?download=1`);
		fireEvent.click(c.getByRole('link', { name: 'Video' }));
		expect(viewer()!.querySelector('video')).toBeTruthy();
	});

	it('shows a log\'s text on demand and opens a PDF in a new tab', async () => {
		const t = open('build.log');
		expect(fileCalls(t.calls)).toHaveLength(0);
		fireEvent.click(t.c.getByRole('button', { name: 'Show text' }));
		await waitFor(() => expect(t.box.querySelector('pre')?.textContent).toBe(docBytes(995015, 'log')));
		cleanup();
		const p = open('manual.pdf');
		const a = p.c.getByRole('link', { name: 'Open' });
		expect([a.getAttribute('href'), a.getAttribute('target'), a.getAttribute('rel')]).toEqual([p.media.url, '_blank', 'noopener']);
		expect(p.box.querySelector(NO_LIVE)).toBeNull();
	});

	it('keeps a zip a plain file row', () => {
		const { c, box, media } = open('sources.zip');
		expect(c.queryByRole('button', { name: 'Show text' })).toBeNull();
		expect(box.querySelector('a.c-doc')!.getAttribute('href')).toBe(`${media.url}?download=1`);
	});
});

// The files page and a group's Files tab share the preview dialog.
const previewChecks = (open: (name: string) => Promise<{ d: HTMLElement; url: string; calls: ReturnType<typeof fakeApi> }>) => {
	it('draws a PNG document as a picture that opens in the photo viewer', async () => {
		const { d, url } = await open('screenshot.png');
		const zoom = within(d).getByRole('button', { name: 'View bigger' });
		expect(zoom.querySelector('img')!.getAttribute('src')).toBe(url);
		fireEvent.click(zoom);
		expect(viewer()!.querySelector('img')!.getAttribute('src')).toBe(url);
		expect(within(d).getByRole('link', { name: 'Download' }).getAttribute('href')).toBe(`${url}?download=1`);
	});

	it('draws an svg document only through an <img>', async () => {
		const { d, url } = await open('diagram.svg');
		expect(d.querySelector(`img[src="${url}"]`)).toBeTruthy();
		expect(d.querySelector(NO_LIVE)).toBeNull();
	});

	it('plays an mp4 and an mp3 document with the chat\'s players', async () => {
		const v = await open('demo.mp4');
		expect(within(v.d).getByRole('link', { name: 'Video' })).toBeTruthy();
		cleanup();
		const a = await open('talk.mp3');
		expect(a.d.querySelector('audio')!.getAttribute('src')).toBe(a.url);
	});

	it('shows a stored log\'s text when opened, and an HTML page as its source', async () => {
		const t = await open('build.log');
		await waitFor(() => expect(t.d.querySelector('pre')?.textContent).toBe(docBytes(995015, 'log')));
		expect(fileCalls(t.calls)[0]!.headers.get('range')).toBe(`bytes=0-${512 * 1024}`);
		cleanup();
		const h = await open('page.html');
		await waitFor(() => expect(h.d.querySelector('pre')?.textContent).toContain('<script>'));
		expect(h.d.querySelector(NO_LIVE)).toBeNull();
		expect(h.d.querySelector('pre h1, .a-preview-text h1')).toBeNull();
	});

	it('cuts a text over 512 KiB with a note', async () => {
		const { d } = await open('trace-big.log');
		expect(await within(d).findByText(/Showing the first 512 KiB/)).toBeTruthy();
		expect(d.querySelector('pre')!.textContent!.length).toBe(512 * 1024);
	});

	it('opens a PDF in a new tab, and leaves a zip a download', async () => {
		const p = await open('manual.pdf');
		const a = within(p.d).getByRole('link', { name: 'Open' });
		expect([a.getAttribute('href'), a.getAttribute('target'), a.getAttribute('rel')]).toEqual([p.url, '_blank', 'noopener']);
		expect(p.d.querySelector(NO_LIVE)).toBeNull();
		cleanup();
		const z = await open('sources.zip');
		expect(within(z.d).queryByRole('link', { name: 'Open' })).toBeNull();
		expect(within(z.d).queryByRole('button', { name: 'Show text' })).toBeNull();
		expect(z.d.querySelector('pre, img.a-preview-img')).toBeNull();
		expect(fileCalls(z.calls)).toHaveLength(0);
		expect(within(z.d).getByRole('link', { name: 'Download' }).getAttribute('href')).toBe(`${z.url}?download=1`);
	});
};

describe('a stored image document in a list', () => {
	it('is drawn from its own URL in the files grid and list, and in the Files tab, having no thumb', () => {
		const f = fixture<FilesProps>('/files?view=list&q=screenshot.png').props.rows.find((r) => r.name === 'screenshot.png')!;
		expect(f.thumb).toBeNull();
		const img = (sel: string) => document.querySelector<HTMLImageElement>(`${sel} img[src="${f.url}"]`);
		renderPage('/files?q=screenshot.png');
		expect(img('.a-filecard')?.getAttribute('loading')).toBe('lazy');
		cleanup();
		renderPage('/files?view=list&q=screenshot.png');
		expect(img('.a-usercell')?.getAttribute('loading')).toBe('lazy');
		cleanup();
		renderPage(`/groups/${KINDS_GROUP.id}/files?kind=document`);
		expect(img('.a-filecard')?.getAttribute('loading')).toBe('lazy');
	});
});

describe('a document in the files preview', () => {
	previewChecks(async (name) => {
		const list = fixture<FilesProps>(`/files?view=list&q=${name}`).props.rows;
		const f = list.find((r) => r.name === name)!;
		const calls = files();
		renderPage(`/files?view=list&q=${name}&file=${f.id}`);
		const d = await screen.findByRole('dialog', { name });
		return { d, url: f.url!, calls };
	});

	it('offers a text file not stored yet only on a click, never loading it by itself', async () => {
		const list = fixture<FilesProps>('/files?view=list&q=build.log').props;
		const f = { ...list.rows.find((r) => r.name === 'build.log')!, stored: false, fetchable: true };
		const calls = files();
		renderPage(`/files?view=list&file=${f.id}`, 'superadmin', { props: { ...list, rows: [f], preview: f } });
		const d = await screen.findByRole('dialog', { name: 'build.log' });
		expect(d.querySelector('pre')).toBeNull();
		expect(fileCalls(calls)).toHaveLength(0);
		fireEvent.click(within(d).getByRole('button', { name: 'Show text' }));
		await waitFor(() => expect(d.querySelector('pre')?.textContent).toBe(docBytes(995015, 'log')));
	});
});

describe('a document in a group\'s Files tab', () => {
	previewChecks(async (name) => {
		const url = `/groups/${KINDS_GROUP.id}/files?kind=document`;
		const f = fixture<GroupFilesPage>(url).props.rows.find((r) => r.name === name)!;
		const calls = files();
		renderPage(url);
		fireEvent.click(screen.getAllByRole('button').find((b) => b.classList.contains('a-filecard') && b.textContent!.includes(name))!);
		const d = await screen.findByRole('dialog', { name });
		return { d, url: f.url!, calls };
	});
});

describe('the download link', () => {
	it('adds download=1 and keeps the query', () => {
		expect(downloadHref('/files/tok')).toBe('/files/tok?download=1');
		expect(downloadHref('/files/tok?x=1#a')).toBe('/files/tok?x=1&download=1#a');
		expect(downloadHref('/files/tok?download=0')).toBe('/files/tok?download=1');
		expect(downloadHref('https://cdn.example.org/f?a=b')).toBe('https://cdn.example.org/f?a=b&download=1');
	});
});

// The owner's rule: a file's text, an HTML page or an svg never goes
// through innerHTML, an iframe, an object, an embed or srcdoc.
describe('the document renderers', () => {
	it('use none of innerHTML, iframe, object, embed or srcdoc', () => {
		for (const f of ['../ui/docpreview.tsx', './chat-media.tsx', '../ui/filegrid.tsx']) {
			// The code, without its comments (which may name what it avoids).
			const src = readFileSync(new URL(f, import.meta.url), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '').replace(/\{\/\*[\s\S]*?\*\/\}/g, '');
			expect(src, f).not.toMatch(/innerHTML|outerHTML|insertAdjacentHTML|srcdoc|createContextualFragment|DOMParser/i);
			expect(src, f).not.toMatch(/<\s*(iframe|object|embed)\b/i);
			expect(src, f).not.toMatch(/createElement\(\s*['"`](iframe|object|embed|script)/i);
		}
	});
});
