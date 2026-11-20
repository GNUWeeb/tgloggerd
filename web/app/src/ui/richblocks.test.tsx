// @vitest-environment happy-dom
// The rich message block renderer: every block type of the stand-in's
// sample draws, nothing goes through innerHTML, unsafe links stay text,
// the chat's collapse, right-to-left pages and edit versions.
import { readFileSync } from 'node:fs';
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { NavProvider } from '../nav.tsx';
import { CHAT_BLOCKS, RichBlocks } from './richblocks.tsx';
import { fixture } from '../test/page.tsx';
import { richBlocks, richDetail, richEditDetail } from '../../standin/rich.ts';
import type { S } from '../api/schema.ts';

afterEach(cleanup);

type Detail = S['RichDetail'] & { blocks: S['RichBlock'][] };
const page = fixture('/');
const v = page.viewer;
const rt = (text: string, entities: S['TextEntity'][] = []) => ({ text, entities });

function show(d: Detail, opts: { collapse?: boolean; edit?: boolean } = {}) {
	const { container } = render(<NavProvider initial={page}><RichBlocks d={d} v={v} {...opts} /></NavProvider>);
	return container.querySelector('.rb-page') as HTMLElement;
}
const full = () => show(richDetail() as Detail);
const blocks = (list: unknown[], extra: Partial<Detail> = {}) => show({ kind: 'rich', blocks: list as S['RichBlock'][], is_rtl: false, is_full: true, ...extra });

