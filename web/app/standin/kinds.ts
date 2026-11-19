// The message kinds of web/docs/message-kinds.md in the stand-in: one
// sample message of each kind in a sample group (and a private call and
// a service message in a private chat), the service messages' actions,
// the interaction counts and a forum group with topics. The samples
// replace the newest messages of those chats but their newest two, so
// the chats' sizes and the message lists stay as they were. Whom a
// detail names follows the viewer as Go does: hidden users and chats
// are null or left out, the *_total fields still count them, and only
// full viewers get a contact's phone.
import { groups, getGroupsOfUser } from '../../mockup/src/data/groups.js';
import { users, getUser } from '../../mockup/src/data/users.js';
import { chatStream, privateMessages } from '../../mockup/src/data/messages.js';
import { RICH_EDIT_TEXT, RICH_PLAIN_TEXT, RICH_TEXT, richDetail, richEditDetail, richPlainDetail } from './rich.ts';

// What the viewer may see, as the request scope says.
export interface View {
	fetch: boolean;
	// Sees users in full (superadmins, admins): a contact's phone.
	full: boolean;
	user: (id: number) => boolean;
	group: (id: number) => boolean;
}
export const ALL: View = { fetch: false, full: true, user: () => true, group: () => true };

type Ref = { id: number; title: string; kind: 'user' | 'group'; photo: null };
const nameOf = (id: number) => {
	const u = getUser(id) as { name?: string; first_name?: string; last_name?: string } | undefined;
	return u ? u.name ?? [u.first_name, u.last_name].filter(Boolean).join(' ') : `#${id}`;
};
const userRef = (id: number): Ref => ({ id, title: nameOf(id), kind: 'user', photo: null });
const groupRef = (id: number): Ref => ({ id, title: groups.find((g) => g.id === id)?.title ?? `#${id}`, kind: 'group', photo: null });
const refOf = (v: View, id: number) => (id > 0 ? (v.user(id) ? userRef(id) : null) : v.group(id) ? groupRef(id) : null);
// A user's profile photo, as the stand-in serves it (fixtures.ts photoOf).
const withPhoto = (r: Ref | null) => (r ? { ...r, photo: `/files/standin-${800000 + (Math.abs(r.id) % 100000)}` } : null);
const refs = (v: View, ids: number[]) => ids.map((id) => refOf(v, id)).filter((r): r is Ref => r !== null);

// The sample chats: kinds in a group, topics in a forum group.
export const KINDS_GROUP = groups[4]!;
export const FORUM_GROUP = groups[2]!;
// An exposed group with polls, for the public Polls tab: two older
// messages of its chat are polls.
export const PUBLIC_POLLS_GROUP = groups[9]!;

// Custom emoji as Go resolves them for a page (telegram_custom_emoji):
// one of each format, one repainted in the text colour, and one
// without a file. Their files are the stand-in's own pictures and
// Lottie circles.
type CustomEmoji = { emoji: string; format: 'webp' | 'tgs' | 'webm' | null; url: string | null; needs_repainting: boolean };
type Effect = { emoji: string; format: 'webp' | 'tgs' | 'webm' | null; url: string | null };
export const CUSTOM_EMOJI: Record<'webp' | 'tgs' | 'webm' | 'repaint' | 'nofile', { id: string; e: CustomEmoji }> = {
	webp: { id: '5368324170671202286', e: { emoji: '🎉', format: 'webp', url: '/files/standin-987001', needs_repainting: false } },
	tgs: { id: '5368324170671202287', e: { emoji: '🔥', format: 'tgs', url: '/files/standin-987002.tgs', needs_repainting: false } },
	webm: { id: '5368324170671202288', e: { emoji: '⭐', format: 'webm', url: '/files/standin-987003', needs_repainting: false } },
	repaint: { id: '5368324170671202289', e: { emoji: '✅', format: 'webp', url: '/files/standin-987004', needs_repainting: true } },
	nofile: { id: '5368324170671202290', e: { emoji: '🙂', format: null, url: null, needs_repainting: false } },
};
// An id the archive does not know yet: Go sends null beside it.
export const UNKNOWN_EMOJI_ID = '5368324170671202299';

type Topic = { id: number; name: string; icon_color: number; icon_custom_emoji_id: string | null; icon_custom_emoji: CustomEmoji | null; closed: boolean; hidden: boolean };
// The forum's topics: one with a custom icon, one whose icon is not
// known yet (drawn with its colour), one with only a colour.
export const TOPICS: Topic[] = [
	{ id: 1, name: 'General', icon_color: 0x6fb9f0, icon_custom_emoji_id: null, icon_custom_emoji: null, closed: false, hidden: false },
	{ id: 12, name: 'Help and questions', icon_color: 0xffd67e, icon_custom_emoji_id: CUSTOM_EMOJI.tgs.id, icon_custom_emoji: CUSTOM_EMOJI.tgs.e, closed: false, hidden: false },
	{ id: 25, name: 'Off-topic', icon_color: 0xcb86db, icon_custom_emoji_id: UNKNOWN_EMOJI_ID, icon_custom_emoji: null, closed: true, hidden: false },
];
export const topicsOf = (chat: { id: number; kind: string }) => (chat.kind === 'group' && chat.id === FORUM_GROUP.id ? TOPICS : []);
export const topicOf = (m: { chat: { id: number; kind: string }; message_id: number }): number | null => {
	const t = topicsOf(m.chat);
	return t.length ? t[m.message_id % t.length]!.id : null;
};

