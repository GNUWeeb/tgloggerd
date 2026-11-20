// A document previewed by its extension (web/docs/media.md,
// "Documents"): its download link, a PDF's "Open" link and the text
// view. Go decides how a document renders (MediaRender); this draws it.
//
// Never through innerHTML, an iframe, an object or an embed: a text
// file (an HTML page too) is shown as React text, escaped, and an svg
// only ever goes into an <img>, so nothing in a file runs on the web's
// origin, where it would act with the viewer's session.
import { useEffect, useState } from 'react';
import { Download, ExternalLink, FileText, LoaderCircle } from 'lucide-react';
import type { S } from '../api/schema.ts';

export type Render = S['MediaRender'];

// The text view reads at most this much of a file.
export const TEXT_LIMIT = 512 * 1024;

// The file's URL that always downloads it (?download=1), keeping any
// query it has.
export function downloadHref(url: string): string {
	const u = new URL(url, 'http://local.invalid');
	u.searchParams.set('download', '1');
	return u.origin === 'http://local.invalid' ? u.pathname + u.search + u.hash : u.href;
}

export function DownloadLink({ url, name, className = 'a-btn a-btn-ghost sm' }: { url: string; name?: string; className?: string }) {
	return <a className={className} href={downloadHref(url)} download={name || ''}><Download size={14} />Download</a>;
}

// A PDF opens in the browser's own viewer, in a tab of its own.
export function OpenLink({ url, className = 'a-btn a-btn-ghost sm' }: { url: string; className?: string }) {
	return <a className={className} href={url} target="_blank" rel="noopener"><ExternalLink size={14} />Open</a>;
}

// The first TEXT_LIMIT bytes of the file, as text, and whether there
// is more: one byte past the limit is asked for, and the reading stops
// there whether or not the server honours the range.
export async function readHead(url: string, signal: AbortSignal): Promise<{ text: string; cut: boolean }> {
	const r = await fetch(new Request(new URL(url, location.href), { headers: { Range: `bytes=0-${TEXT_LIMIT}` }, credentials: 'same-origin', signal }));
	if (!r.ok) throw new Error(`HTTP ${r.status}`);
	const chunks: Uint8Array[] = [];
	let n = 0;
	const reader = r.body?.getReader();
	if (reader) {
		while (n <= TEXT_LIMIT) {
			const { done, value } = await reader.read();
			if (done) break;
			chunks.push(value);
			n += value.length;
		}
		if (n > TEXT_LIMIT) void reader.cancel().catch(() => {});
	} else {
		const all = new Uint8Array(await r.arrayBuffer());
		chunks.push(all);
		n = all.length;
	}
	const buf = new Uint8Array(Math.min(n, TEXT_LIMIT));
	let at = 0;
	for (const c of chunks) {
		if (at >= buf.length) break;
		const part = c.subarray(0, buf.length - at);
		buf.set(part, at);
		at += part.length;
	}
	// stream: a character cut at the limit is left out, not mangled.
	return { text: new TextDecoder('utf-8').decode(buf, { stream: n > TEXT_LIMIT }), cut: n > TEXT_LIMIT };
}

// The file's text as a code view, loaded when it is drawn; the caller
// draws it only on demand (opened, or asked for).
export function TextView({ url, name }: { url: string; name?: string }) {
	const [st, setSt] = useState<{ text: string; cut: boolean } | 'wait' | 'failed'>('wait');
	useEffect(() => {
		const ac = new AbortController();
		setSt('wait');
		readHead(url, ac.signal).then((r) => { if (!ac.signal.aborted) setSt(r); }, () => { if (!ac.signal.aborted) setSt('failed'); });
		return () => ac.abort();
	}, [url]);
	if (st === 'wait') return <div className="a-textview a-muted sm" role="status"><LoaderCircle size={14} className="a-spin" />Loading the text...</div>;
	if (st === 'failed') return <div className="a-textview a-muted sm" role="alert">The text could not be loaded.</div>;
	return (
		<div className="a-textview">
			<pre className="a-textview-pre" tabIndex={0} aria-label={name ? `Text of ${name}` : 'File text'}><code>{st.text}</code></pre>
			{st.cut && <p className="a-muted sm a-textview-cut">Showing the first 512 KiB. <a className="a-link" href={downloadHref(url)} download={name || ''}>Download</a> the whole file.</p>}
		</div>
	);
}

// The text view behind a button, so a list or a chat loads no text
// until asked; open draws it at once.
export function TextToggle({ url, name, open: initial = false }: { url: string; name?: string; open?: boolean }) {
	const [open, setOpen] = useState(initial);
	return (
		<>
			{!initial && <button type="button" className="a-btn a-btn-ghost sm" aria-expanded={open} onClick={() => setOpen(!open)}><FileText size={14} />{open ? 'Hide text' : 'Show text'}</button>}
			{open && <TextView url={url} name={name} />}
		</>
	);
}