describe('RichBlocks', () => {
	it('draws a document block by its render: a PDF to open, a text to show, each with its download', () => {
		const doc = (name: string, render: 'pdf' | 'text', url: string) => ({ type: 'document', media: { stored: true, url, thumb: null, render, ext: name.split('.').pop(), name, size: 2048 }, caption: { text: rt(''), credit: rt('') } });
		const el = blocks([doc('guide.pdf', 'pdf', '/files/rb-pdf'), doc('notes.txt', 'text', '/files/rb-txt')]);
		const open = within(el).getByRole('link', { name: 'Open' });
		expect([open.getAttribute('href'), open.getAttribute('target'), open.getAttribute('rel')]).toEqual(['/files/rb-pdf', '_blank', 'noopener']);
		expect(within(el).getByRole('button', { name: 'Show text' })).toBeTruthy();
		expect([...el.querySelectorAll('a.c-doc')].map((a) => a.getAttribute('href'))).toEqual(['/files/rb-pdf?download=1', '/files/rb-txt?download=1']);
	});

	it('draws the text blocks as headings, paragraphs, code and a rule', () => {
		const el = full();
		expect(el.querySelector('h2.rb-title')!.textContent).toBe('Release 2.0 of the sample tool');
		expect(el.querySelector('h3.rb-subtitle')!.textContent).toBe('What changed, and how to move over');
		expect(el.querySelector('h3.rb-header')!.textContent).toBe('Highlights');
		expect(el.querySelector('h4.rb-subheader')!.textContent).toBe('Before you start');
		expect(el.querySelector('h4.rb-heading')!.textContent).toBe('Upgrading');
		expect(el.querySelector('.rb-kicker')!.textContent).toBe('Release notes');
		expect(el.querySelector('.rb-byline')!.textContent).toMatch(/^Sample Team . /);
		expect(el.querySelector('.rb-footer')!.textContent).toBe('Sample Team, 2026');
		expect(el.querySelector('.rb-thinking')!.textContent).toBe('Checking the numbers once more.');
		expect(el.querySelector('pre.rt-pre code.language-sh')!.textContent).toBe('make dist\n./sample --version');
		expect(el.querySelector('.rb-math code')!.textContent).toBe('e^{i\\pi} + 1 = 0');
		expect(el.querySelector('hr.rb-hr')).toBeTruthy();
		const p = el.querySelector('p.rb-p')!;
		expect(p.querySelector('sub')!.textContent).toBe('2');
		expect(p.querySelector('sup')!.textContent).toBe('2');
		expect(p.querySelector('mark')!.textContent).toBe('highlighted');
		expect(p.querySelector('strong em')!.textContent).toBe('italic');
		const a = within(p as HTMLElement).getByRole('link', { name: 'the guide' });
		expect(a.getAttribute('href')).toBe('https://example.org/guide');
		expect(a.getAttribute('rel')).toBe('noopener noreferrer nofollow');
	});

	it('keeps markup in the text as text', () => {
		full();
		const p = screen.getByText(/stays text\.$/);
		expect(p.querySelector('b')).toBeNull();
		expect(p.textContent).toContain('<b>not bold</b> stays text.');
	});

	it('never draws through innerHTML', () => {
		const src = readFileSync(`${process.cwd()}/src/ui/richblocks.tsx`, 'utf8');
		expect(src).not.toMatch(/dangerouslySetInnerHTML|innerHTML/);
	});

	it('draws lists numbered, bulleted, nested and with checkboxes', () => {
		const el = full();
		const ol = el.querySelector('ol.rb-list')!;
		expect([...ol.children].map((li) => li.querySelector('.rb-label')!.textContent)).toEqual(['1.', '2.']);
		expect(ol.querySelector('ul.rb-list')!.textContent).toContain('install the new package');
		const checks = [...el.querySelectorAll('ul.rb-list')].find((u) => u.querySelector('[aria-label="checked"]'))!;
		expect(checks.querySelector('li.done')!.textContent).toBe('Read the changelog');
		expect(checks.querySelector('[aria-label="not checked"]')).toBeTruthy();
	});

	it('draws quotes with their credit, and opens a folded one', () => {
		const el = full();
		expect(el.querySelector('.rb-quote')!.textContent).toBe('The upgrade took five minutes.A tester');
		expect(el.querySelector('.rb-pull')!.textContent).toBe('Faster than ever.The benchmark');
		const folded = el.querySelector('.rt-expandable')!;
		const more = within(folded as HTMLElement).getByRole('button', { name: 'Show more' });
		fireEvent.click(more);
		expect(folded.classList.contains('open')).toBe(true);
		expect(more.getAttribute('aria-expanded')).toBe('true');
	});

	it('draws a table with header cells, spans and alignment', () => {
		const t = full().querySelector('table.rb-table.bordered.striped')!;
		expect(t.querySelector('caption')!.textContent).toBe('Options that changed');
		expect([...t.querySelectorAll('th')].map((x) => x.textContent)).toEqual(['Option', 'Default', 'Notes']);
		const span = [...t.querySelectorAll('td')].find((x) => x.textContent === 'Raised from 16 MB') as HTMLTableCellElement;
		expect(span.getAttribute('rowspan')).toBe('2');
		const wide = [...t.querySelectorAll('td')].find((x) => x.textContent === 'Both the workers and the queue') as HTMLTableCellElement;
		expect(wide.getAttribute('colspan')).toBe('2');
		expect(wide.style.textAlign).toBe('center');
	});

	it('draws the media blocks with the chat\'s components', () => {
		const el = full();
		expect(el.querySelectorAll('a[aria-label="Photo"]').length).toBeGreaterThanOrEqual(6);
		expect(el.querySelector('a[aria-label="Video"]')).toBeTruthy();
		expect(el.querySelector('a[aria-label="Animation"]')).toBeTruthy();
		expect(el.querySelector('.c-voice')).toBeTruthy();
		expect(el.querySelector('audio.c-audio')).toBeTruthy();
		expect(screen.getByText('release-notes.pdf')).toBeTruthy();
		expect(screen.getByText('A photo that was not logged', { selector: '.rb-nomedia' })).toBeTruthy();
		expect(el.querySelector('.rb-collage')!.querySelectorAll('a[aria-label="Photo"]')).toHaveLength(2);
		expect(el.querySelector('.rb-slideshow')!.querySelectorAll('a[aria-label="Photo"]')).toHaveLength(3);
		expect(screen.getByText('A short tour')).toBeTruthy();
		// A photo's own link, under it.
		expect(screen.getByRole('link', { name: 'https://example.org/gallery' }).getAttribute('rel')).toContain('noopener');
	});

	it('hides a spoiler photo until asked', () => {
		full();
		const fig = screen.getByText('A preview of the next release').closest('figure')!;
		expect(fig.querySelector('img')).toBeNull();
		fireEvent.click(within(fig).getByRole('button', { name: 'Spoiler, show the media' }));
		expect(fig.querySelector('a[aria-label="Photo"]')).toBeTruthy();
	});

	it('draws maps, embeds, posts, chat links, related articles and buttons as links', () => {
		full();
		const map = screen.getByRole('link', { name: /^Map at -6\.20880, 106\.84560$/ });
		expect(map.getAttribute('href')).toBe('https://www.openstreetmap.org/?mlat=-6.2088&mlon=106.8456#map=15/-6.2088/106.8456');
		const embed = screen.getByRole('link', { name: 'Embedded content: https://example.org/embed/demo' });
		expect(embed.closest('figure')!.querySelector('a[aria-label="Photo"]')).toBeTruthy();
		expect(document.querySelector('iframe, script')).toBeNull();
		expect(screen.getByRole('link', { name: 'Sample Blog' }).getAttribute('href')).toBe('https://example.org/blog/release');
		expect(screen.getByText('We are shipping 2.0 tomorrow.')).toBeTruthy();
		expect(screen.getByRole('link', { name: 'Sample Tool Chat' }).getAttribute('href')).toBe('https://t.me/sample_tool_chat');
		expect(screen.getByRole('link', { name: 'Release 1.9' }).getAttribute('href')).toBe('https://example.org/blog/1.9');
		// An article whose link was left out is its title only.
		expect(screen.getByText('An article whose link was not sent').closest('a')).toBeNull();
		expect(screen.getByRole('link', { name: 'Download' }).getAttribute('href')).toBe('https://example.org/download');
		expect(screen.getByText('Vote').closest('a')).toBeNull();
		expect(document.querySelector('.rb-buttons.center')).toBeTruthy();
	});

	it('draws details closed or open as the page says', () => {
		const el = full();
		const [closed, open] = [...el.querySelectorAll('details.rb-details')] as HTMLDetailsElement[];
		expect(closed!.querySelector('summary')!.textContent).toBe('Frequently asked');
		expect(closed!.open).toBe(false);
		expect(open!.open).toBe(true);
	});

	it('says which block it does not show', () => {
		full();
		expect(screen.getByText('A block this view does not show (future_block)')).toBeTruthy();
		cleanup();
		blocks([{ type: 'brand_new' }, { type: 'unknown' }]);
		expect(screen.getByText('A block this view does not show (brand_new)')).toBeTruthy();
		expect(screen.getByText('A block this view does not show (unknown)')).toBeTruthy();
	});

	it('leaves links of an unsafe scheme as text', () => {
		const t = 'a b c d';
		const el = blocks([
			{ type: 'paragraph', text: rt(t, [{ offset: 0, length: 1, type: 'text_url', url: 'javascript:alert(1)' }]) },
			{ type: 'photo', media: (richBlocks()[0] as { blocks: { media: unknown }[] }).blocks[0]!.media, caption: { text: rt(''), credit: rt('') }, url: 'javascript:alert(2)' },
			{ type: 'embedded', url: 'data:text/html,x', caption: { text: rt(''), credit: rt('') } },
			{ type: 'embedded_post', url: 'vbscript:x', author: 'Someone', blocks: [] },
			{ type: 'related_articles', articles: [{ url: 'javascript:alert(3)', title: 'Bad', description: '', author: '', date: null }] },
			{ type: 'button_row', buttons: [{ text: rt('Press'), url: 'javascript:alert(4)' }] },
		]);
		for (const a of el.querySelectorAll('a')) expect(a.getAttribute('href')).not.toMatch(/^(javascript|data|vbscript):/i);
		expect(screen.getByText('Bad').closest('a')).toBeNull();
		expect(screen.getByText('Press').closest('a')).toBeNull();
		expect(screen.getByText('Someone').closest('a')).toBeNull();
	});

	it('shows the first blocks in the chat, and all of them on Show all', () => {
		const d = richDetail() as Detail;
		const el = show(d, { collapse: true });
		expect(el.querySelector('h2.rb-title')).toBeTruthy();
		expect(el.querySelector('table')).toBeNull();
		const more = d.blocks.length - CHAT_BLOCKS;
		fireEvent.click(screen.getByRole('button', { name: `Show all (${more} more blocks)` }));
		expect(el.querySelector('table')).toBeTruthy();
		expect(screen.queryByRole('button', { name: /Show all/ })).toBeNull();
	});

	it('does not collapse a short page', () => {
		blocks([{ type: 'paragraph', text: rt('one') }], {});
		expect(screen.queryByRole('button', { name: /Show all/ })).toBeNull();
	});

	it('reads right to left when the page does, and says when only its start was logged', () => {
		const el = blocks([{ type: 'paragraph', text: rt('right to left') }], { is_rtl: true, is_full: false });
		expect(el.getAttribute('dir')).toBe('rtl');
		expect(screen.getByText('Only the start of this message was logged.')).toBeTruthy();
		cleanup();
		expect(full().getAttribute('dir')).toBeNull();
	});

	it('draws an edit\'s version without its media', () => {
		const el = show(richEditDetail() as Detail, { edit: true });
		expect(el.querySelector('h2.rb-title')).toBeTruthy();
		expect(screen.getByText('The release banner')).toBeTruthy();
		expect(el.querySelector('.rb-nomedia')).toBeNull();
	});
});