// People and chats the samples name: some a member sees (they are in
// the sample group), some it never does (they are in no group).
// Members of the sample group, as the archive's membership says.
const posters = (): number[] => (users as { id: number }[]).filter((u) => (getGroupsOfUser(u) as { id: number }[]).some((g) => g.id === KINDS_GROUP.id)).map((u) => u.id);
// In no group at all, so no member ever sees them.
const strangers = (): number[] => (users as { id: number }[]).filter((u) => !posters().includes(u.id) && !getGroupsOfUser(u).length).map((u) => u.id);

// A paid media message's items: a photo and a video the logging account
// paid for (stored, so they open), one it did not (only a blurred
// preview is known) and one TDLib could not read.
const PAID_BLUR = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="8" height="6"><rect width="8" height="6" fill="#7a8fa6"/><circle cx="6" cy="2" r="2" fill="#c9d6e3"/></svg>')}`;
const PAID_ITEMS = [
	{ kind: 'photo', media: { stored: true, fetchable: false, url: '/files/standin-985001', thumb: '/files/standin-985001', render: 'image', ext: 'jpg', w: 1280, h: 960, name: '', size: 182000 } },
	{ kind: 'video', media: { stored: true, fetchable: false, url: '/files/standin-985002', thumb: '/files/standin-985102', poster: '/files/standin-985102', render: 'video', ext: 'mp4', w: 1280, h: 720, duration: 21, name: '', size: 2400000 } },
	{ kind: 'preview', media: { stored: false, fetchable: false, url: null, thumb: null, minithumb: PAID_BLUR, w: 1280, h: 960 } },
	{ kind: 'unsupported', media: null },
];

// A game's cover is an animation (it plays); an invoice's a photo.
const GAME_COVER = { stored: true, fetchable: false, url: '/files/standin-986001', thumb: '/files/standin-986101', poster: '/files/standin-986101', render: 'animation', ext: 'mp4', w: 640, h: 360, duration: 3, name: '', size: 310000 };
const INVOICE_PHOTO = { stored: true, fetchable: false, url: '/files/standin-986002', thumb: '/files/standin-986002', render: 'image', ext: 'jpg', w: 800, h: 600, name: '', size: 64000 };

type Entity = { offset: number; length: number; type: string; url?: string; user_id?: number | null; language?: string; custom_emoji_id?: string; custom_emoji?: CustomEmoji | null };
type Edit = { at: string; text: string; content_type: string; entities: Entity[]; detail: unknown };
type Sample = { content_type: string; text?: string; entities?: (v: View) => Entity[]; edits?: (v: View) => Edit[]; edited_at?: string; detail?: (v: View) => unknown; service?: (v: View) => Service; media?: { round?: boolean }; tdlib_type?: string; kinds?: Partial<Kinds> };
// A service message's action as Go sends it. parts spell the text, a
// name pointing at its index in refs.users (LOCAL until openapi.yaml
// has ServiceInfo.parts; src/ui/serviceparts.tsx reads them).
type ServicePart = { text: string; user?: number; chat?: number };
// detail is the action's stored detail, absent when there is none.
type Service = { type: string; text: string; parts?: ServicePart[]; refs: { users: Ref[]; chats: Ref[]; message_id: number | null }; detail?: unknown };
type Kinds = { pinned: boolean; views: number | null; forwards: number | null; replies: number | null; reactions: { emoji?: string; custom_emoji_id?: string; custom_emoji?: CustomEmoji | null; paid?: boolean; count: number }[]; effect_id: string | null; effect: Effect | null };

// Entities over a text: each part is found in the text (after the
// previous one's start) and given its UTF-16 offset and length, as the
// daemon stores them.
type Part = [part: string, type: string, extra?: Partial<Entity>];
export function entitiesIn(text: string, parts: Part[]): Entity[] {
	let from = 0;
	return parts.map(([part, type, extra]) => {
		const offset = text.indexOf(part, from);
		if (offset < 0) throw new Error(`entitiesIn: no "${part}" in the text`);
		from = offset;
		return { offset, length: part.length, type, ...extra };
	});
}
const handleOf = (id: number) => (getUser(id) as { username?: string } | undefined)?.username || '';
// A user's id where the viewer may see them, else null (mention_name)
// or nothing (a mention by @username), as Go answers.
const seen = (v: View, id: number) => (v.user(id) ? id : null);

