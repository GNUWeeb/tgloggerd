// A message's link preview, shared by the chat and the message page.
import { ExternalLink } from 'lucide-react';
import type { ChatMessage } from '../types.ts';

// A link preview: its site, title and description, and its picture,
// linking to the page.
export function LinkPreview({ lp }: { lp: NonNullable<ChatMessage['link_preview']> }) {
	const inner = (
		<>
			<b className="c-lp-site">{lp.site}</b><b>{lp.title}</b><span>{lp.desc}</span>
			{lp.media?.thumb && <img className="c-lp-img" src={lp.media.thumb} alt="" loading="lazy" style={lp.media.w && lp.media.h ? { aspectRatio: `${lp.media.w} / ${lp.media.h}` } : undefined} />}
		</>
	);
	return lp.url
		? <a className="c-lp" href={lp.url} target="_blank" rel="noopener noreferrer nofollow">{inner}<span className="c-lp-url"><ExternalLink size={11} />{lp.url.replace(/^https?:\/\//, '')}</span></a>
		: <div className="c-lp">{inner}</div>;
}
