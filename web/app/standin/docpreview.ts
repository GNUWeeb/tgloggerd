// Documents previewed by their extension (docs/todo/web-doc-preview.md,
// web/docs/media.md "Documents"), as Go renders and serves them: the
// extension table, the sample documents of the kinds group, and the
// bytes and headers /files/<token> answers for them.

type Render = 'image' | 'video' | 'audio' | 'pdf' | 'text' | 'file';

const BY_EXT: Record<string, Render> = {};
const put = (r: Render, exts: string) => { for (const e of exts.split(' ')) BY_EXT[e] = r; };
put('image', 'jpg jpeg png gif webp avif bmp ico apng svg');
put('video', 'mp4 m4v webm mov ogv');
put('audio', 'mp3 m4a aac ogg oga opus wav flac');
put('pdf', 'pdf');
put('text', 'txt log md csv tsv json xml yaml yml toml ini conf cfg sh py c h cpp hpp go rs js ts java kt sql diff patch html htm');

// How a document renders: by its lower-case extension, else a file.
export const docRender = (ext: string | undefined): Render => BY_EXT[(ext ?? '').toLowerCase()] ?? 'file';

// How a file of a list renders (FileRow.render, GroupFileRow.render):
// by its type, a sticker by its extension, a document by the table.
export function fileRender(type: string, ext: string): string {
	switch (type) {
		case 'photo': case 'thumbnail': return 'image';
		case 'video': case 'video_note': return 'video';
		case 'animation': return 'animation';
		case 'audio': case 'voice': return 'audio';
		case 'sticker': return ext === 'tgs' ? 'lottie' : ext === 'webm' ? 'animation' : 'image';
		case 'document': return docRender(ext);
		default: return 'file';
	}
}

// A previewed document's URL says its extension (as a .tgs sticker's
// does), so the stand-in's /files knows what to answer with.
export const docSuffix = (ext: string | undefined) => {
	const e = (ext ?? '').toLowerCase();
	return docRender(e) === 'text' || docRender(e) === 'pdf' || e === 'svg' ? e : '';
};

export type DocSample = { fileId: number; name: string; ext: string; size: number; w?: number; h?: number; duration?: number; caption: string };

// Over the text view's limit, so it is cut.
export const BIG_TEXT_SIZE = 600 * 1024;

// The sample documents, one message each in the kinds group: a picture
// of each kind, a video and a song, two text files and an HTML page
// that tries to run a script, a PDF, an archive (stays a file) and a
// text file over 512 KiB.
export const DOC_SAMPLES: DocSample[] = [
	{ fileId: 995011, name: 'screenshot.png', ext: 'png', size: 182_000, w: 1280, h: 720, caption: 'A screenshot sent as a file' },
	{ fileId: 995012, name: 'diagram.svg', ext: 'svg', size: 4_200, w: 400, h: 300, caption: 'A diagram' },
	{ fileId: 995013, name: 'demo.mp4', ext: 'mp4', size: 2_400_000, w: 1280, h: 720, duration: 14, caption: '' },
	{ fileId: 995014, name: 'talk.mp3', ext: 'mp3', size: 1_900_000, duration: 95, caption: '' },
	{ fileId: 995015, name: 'build.log', ext: 'log', size: 1_024, caption: 'The build log' },
	{ fileId: 995016, name: 'page.html', ext: 'html', size: 210, caption: '' },
	{ fileId: 995017, name: 'manual.pdf', ext: 'pdf', size: 320, caption: '' },
	{ fileId: 995018, name: 'sources.zip', ext: 'zip', size: 88_000, caption: '' },
	{ fileId: 995019, name: 'trace-big.log', ext: 'log', size: BIG_TEXT_SIZE, caption: 'A long trace' },
];

export const BUILD_LOG = 'make[1]: Entering directory \'/src\'\ncc -O2 -c main.c -o main.o\ncc -O2 -c util.c -o util.o\ncc main.o util.o -o app\nmake[1]: Leaving directory \'/src\'\n';
// An HTML page that would run a script if it were ever rendered.
export const HTML_SAMPLE = '<!doctype html><html><body><h1>Status</h1><script>document.title = "doc ran"</script><img src="x" onerror="document.title = \'doc ran\'"></body></html>\n';
const PDF_SAMPLE = '%PDF-1.4\n1 0 obj <</Type /Catalog /Pages 2 0 R>> endobj\n2 0 obj <</Type /Pages /Kids [3 0 R] /Count 1>> endobj\n3 0 obj <</Type /Page /Parent 2 0 R /MediaBox [0 0 200 100]>> endobj\ntrailer <</Root 1 0 R>>\n%%EOF\n';

// A big text: numbered lines up to its size.
function bigText(size: number): string {
	const out: string[] = [];
	let n = 0;
	for (let i = 1; n < size; i++) {
		const line = `${String(i).padStart(6, '0')} trace: step ${i} done\n`;
		out.push(line);
		n += line.length;
	}
	return out.join('').slice(0, size);
}

// What /files/standin-<id>.<ext> answers: the sample's bytes, or for
// another document text of its kind.
export function docBytes(id: number, ext: string): string {
	const s = DOC_SAMPLES.find((d) => d.fileId === id);
	if (ext === 'svg') return '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="300" viewBox="0 0 400 300"><rect width="400" height="300" fill="#6a9fd4"/><circle cx="200" cy="150" r="80" fill="#f4d36b"/><script>document.title = "svg ran"</script></svg>';
	if (ext === 'pdf') return PDF_SAMPLE;
	if (ext === 'html' || ext === 'htm') return HTML_SAMPLE;
	if (s && s.size > 64 * 1024) return bigText(s.size);
	if (s) return BUILD_LOG;
	return `Sample ${ext} file ${id}\nline two\nline three\n`;
}

// The headers Go sends a previewed document with: inline with its
// name, nosniff, the sandbox for images and text, text as plain UTF-8
// text, a PDF as itself without the sandbox (it would stop the
// browser's viewer); ?download=1 makes it an attachment.
export const DOC_CSP = "sandbox; default-src 'none'; img-src 'self'; style-src 'unsafe-inline'";
export function docHeaders(id: number, ext: string, download: boolean): Record<string, string> {
	const r = docRender(ext);
	const name = DOC_SAMPLES.find((d) => d.fileId === id)?.name ?? `document_${id}.${ext}`;
	const type = r === 'text' ? 'text/plain; charset=utf-8' : r === 'pdf' ? 'application/pdf' : ext === 'svg' ? 'image/svg+xml' : 'application/octet-stream';
	return {
		'content-type': type, 'cache-control': 'no-store', 'x-content-type-options': 'nosniff',
		'content-disposition': `${download ? 'attachment' : 'inline'}; filename="${name}"`,
		...(!download && r !== 'pdf' ? { 'content-security-policy': DOC_CSP } : {}),
	};
}