// A contact's vCard as a phone writes it: CRLF, a folded line, an
// escaped comma, and a second number and an address beyond the one
// number the card shows. Synthetic numbers and example.org only.
const CONTACT_VCARD = 'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Contact\r\nTEL;TYPE=CELL:+6281200000101\r\nTEL;TYPE=WORK:+62 21 0000 0202\r\nEMAIL;TYPE=INTERNET:contact.sample@exam\r\n ple.org\r\nORG:Sample Works\\, Ltd;Field Team\r\nEND:VCARD\r\n';
const REACTIONS = [{ emoji: '👍', count: 12 }, { emoji: '🔥', count: 5 }, { custom_emoji_id: UNKNOWN_EMOJI_ID, custom_emoji: null, count: 3 }, { paid: true, count: 2 }];
// A text in custom emoji of every kind, as entities carry them.
const EMOJI_TEXT = 'Custom emoji: 🎉 a picture, 🔥 an animation, ⭐ a video, ✅ in the text colour, 🙂 without a file, ❓ not known yet.';
const emojiPart = (k: keyof typeof CUSTOM_EMOJI): Part => [CUSTOM_EMOJI[k].e.emoji, 'custom_emoji', { custom_emoji_id: CUSTOM_EMOJI[k].id, custom_emoji: CUSTOM_EMOJI[k].e }];
// A message effect with its small picture (telegram_message_effects).
const EFFECT = { id: '5104841245755180586', effect: { emoji: '🔥', format: 'webp' as const, url: '/files/standin-987010' } };
// Custom emoji in a text and in reactions, and a message effect.
export const EMOJI_SAMPLE: Sample = {
	content_type: 'text', text: EMOJI_TEXT,
	entities: () => entitiesIn(EMOJI_TEXT, [emojiPart('webp'), emojiPart('tgs'), emojiPart('webm'), emojiPart('repaint'), emojiPart('nofile'), ['❓', 'custom_emoji', { custom_emoji_id: UNKNOWN_EMOJI_ID, custom_emoji: null }]]),
	kinds: { reactions: [{ custom_emoji_id: CUSTOM_EMOJI.tgs.id, custom_emoji: CUSTOM_EMOJI.tgs.e, count: 7 }, { custom_emoji_id: CUSTOM_EMOJI.webp.id, custom_emoji: CUSTOM_EMOJI.webp.e, count: 3 }, { emoji: '👍', count: 2 }], effect_id: EFFECT.id, effect: EFFECT.effect },
};
const poll = (question: string, type: 'regular' | 'quiz', opts: string[], extra = {}) => () => {
	const votes = opts.map((_, i) => (opts.length - i) * 7 + (i % 3));
	const total = votes.reduce((a, b) => a + b, 0);
	return {
		kind: 'poll', question, type, anonymous: type === 'regular', multiple: false, closed: false, total_voters: total, close_date: null, explanation: type === 'quiz' ? 'Linus wrote the first version in 1991.' : null,
		options: opts.slice(0, 20).map((text, i) => ({ text, voters: votes[i]!, percent: Math.round((votes[i]! / total) * 100), correct: type === 'quiz' && i === 1 })),
		options_total: opts.length, question_entities: [], explanation_entities: [], ...extra,
	};
};

// A service text that names people, as Go rebuilds it: the ones the
// viewer may see by name, then a count of the others ("added Dewi
// Kusuma and 2 other members to the group"); the stored text, which
// names nobody, when none is visible.
function naming(v: View, type: string, ids: number[], verb: string, noun: string, tail: string, stored: string): Service {
	const shown = refs(v, ids);
	if (!shown.length) return { type, text: stored, parts: [{ text: stored }], refs: { users: [], chats: [], message_id: null } };
	const hidden = ids.length - shown.length;
	const items: ServicePart[] = shown.map((r, i) => ({ text: r.title, user: i }));
	if (hidden) items.push({ text: `${hidden} other ${noun}${hidden > 1 ? 's' : ''}` });
	const parts: ServicePart[] = [{ text: `${verb} ` }];
	const plain = (text: string) => {
		const last = parts[parts.length - 1]!;
		if (last.user === undefined) last.text += text;
		else parts.push({ text });
	};
	items.forEach((p, i) => {
		if (i) plain(i === items.length - 1 ? ' and ' : ', ');
		if (p.user === undefined) plain(p.text);
		else parts.push(p);
	});
	plain(` ${tail}`);
	return { type, text: parts.map((p) => p.text).join(''), parts, refs: { users: shown, chats: [], message_id: null } };
}

