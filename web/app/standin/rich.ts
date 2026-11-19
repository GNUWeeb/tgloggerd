// A rich message's page blocks in the stand-in (RichDetail): one of
// every block type the web draws, an unknown one, formatting the text
// entities renderer draws (subscript, superscript and marked among
// them), links, a link Go left out (its scheme was not allowed: the
// text stays, the entity or url goes), and media blocks with stand-in
// files. An earlier version of it has its page without media, as
// edits do. And a rich message whose layout was not kept (blocks
// null), which the chat shows as its text. Synthetic text and
// example.org links only.

type Entity = { offset: number; length: number; type: string; url?: string };
type Text = { text: string; entities: Entity[] };

// A run of text with entities over parts of it, found in order.
function rt(text: string, parts: [part: string, type: string, url?: string][] = []): Text {
	let from = 0;
	return {
		text,
		entities: parts.map(([part, type, url]) => {
			const offset = text.indexOf(part, from);
			if (offset < 0) throw new Error(`rich: no "${part}" in the text`);
			from = offset;
			return { offset, length: part.length, type, ...(url ? { url } : {}) };
		}),
	};
}
const caption = (text = '', credit = '') => ({ text: rt(text), credit: rt(credit) });

// Stand-in files, served by /files/standin-<id> as pictures.
const file = (id: number, extra: Record<string, unknown>) => ({ stored: true, fetchable: false, url: `/files/standin-${id}`, thumb: `/files/standin-${id}`, name: '', size: 120000, ...extra });
const picture = (id: number, w = 1280, h = 720) => file(id, { render: 'image', ext: 'jpg', w, h });
const photo = (id: number, text = '', extra = {}) => ({ type: 'photo', media: picture(id), caption: caption(text), spoiler: false, ...extra });
const para = (text: string, parts?: [string, string, string?][]) => ({ type: 'paragraph', text: rt(text, parts) });
const cell = (text: string, extra = {}) => ({ text: rt(text), header: false, colspan: 1, rowspan: 1, align: 'left', valign: 'top', ...extra });

export const RICH_FORMATTED = 'Water is H2O and the area grows as x2. Read the guide or a bad link. This part is highlighted, bold and italic. <b>not bold</b> stays text.';

