// A paid media message's items, drawn as an album is: a paid-for photo
// opens the viewer and a video plays, with the chat's own media boxes.
// An item the logging account did not pay for has only its size and
// blurred preview, so it stays locked; an item TDLib could not read
// says so. The chat and the message page share it.
import type { ReactNode } from 'react';
import { CircleHelp, Image, Lock, Video } from 'lucide-react';
import { Picture, VideoBox } from './chat-media.tsx';
import { bytes } from '../format.ts';
import type { S } from '../api/schema.ts';

type Item = S['PaidMediaDetail']['items'][number];

// A tile that does not open: its icon, what it is, and the blurred
// preview behind it when one is known.
function Tile({ pic, title, children }: { pic?: string | null; title: string; children: ReactNode }) {
	return (
		<div className={`c-media c-missing k-paidtile ${pic ? 'c-blurred' : ''}`} style={pic ? { backgroundImage: `url("${pic}")` } : {}} title={title}>
			{children}
		</div>
	);
}

function PaidItem({ it, single, onClick }: { it: Item; single: boolean; onClick: boolean }) {
	const m = it.media;
	switch (it.kind) {
		case 'photo':
			return m ? <Picture m={m} single={single} onClick={onClick} /> : <Tile title="Photo, not stored"><Image size={20} /><span>Not stored</span></Tile>;
		case 'video':
			return m ? <VideoBox m={m} single={single} onClick={onClick} /> : <Tile title="Video, not stored"><Video size={20} /><span>Not stored</span></Tile>;
		case 'preview':
			return (
				<Tile pic={m?.minithumb ?? m?.thumb} title="Not paid for: only a blurred preview is known">
					<Lock size={20} /><span>Not paid for</span>
					{m?.size ? <span className="c-sub">{bytes(m.size)}</span> : null}
				</Tile>
			);
		default:
			return <Tile title="Unsupported item: TDLib could not read it"><CircleHelp size={20} /><span>Unsupported item</span></Tile>;
	}
}

export function PaidMedia({ items, onClick }: { items: Item[]; onClick: boolean }) {
	if (!items.length) return null;
	if (items.length === 1) return <PaidItem it={items[0]!} single onClick={onClick} />;
	return (
		<div className={`c-album n${Math.min(4, items.length)}`} aria-label={`paid media . ${items.length} items`}>
			{items.map((it, i) => <PaidItem key={i} it={it} single={false} onClick={onClick} />)}
		</div>
	);
}