// The samples, oldest first; the poll's message id is filled in later
// for the pin's service message.
function samples(): Sample[] {
	const [a, b, c] = posters();
	const [x, y] = strangers();
	// Posters and strangers with a username, for the @mentions.
	const ha = posters().find((id) => handleOf(id))!;
	const hx = strangers().find((id) => handleOf(id))!;
	// A poster without one.
	const plain = posters().find((id) => !handleOf(id)) ?? (() => { throw new Error('kinds: every poster has a username'); })();
	// A story, with its poster's username for a viewer who may see them,
	// as Go sends it.
	const story = (v: View, poster: number, id: number) => ({ kind: 'story', poster: refOf(v, poster), story_id: id, username: v.user(poster) ? handleOf(poster) || null : null });
	const formatted = `Release notes: https://example.org/notes and the changelog. Ping @${handleOf(ha)}, @${handleOf(hx)} or @nobody_here_x, `
		+ `and ${nameOf(b!)} and ${nameOf(y!)}. #release $TGD /status@tgd_bot mail team@example.org or call +62 812 0000 0199. `
		+ 'Bold with italic inside, underlined, struck, a hidden ending, inline code, a 🎉 custom emoji, at 1:05, card 4242 4242 4242 4242, a future kind, a sneaky link, and bold to the end';
	const codeText = '👨‍👩‍👧 Family 𝕏 first, then code:\nfn main() {\n\tprintln!("hi");\n}\nplain block\nA quote from the docs.\n'
		+ 'A long quote that Telegram folds: line one of it,\nline two of it,\nline three of it,\nline four of it.';
	const editedText = 'The meetup is on Friday at the usual place.';
	return [
		// Samples added later go first: each sample takes the message at
		// its place from the end, so the ones after them keep theirs (and
		// their file ids).
		{ content_type: 'expired', detail: () => ({ kind: 'expired' }), tdlib_type: 'messageExpiredVideoNote' },
		// A poll whose poll was never stored, and a location logged before
		// its place was: no detail.
		{ content_type: 'poll' },
		{ content_type: 'location' },
		// A story by a poster without a public username: no link.
		{ content_type: 'story', detail: (v) => story(v, plain, 7) },
		{ content_type: 'text', text: formatted, entities: (v) => entitiesIn(formatted, [
			['https://example.org/notes', 'url'], ['the changelog', 'text_url', { url: 'https://example.org/changelog' }],
			[`@${handleOf(ha)}`, 'mention', v.user(ha) ? { user_id: ha } : {}], [`@${handleOf(hx)}`, 'mention', v.user(hx) ? { user_id: hx } : {}], ['@nobody_here_x', 'mention'],
			[nameOf(b!), 'mention_name', { user_id: seen(v, b!) }], [nameOf(y!), 'mention_name', { user_id: seen(v, y!) }],
			['#release', 'hashtag'], ['$TGD', 'cashtag'], ['/status@tgd_bot', 'bot_command'], ['team@example.org', 'email'], ['+62 812 0000 0199', 'phone_number'],
			['Bold with italic inside', 'bold'], ['italic', 'italic'], ['underlined', 'underline'], ['struck', 'strikethrough'], ['a hidden ending', 'spoiler'],
			['inline code', 'code'], ['🎉', 'custom_emoji', { custom_emoji_id: UNKNOWN_EMOJI_ID, custom_emoji: null }], ['1:05', 'media_timestamp'],
			['4242 4242 4242 4242', 'bank_card_number'], ['a future kind', 'unknown'], ['a sneaky link', 'text_url', { url: 'javascript:alert(1)' }],
		]).concat([{ offset: formatted.length - 'bold to the end'.length, length: 500, type: 'bold' }]) },
		{ content_type: 'text', text: codeText, entities: () => entitiesIn(codeText, [
			['Family', 'bold'], ['𝕏', 'italic'], ['fn main() {\n\tprintln!("hi");\n}', 'pre_code', { language: 'rust' }], ['plain block', 'pre'],
			['A quote from the docs.', 'block_quote'], ['docs', 'italic'], ['A long quote that Telegram folds: line one of it,\nline two of it,\nline three of it,\nline four of it.', 'expandable_block_quote'],
		]) },
		{ content_type: 'text', text: editedText, entities: () => entitiesIn(editedText, [['Friday', 'bold'], ['the usual place', 'text_url', { url: 'https://example.org/place' }]]),
			edited_at: '2026-11-16T08:05:00Z', edits: () => [{ at: '2026-11-16T08:00:00Z', text: 'The meetup is on Thursday, see #meetup.', content_type: 'text', detail: null, entities: entitiesIn('The meetup is on Thursday, see #meetup.', [['Thursday', 'italic'], ['#meetup', 'hashtag']]) }] },
		{ content_type: 'video_note', media: { round: true } },
		{ content_type: 'location', detail: () => ({ kind: 'location', lat: -6.2088, lon: 106.8456, accuracy: null, live: null }), kinds: { reactions: [{ emoji: '📍', count: 2 }], effect_id: '5104841245755180587', effect: null } },
		// A live location with its 17 earlier positions, one a minute.
		{ content_type: 'location', detail: () => ({ kind: 'location', lat: -6.9175, lon: 107.6191, accuracy: 12, live: { period: 3600, heading: 90, proximity_radius: 0, updates: 17 } }),
			edited_at: '2026-11-16T07:17:00Z', edits: () => Array.from({ length: 17 }, (_, i) => ({
				at: `2026-11-16T07:${String(i).padStart(2, '0')}:00Z`, text: '', content_type: 'location', entities: [],
				detail: { kind: 'location', lat: Number((-6.9175 - (17 - i) * 0.0004).toFixed(4)), lon: Number((107.6191 - (17 - i) * 0.0003).toFixed(4)), accuracy: 12, live: { period: 3600, heading: 90, proximity_radius: 0, updates: i } },
			})) },
		{ content_type: 'venue', detail: () => ({ kind: 'venue', lat: -6.1754, lon: 106.8272, title: 'Monas', address: 'Gambir, Central Jakarta', provider: 'foursquare', venue_id: '4b0588f0f964a520a6d822e3', venue_type: 'arts_entertainment/monument' }) },
		{ content_type: 'contact', detail: (v) => ({ kind: 'contact', first_name: nameOf(a!).split(' ')[0] ?? 'Dewi', last_name: nameOf(a!).split(' ').slice(1).join(' '), phone: v.full ? '+62 812 0000 0101' : null, vcard: v.full ? CONTACT_VCARD : null, user: withPhoto(refOf(v, a!)) }) },
		{ content_type: 'dice', detail: () => ({ kind: 'dice', emoji: '🎲', value: 4, stake: false }) },
		{ content_type: 'dice', detail: () => ({ kind: 'dice', emoji: '', value: 6, stake: true }) },
		{ content_type: 'game', text: 'Beat my score!', detail: () => ({ kind: 'game', title: 'Lumberjack', description: 'Chop the tree, dodge the branches.', short_name: 'lumberjack', photo: GAME_COVER }) },
		{ content_type: 'poll', text: 'For the next meetup.', detail: poll('Which editor do you use?', 'regular', ['Vim', 'Neovim', 'Emacs', 'VS Code', 'Helix']), kinds: { pinned: true, views: 1534, forwards: 12, replies: 4, reactions: REACTIONS } },
		{ content_type: 'poll', detail: poll('Who wrote the first Linux kernel?', 'quiz', ['Richard Stallman', 'Linus Torvalds', 'Andrew Tanenbaum'], {
			closed: true, close_date: '2026-11-10T12:00:00Z',
			question_entities: entitiesIn('Who wrote the first Linux kernel?', [['first', 'bold'], ['Linux kernel', 'italic']]),
			explanation_entities: entitiesIn('Linus wrote the first version in 1991.', [['the first version', 'text_url', { url: 'https://example.org/linux-0.01' }], ['1991', 'bold']]),
		}) },
		{ content_type: 'poll', detail: poll('Pick your favourite syscall', 'regular', Array.from({ length: 25 }, (_, i) => `syscall_${i + 1}`)) },
		// A checklist whose first version had two tasks, none done.
		{ content_type: 'checklist', text: 'Release checklist', edited_at: '2026-11-16T08:40:00Z', edits: () => [{
			at: '2026-11-16T08:30:00Z', text: 'Release checklist', content_type: 'checklist', entities: [],
			detail: { kind: 'checklist', tasks_total: 2, others_can_add: true, others_can_mark: true, tasks: [
				{ id: 1, text: 'Tag the release', done: false, done_by: null, done_at: null, entities: [] },
				{ id: 2, text: 'Write the changelog', done: false, done_by: null, done_at: null, entities: [] },
			] },
		}], detail: (v) => ({
			kind: 'checklist', tasks_total: 4, others_can_add: true, others_can_mark: true,
			tasks: [
				{ id: 1, text: 'Tag the release', done: true, done_by: refOf(v, b!), done_at: '2026-11-16T09:00:00Z', entities: [] },
				{ id: 2, text: 'Write the changelog', done: true, done_by: refOf(v, x!), done_at: '2026-11-16T10:30:00Z', entities: [] },
				{ id: 3, text: 'Build the packages with make dist', done: false, done_by: null, done_at: null, entities: entitiesIn('Build the packages with make dist', [['make dist', 'code']]) },
				{ id: 4, text: 'Announce it in #news', done: false, done_by: null, done_at: null, entities: entitiesIn('Announce it in #news', [['#news', 'hashtag']]) },
			],
		}) },
		{ content_type: 'story', detail: (v) => story(v, ha, 42) },
		{ content_type: 'paid_media', text: 'Behind the scenes', detail: () => ({ kind: 'paid_media', star_count: 50, items: PAID_ITEMS }) },
		{ content_type: 'invoice', detail: () => ({ kind: 'invoice', title: 'Conference ticket', description: 'One seat at the 2026 meetup.', currency: 'IDR', total_amount: 15000000, is_test: true, photo: INVOICE_PHOTO }) },
		{ content_type: 'giveaway', detail: (v) => ({ kind: 'giveaway', prize: { premium_months: 3 }, winner_count: 5, winners_selection_date: '2026-12-01T12:00:00Z', only_new_members: false, prize_description: '', chats: refs(v, [KINDS_GROUP.id, groups[9]!.id]), chats_total: 2 }) },
		{ content_type: 'call', detail: (v) => ({ kind: 'call', video: true, group: true, duration: 0, outcome: 'active', participants: refs(v, [a!, b!, x!]), participants_total: 3 }) },
		// A rich message with every block type the web draws.
		{ content_type: 'rich', text: RICH_TEXT, detail: richDetail, edited_at: '2026-11-10T09:30:00Z',
			edits: () => [{ at: '2026-11-10T09:00:00Z', text: RICH_EDIT_TEXT, content_type: 'rich', entities: [], detail: richEditDetail() }] },
		{ content_type: 'widget', detail: () => ({ kind: 'widget' }) },
		{ content_type: 'expired', detail: () => ({ kind: 'expired' }) },
		{ content_type: 'unsupported', detail: () => ({ kind: 'unsupported' }) },
		{ content_type: 'unknown', tdlib_type: 'messageFancyNewThing' },
		// Members added: one a member sees and two it does not.
		{ content_type: 'service', service: (v) => naming(v, 'chat_add_members', [b!, x!, y!], 'added', 'member', 'to the group', 'added 3 members to the group') },
		{ content_type: 'service', service: () => ({ type: 'pin_message', text: 'pinned a message', refs: { users: [], chats: [], message_id: null } }) },
	];
}