export function richBlocks() {
	return [
		{ type: 'cover', blocks: [photo(987001, 'The release banner')] },
		{ type: 'title', text: rt('Release 2.0 of the sample tool') },
		{ type: 'subtitle', text: rt('What changed, and how to move over') },
		{ type: 'author_date', text: rt('Sample Team'), date: '2026-11-10T09:00:00Z' },
		{ type: 'kicker', text: rt('Release notes') },
		{ type: 'header', text: rt('Highlights') },
		para(RICH_FORMATTED, [['2', 'subscript'], ['2', 'superscript'], ['the guide', 'text_url', 'https://example.org/guide'],
			['highlighted', 'marked'], ['bold and italic', 'bold'], ['italic', 'italic']]),
		{ type: 'section_heading', text: rt('Upgrading'), size: 2 },
		{ type: 'subheader', text: rt('Before you start') },
		{ type: 'list', items: [
			{ label: '1.', checkbox: false, checked: false, blocks: [para('Back up the config file.')] },
			{ label: '2.', checkbox: false, checked: false, blocks: [para('Stop the service, then:'), { type: 'list', items: [
				{ label: '•', checkbox: false, checked: false, blocks: [para('install the new package')] },
				{ label: '•', checkbox: false, checked: false, blocks: [para('start it again')] },
			] }] },
		] },
		{ type: 'list', items: [
			{ label: '', checkbox: true, checked: true, blocks: [para('Read the changelog')] },
			{ label: '', checkbox: true, checked: false, blocks: [para('Try the new search')] },
		] },
		{ type: 'preformatted', text: rt('make dist\n./sample --version'), language: 'sh' },
		{ type: 'mathematical_expression', expression: 'e^{i\\pi} + 1 = 0' },
		{ type: 'divider' },
		{ type: 'anchor', name: 'faq' },
		{ type: 'block_quote', blocks: [para('The upgrade took five minutes.')], credit: rt('A tester') },
		{ type: 'expandable_block_quote', text: rt('A long quote that folds: line one,\nline two,\nline three,\nline four.'), credit: rt('The forum') },
		{ type: 'pull_quote', text: rt('Faster than ever.'), credit: rt('The benchmark') },
		{ type: 'table', text: rt('Options that changed'), bordered: true, striped: true, rows: [
			[cell('Option', { header: true }), cell('Default', { header: true }), cell('Notes', { header: true })],
			[cell('cache_size'), cell('64 MB', { align: 'right' }), cell('Raised from 16 MB', { rowspan: 2, valign: 'middle' })],
			[cell('Both the workers and the queue', { colspan: 2, align: 'center' })],
		] },
		photo(987002, 'The new settings page', { url: 'https://example.org/gallery' }),
		photo(987003, 'A preview of the next release', { spoiler: true }),
		{ type: 'video', media: file(987004, { render: 'video', ext: 'mp4', w: 1280, h: 720, duration: 42, poster: '/files/standin-987104' }), caption: caption('A short tour', 'Sample Team'), spoiler: false, autoplay: false, loop: false },
		{ type: 'animation', media: file(987005, { render: 'animation', ext: 'mp4', w: 480, h: 270, duration: 3, poster: '/files/standin-987105' }), caption: caption('The new loader'), spoiler: false, autoplay: true },
		{ type: 'audio', media: file(987006, { render: 'audio', ext: 'mp3', duration: 95, name: 'theme.mp3', size: 1520000 }), caption: caption('The release theme') },
		{ type: 'voice_note', media: file(987007, { ext: 'ogg', duration: 12, size: 24000 }), caption: caption('A word from the team') },
		{ type: 'document', media: file(987008, { render: 'file', ext: 'pdf', name: 'release-notes.pdf', size: 320000 }), caption: caption() },
		{ type: 'photo', caption: caption('A photo that was not logged'), spoiler: false },
		{ type: 'collage', blocks: [photo(987009), photo(987010)], caption: caption('Before and after') },
		{ type: 'slideshow', blocks: [photo(987011), photo(987012), photo(987013)], caption: caption('Screenshots') },
		{ type: 'map', lat: -6.2088, lon: 106.8456, zoom: 15, width: 600, height: 300, caption: caption('Where the meetup is') },
		{ type: 'embedded', url: 'https://example.org/embed/demo', media: picture(987015, 640, 360), width: 640, height: 360, caption: caption('The demo video') },
		{ type: 'embedded_post', url: 'https://example.org/blog/release', author: 'Sample Blog', date: '2026-11-09T12:00:00Z', blocks: [para('We are shipping 2.0 tomorrow.')], caption: caption() },
		{ type: 'chat_link', title: 'Sample Tool Chat', username: 'sample_tool_chat' },
		{ type: 'details', text: rt('Frequently asked'), open: false, blocks: [para('Yes, the old config still loads.')] },
		{ type: 'details', text: rt('What is new'), open: true, blocks: [para('Search, tables and a faster cache.')] },
		{ type: 'related_articles', text: rt('Read more'), articles: [
			{ url: 'https://example.org/blog/1.9', title: 'Release 1.9', description: 'The last of the old line.', author: 'Sample Team', date: '2026-08-01T09:00:00Z', photo: picture(987014, 400, 300) },
			{ url: '', title: 'An article whose link was not sent', description: '', author: '', date: null },
		] },
		{ type: 'button_row', align: 'center', buttons: [{ text: rt('Download'), url: 'https://example.org/download' }, { text: rt('Vote') }] },
		{ type: 'thinking', text: rt('Checking the numbers once more.') },
		{ type: 'footer', text: rt('Sample Team, 2026') },
		{ type: 'unknown', name: 'future_block' },
	];
}

// The message text Go sends beside the blocks: the plain text of every
// block (the chat draws the blocks instead).
export const RICH_TEXT = 'Release 2.0 of the sample tool\nWhat changed, and how to move over\n\nHighlights\n' + RICH_FORMATTED;
// The version before the last edit: fewer blocks, and no media.
export const RICH_EDIT_TEXT = 'Release 2.0 of the sample tool\nDraft notes';
export const richEditDetail = () => ({ kind: 'rich', is_rtl: false, is_full: true, blocks: [
	{ type: 'title', text: rt('Release 2.0 of the sample tool') },
	para('Draft notes', [['Draft', 'italic']]),
	{ type: 'photo', caption: caption('The release banner'), spoiler: false },
] });
export const richDetail = () => ({ kind: 'rich', blocks: richBlocks(), is_rtl: false, is_full: true });

// A rich message stored before its layout was kept.
export const RICH_PLAIN_TEXT = 'A rich message whose layout was not kept: the plain text of its blocks.';
export const richPlainDetail = () => ({ kind: 'rich', blocks: null, is_rtl: false, is_full: true });
