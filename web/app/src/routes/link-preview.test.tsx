// @vitest-environment happy-dom
// A link preview's card in the chat and on the message page: its type,
// author and length, its own video, animation or audio played with the
// chat's components, an embedded player's link, and a Telegram message
// link opening the archived message.
import { beforeEach, describe, expect, it } from 'vitest';
import { fixture, setup } from '../test/page.tsx';
import type { ChatProps } from './chat.tsx';
import type { GroupsProps } from './groups.tsx';
import type { ChatMessage } from '../types.ts';

type LP = NonNullable<ChatMessage['link_preview']>;

beforeEach(setup);

// Every link preview in the first window of each sample group's chat.
const samples = (persona: 'superadmin' | 'member' = 'superadmin') => fixture<GroupsProps>('/groups', persona).props.rows
	.flatMap((g) => fixture<ChatProps>(`/groups/${g.id}/chat`, persona).props.messages)
	.map((m) => m.link_preview).filter((x): x is LP => !!x);

describe('the stand-in\'s link previews', () => {
	it('has a sample of each kind the card draws', () => {
		const all = samples();
		const of = (t: string) => all.filter((x) => x.type === t);
		expect(of('article').some((x) => x.media?.w && x.media.h)).toBe(true);
		expect(of('video').some((x) => x.media?.render === 'video' && x.media.poster && x.duration)).toBe(true);
		expect(of('audio').some((x) => x.media?.render === 'audio' && x.author)).toBe(true);
		expect(of('embedded_video_player').some((x) => x.embed?.url)).toBe(true);
		expect(of('message').some((x) => x.target)).toBe(true);
		expect(of('unsupported').length).toBeGreaterThan(0);
		// A preview's own medium never carries a minithumb.
		expect(all.every((x) => !x.media?.minithumb)).toBe(true);
	});
});