// More service messages that name people, newest first: a member added
// whom a member never sees, a member removed, participants invited.
// They take the place of the sample group's older service messages, so
// no other message changes.
function namingSamples(): Sample[] {
	const [a, b] = posters();
	const [x, y] = strangers();
	return [
		{ content_type: 'service', service: (v) => naming(v, 'chat_add_members', [x!], 'added', 'member', 'to the group', 'added a member to the group') },
		{ content_type: 'service', service: (v) => naming(v, 'chat_delete_member', [b!], 'removed', 'member', 'from the group', 'removed a member from the group') },
		{ content_type: 'service', service: (v) => naming(v, 'invite_video_chat_participants', [a!, y!], 'invited', 'participant', 'to the video chat', 'invited participants to the video chat') },
	];
}

// The sample of a message, by chat and message id.
let byKey: Map<string, Sample> | null = null;
function table(): Map<string, Sample> {
	if (byKey) return byKey;
	byKey = new Map();
	const all = chatStream(KINDS_GROUP);
	const list = samples();
	const start = all.length - 2 - list.length;
	let pollId: number | null = null;
	list.forEach((s, i) => {
		const m = all[start + i]!;
		if (s.content_type === 'poll' && s.detail && pollId === null) pollId = m.message_id;
		if (s.service && s.content_type === 'service' && i === list.length - 1) {
			const make = s.service;
			s.service = (v) => ({ ...make(v), refs: { ...make(v).refs, message_id: pollId } });
		}
		byKey!.set(`group${KINDS_GROUP.id}:${m.message_id}`, s);
	});
	const older = all.slice(0, start).filter((m: { content_type: string }) => m.content_type === 'service').reverse();
	// The custom emoji sample takes the newest text message before the
	// samples, so no other sample moves.
	const plain = all.slice(0, start).reverse().find((m: { content_type: string }) => m.content_type === 'text');
	if (plain) byKey.set(`group${KINDS_GROUP.id}:${plain.message_id}`, EMOJI_SAMPLE);
	namingSamples().forEach((s, i) => {
		const m = older[i];
		if (m) byKey!.set(`group${KINDS_GROUP.id}:${m.message_id}`, s);
	});
	// A missed private call, in the first private chat.
	const pm = privateMessages()[0]?.chat;
	if (pm) {
		const ps = chatStream(pm);
		const m = ps[ps.length - 3];
		if (m) byKey.set(`user${pm.id}:${m.message_id}`, { content_type: 'call', detail: () => ({ kind: 'call', video: false, group: false, duration: 0, outcome: 'missed', participants: [], participants_total: 0 }) });
		// Private chats have service rows too.
		const t = ps[ps.length - 4];
		if (t) byKey.set(`user${pm.id}:${t.message_id}`, { content_type: 'service', service: () => ({ type: 'screenshot_taken', text: 'took a screenshot', refs: { users: [], chats: [], message_id: null } }) });
		// A rich message stored before its layout was kept.
		const r = ps[ps.length - 5];
		if (r) byKey.set(`user${pm.id}:${r.message_id}`, { content_type: 'rich', text: RICH_PLAIN_TEXT, detail: richPlainDetail });
	}
	return byKey;
}

// The public polls group's samples, looked up only once its chat is
// asked about: generating a chat stream early would move the mockup's
// file ids, which are handed out in the order chats are generated.
let pubKeys: Map<number, Sample> | null = null;
function publicPolls(): Map<number, Sample> {
	if (pubKeys) return pubKeys;
	pubKeys = new Map();
	const pub = chatStream(PUBLIC_POLLS_GROUP);
	const list: Sample[] = [
		{ content_type: 'poll', detail: poll('Which CTF category next week?', 'regular', ['Pwn', 'Web', 'Crypto', 'Forensics']) },
		{ content_type: 'poll', detail: poll('What does ROP stand for?', 'quiz', ['Read Only Page', 'Return Oriented Programming', 'Remote Object Pointer'], { explanation: 'It chains short code pieces that end in a return.' }) },
	];
	list.forEach((s, i) => {
		const m = pub[pub.length - 40 - 20 * i];
		if (m) pubKeys!.set(m.message_id, s);
	});
	return pubKeys;
}

// The forum group's newest service messages, each an action with its
// stored detail: a new photo (stored, and one not stored), a pin with
// the pinned message's snippet and one logged before pins kept it, a
// video chat's length and start, a poll option, a topic, a community
// (one a member may not see), the group's old title, a background
// logged before it was kept; then, on older text rows, an animated
// photo and the backgrounds below.
// Looked up only once the forum's chat is asked about, as above.
let forumKeys: Map<number, Sample> | null = null;
const NEW_PHOTO = { stored: true, fetchable: false, url: '/files/standin-986010', thumb: '/files/standin-986010', render: 'image', ext: 'jpg', w: 640, h: 640, name: '', size: 48000 };
// An animated group photo: its still and its video, the still as poster.
const ANIM_STILL = { stored: true, fetchable: false, url: '/files/standin-986011', thumb: '/files/standin-986011', render: 'image', ext: 'jpg', w: 640, h: 640, name: '', size: 46000 };
const ANIM_VIDEO = { stored: true, fetchable: false, url: '/files/standin-986012', poster: '/files/standin-986011', render: 'animation', ext: 'mp4', mime: 'video/mp4', w: 640, h: 640, duration: 3, name: '', size: 310000 };
const LOST_PHOTO = { stored: false, fetchable: false, url: null, thumb: null, render: 'image', ext: 'jpg', w: 640, h: 640, name: '', size: 51000 };
// A background logged before the background itself was kept.
const OLD_BG = { kind: 'background', only_for_self: false, type: null, dark: false, image: null, fill: null, intensity: null, is_inverted: false, is_moving: false, is_blurred: false, theme_name: null };
// Backgrounds as Go sends them: a blurred wallpaper (JPEG), a PNG
// pattern over a gradient at 40% and one inverted over a freeform
// gradient, a pattern in Telegram's own format (not drawn), a solid and
// a gradient fill, and an emoji chat theme.
const bg = (o: object) => ({ ...OLD_BG, ...o });
const BG_IMAGE = (id: number, o: object = {}) => ({ stored: true, fetchable: false, url: `/files/standin-${id}`, thumb: `/files/standin-${id}`, render: 'image', ext: 'jpg', mime: 'image/jpeg', w: 1440, h: 2560, name: '', size: 210000, ...o });
const BACKGROUNDS = [
	bg({ type: 'wallpaper', image: BG_IMAGE(986020), is_blurred: true }),
	bg({ type: 'pattern', image: BG_IMAGE(986021, { ext: 'png', mime: 'image/png', size: 64000 }), fill: { type: 'gradient', colors: [0xdbddbb, 0x6ba587], rotation_angle: 45 }, intensity: 40, is_moving: true }),
	bg({ type: 'pattern', image: BG_IMAGE(986022, { ext: 'png', mime: 'image/png', size: 64000 }), fill: { type: 'freeform_gradient', colors: [0xdbddbb, 0x6ba587, 0xd5d88d, 0x88b884], rotation_angle: 0 }, intensity: 55, is_inverted: true, dark: true }),
	bg({ type: 'pattern', image: BG_IMAGE(986023, { render: 'file', thumb: null, ext: 'tgv', mime: 'application/x-tgwallpattern', size: 21000 }), fill: { type: 'freeform_gradient', colors: [0xfec496, 0xdd6cb9, 0x962fbf], rotation_angle: 0 }, intensity: 50 }),
	bg({ type: 'fill', fill: { type: 'solid', colors: [0x2b5278], rotation_angle: 0 }, dark: true }),
	bg({ type: 'fill', fill: { type: 'gradient', colors: [0xffd67e, 0xe17076], rotation_angle: 90 } }),
	bg({ type: 'chat_theme', theme_name: '\u{1F384}' }),
];
function forumServices(): Map<number, Sample> {
	if (forumKeys) return forumKeys;
	forumKeys = new Map();
	const all = chatStream(FORUM_GROUP) as { content_type: string; message_id: number; text: string }[];
	const pinned = all.filter((m) => m.content_type === 'text' && m.text).at(-8);
	const svc = (type: string, text: string, detail: (v: View) => unknown, message_id: number | null = null): Sample =>
		({ content_type: 'service', service: (v) => ({ type, text, refs: { users: [], chats: [], message_id }, detail: detail(v) }) });
	const list: Sample[] = [
		svc('chat_change_photo', 'changed the group photo', () => ({ kind: 'photo', photo: NEW_PHOTO, animation: null })),
		svc('chat_change_photo', 'changed the group photo', () => ({ kind: 'photo', photo: LOST_PHOTO, animation: null })),
		svc('pin_message', 'pinned a message', () => (pinned ? { kind: 'pin', snippet: pinned.text.slice(0, 80) } : null), pinned?.message_id ?? null),
		svc('pin_message', 'pinned a message', () => null),
		svc('video_chat_ended', 'ended the video chat', () => ({ kind: 'video_chat', duration: 3725, start_date: null })),
		svc('video_chat_scheduled', 'scheduled a video chat', () => ({ kind: 'video_chat', duration: null, start_date: '2026-12-01T12:00:00Z' })),
		svc('poll_option_added', 'added an option to a poll', () => ({ kind: 'poll_option', text: 'Rust in the kernel' })),
		svc('forum_topic_created', `created topic "${TOPICS[1]!.name}"`, () => ({ kind: 'topic', topic: { id: TOPICS[1]!.id, name: TOPICS[1]!.name, icon_color: TOPICS[1]!.icon_color, icon_custom_emoji_id: TOPICS[1]!.icon_custom_emoji_id, icon_custom_emoji: TOPICS[1]!.icon_custom_emoji } })),
		svc('chat_added_to_community', 'added the group to a community', (v) => { const c = refOf(v, PUBLIC_POLLS_GROUP.id); return c ? { kind: 'community', community: c } : null; }),
		svc('chat_upgrade_from', 'the supergroup was created from a group', (v) => ({ kind: 'title', title: 'Linux Study Group', basic_group: refOf(v, KINDS_GROUP.id) })),
		svc('chat_set_background', 'changed the chat background', () => ({ ...OLD_BG, only_for_self: true })),
	];
	const rows = all.filter((m) => m.content_type === 'service').slice(-list.length);
	rows.forEach((m, i) => forumKeys!.set(m.message_id, list[i]!));
	// The forum has no more service rows: the other samples take the
	// newest plain text rows before them (not the pinned one).
	const extra: Sample[] = [
		svc('chat_change_photo', 'changed the group photo', () => ({ kind: 'photo', photo: ANIM_STILL, animation: ANIM_VIDEO })),
		...BACKGROUNDS.map((b) => svc('chat_set_background', 'changed the chat background', () => b)),
	];
	const first = rows[0]?.message_id ?? Infinity;
	const plain = all.filter((m) => m.content_type === 'text' && m !== pinned && m.message_id < first).slice(-extra.length);
	plain.forEach((m, i) => forumKeys!.set(m.message_id, extra[i]!));
	return forumKeys;
}

export const sampleOf = (m: { chat: { id: number; kind: string }; message_id: number }) =>
	(m.chat.kind === 'group' && m.chat.id === PUBLIC_POLLS_GROUP.id ? publicPolls().get(m.message_id)
		: m.chat.kind === 'group' && m.chat.id === FORUM_GROUP.id ? forumServices().get(m.message_id)
		: table().get(`${m.chat.kind}${m.chat.id}:${m.message_id}`)) ?? null;

// The MessageKinds of a message for a viewer, and what a sample changes
// of the message itself.
export function kindsOf(m: { chat: { id: number; kind: string }; message_id: number; content_type: string; text?: string }, v: View) {
	const s = sampleOf(m);
	const svc = s?.service?.(v);
	const base = {
		detail: s?.detail ? s.detail(v) : null,
		// Every service message says its action; the stand-in's own ones
		// name nobody.
		service: svc ? { type: svc.type, refs: svc.refs, ...(svc.parts ? { parts: svc.parts } : {}), ...(svc.detail != null ? { detail: svc.detail } : {}) } : null,
		topic_id: topicOf(m), pinned: false, views: null as number | null, forwards: null as number | null, replies: null as number | null,
		reactions: [] as Kinds['reactions'],
		effect_id: null as string | null, effect: null as Effect | null,
		...(s?.kinds ?? {}),
	};
	return { base, sample: s, serviceText: svc?.text };
}
