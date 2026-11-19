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
const refs = (v: View, ids: number[]) => ids.map((id) => refOf(v, id)).filter((r): r is Ref => r !== null);

// The sample chats: kinds in a group, topics in a forum group.
export const KINDS_GROUP = groups[4]!;
export const FORUM_GROUP = groups[2]!;
// An exposed group with polls, for the public Polls tab: two older
// messages of its chat are polls.
export const PUBLIC_POLLS_GROUP = groups[9]!;

export const TOPICS = [
	{ id: 1, name: 'General', icon_color: 0x6fb9f0, closed: false, hidden: false },
	{ id: 12, name: 'Help and questions', icon_color: 0xffd67e, closed: false, hidden: false },
	{ id: 25, name: 'Off-topic', icon_color: 0xcb86db, closed: true, hidden: false },
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

type Entity = { offset: number; length: number; type: string; url?: string; user_id?: number | null; language?: string; custom_emoji_id?: string };
type Edit = { at: string; text: string; content_type: string; entities: Entity[] };
type Sample = { content_type: string; text?: string; entities?: (v: View) => Entity[]; edits?: (v: View) => Edit[]; edited_at?: string; detail?: (v: View) => unknown; service?: (v: View) => { type: string; text: string; refs: { users: Ref[]; chats: Ref[]; message_id: number | null } }; media?: { round?: boolean }; tdlib_type?: string; kinds?: Partial<Kinds> };
type Kinds = { pinned: boolean; views: number | null; forwards: number | null; replies: number | null; reactions: { emoji?: string; custom_emoji_id?: string; paid?: boolean; count: number }[] };

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

const REACTIONS = [{ emoji: '👍', count: 12 }, { emoji: '🔥', count: 5 }, { custom_emoji_id: '5368324170671202286', count: 3 }, { paid: true, count: 2 }];
const poll = (question: string, type: 'regular' | 'quiz', opts: string[], extra = {}) => () => {
	const votes = opts.map((_, i) => (opts.length - i) * 7 + (i % 3));
	const total = votes.reduce((a, b) => a + b, 0);
	return {
		kind: 'poll', question, type, anonymous: type === 'regular', multiple: false, closed: false, total_voters: total, close_date: null, explanation: type === 'quiz' ? 'Linus wrote the first version in 1991.' : null,
		options: opts.slice(0, 20).map((text, i) => ({ text, voters: votes[i]!, percent: Math.round((votes[i]! / total) * 100), correct: type === 'quiz' && i === 1 })),
		options_total: opts.length, question_entities: [], explanation_entities: [], ...extra,
	};
};

// The samples, oldest first; the poll's message id is filled in later
// for the pin's service message.
function samples(): Sample[] {
	const [a, b, c] = posters();
	const [x, y] = strangers();
	// Posters and strangers with a username, for the @mentions.
	const ha = posters().find((id) => handleOf(id))!;
	const hx = strangers().find((id) => handleOf(id))!;
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
		{ content_type: 'text', text: formatted, entities: (v) => entitiesIn(formatted, [
			['https://example.org/notes', 'url'], ['the changelog', 'text_url', { url: 'https://example.org/changelog' }],
			[`@${handleOf(ha)}`, 'mention', v.user(ha) ? { user_id: ha } : {}], [`@${handleOf(hx)}`, 'mention', v.user(hx) ? { user_id: hx } : {}], ['@nobody_here_x', 'mention'],
			[nameOf(b!), 'mention_name', { user_id: seen(v, b!) }], [nameOf(y!), 'mention_name', { user_id: seen(v, y!) }],
			['#release', 'hashtag'], ['$TGD', 'cashtag'], ['/status@tgd_bot', 'bot_command'], ['team@example.org', 'email'], ['+62 812 0000 0199', 'phone_number'],
			['Bold with italic inside', 'bold'], ['italic', 'italic'], ['underlined', 'underline'], ['struck', 'strikethrough'], ['a hidden ending', 'spoiler'],
			['inline code', 'code'], ['🎉', 'custom_emoji', { custom_emoji_id: '5368324170671202286' }], ['1:05', 'media_timestamp'],
			['4242 4242 4242 4242', 'bank_card_number'], ['a future kind', 'unknown'], ['a sneaky link', 'text_url', { url: 'javascript:alert(1)' }],
		]).concat([{ offset: formatted.length - 'bold to the end'.length, length: 500, type: 'bold' }]) },
		{ content_type: 'text', text: codeText, entities: () => entitiesIn(codeText, [
			['Family', 'bold'], ['𝕏', 'italic'], ['fn main() {\n\tprintln!("hi");\n}', 'pre_code', { language: 'rust' }], ['plain block', 'pre'],
			['A quote from the docs.', 'block_quote'], ['docs', 'italic'], ['A long quote that Telegram folds: line one of it,\nline two of it,\nline three of it,\nline four of it.', 'expandable_block_quote'],
		]) },
		{ content_type: 'text', text: editedText, entities: () => entitiesIn(editedText, [['Friday', 'bold'], ['the usual place', 'text_url', { url: 'https://example.org/place' }]]),
			edited_at: '2026-11-16T08:05:00Z', edits: () => [{ at: '2026-11-16T08:00:00Z', text: 'The meetup is on Thursday, see #meetup.', content_type: 'text', entities: entitiesIn('The meetup is on Thursday, see #meetup.', [['Thursday', 'italic'], ['#meetup', 'hashtag']]) }] },
		{ content_type: 'video_note', media: { round: true } },
		{ content_type: 'location', detail: () => ({ kind: 'location', lat: -6.2088, lon: 106.8456, accuracy: null, live: null }), kinds: { reactions: [{ emoji: '📍', count: 2 }] } },
		{ content_type: 'location', detail: () => ({ kind: 'location', lat: -6.9175, lon: 107.6191, accuracy: 12, live: { period: 3600, heading: 90, proximity_radius: 0, updates: 17 } }) },
		{ content_type: 'venue', detail: () => ({ kind: 'venue', lat: -6.1754, lon: 106.8272, title: 'Monas', address: 'Gambir, Central Jakarta', provider: 'foursquare', venue_id: '4b0588f0f964a520a6d822e3', venue_type: 'arts_entertainment/monument' }) },
		{ content_type: 'contact', detail: (v) => ({ kind: 'contact', first_name: nameOf(a!).split(' ')[0] ?? 'Dewi', last_name: nameOf(a!).split(' ').slice(1).join(' '), phone: v.full ? '+62 812 0000 0101' : null, vcard: v.full ? 'BEGIN:VCARD\nVERSION:3.0\nTEL:+6281200000101\nEND:VCARD' : null, user: refOf(v, a!) }) },
		{ content_type: 'dice', detail: () => ({ kind: 'dice', emoji: '🎲', value: 4, stake: false }) },
		{ content_type: 'dice', detail: () => ({ kind: 'dice', emoji: '', value: 6, stake: true }) },
		{ content_type: 'game', text: 'Beat my score!', detail: () => ({ kind: 'game', title: 'Lumberjack', description: 'Chop the tree, dodge the branches.', short_name: 'lumberjack', photo: null }) },
		{ content_type: 'poll', text: 'For the next meetup.', detail: poll('Which editor do you use?', 'regular', ['Vim', 'Neovim', 'Emacs', 'VS Code', 'Helix']), kinds: { pinned: true, views: 1534, forwards: 12, replies: 4, reactions: REACTIONS } },
		{ content_type: 'poll', detail: poll('Who wrote the first Linux kernel?', 'quiz', ['Richard Stallman', 'Linus Torvalds', 'Andrew Tanenbaum'], {
			closed: true, close_date: '2026-11-10T12:00:00Z',
			question_entities: entitiesIn('Who wrote the first Linux kernel?', [['first', 'bold'], ['Linux kernel', 'italic']]),
			explanation_entities: entitiesIn('Linus wrote the first version in 1991.', [['the first version', 'text_url', { url: 'https://example.org/linux-0.01' }], ['1991', 'bold']]),
		}) },
		{ content_type: 'poll', detail: poll('Pick your favourite syscall', 'regular', Array.from({ length: 25 }, (_, i) => `syscall_${i + 1}`)) },
		{ content_type: 'checklist', text: 'Release checklist', detail: (v) => ({
			kind: 'checklist', tasks_total: 4, others_can_add: true, others_can_mark: true,
			tasks: [
				{ id: 1, text: 'Tag the release', done: true, done_by: refOf(v, b!), done_at: '2026-11-16T09:00:00Z', entities: [] },
				{ id: 2, text: 'Write the changelog', done: true, done_by: refOf(v, x!), done_at: '2026-11-16T10:30:00Z', entities: [] },
				{ id: 3, text: 'Build the packages with make dist', done: false, done_by: null, done_at: null, entities: entitiesIn('Build the packages with make dist', [['make dist', 'code']]) },
				{ id: 4, text: 'Announce it in #news', done: false, done_by: null, done_at: null, entities: entitiesIn('Announce it in #news', [['#news', 'hashtag']]) },
			],
		}) },
		{ content_type: 'story', detail: (v) => ({ kind: 'story', poster: refOf(v, c!), story_id: 42 }) },
		{ content_type: 'paid_media', text: 'Behind the scenes', detail: () => ({ kind: 'paid_media', star_count: 50, items: PAID_ITEMS }) },
		{ content_type: 'invoice', detail: () => ({ kind: 'invoice', title: 'Conference ticket', description: 'One seat at the 2026 meetup.', currency: 'IDR', total_amount: 15000000, is_test: true, photo: null }) },
		{ content_type: 'giveaway', detail: (v) => ({ kind: 'giveaway', prize: { premium_months: 3 }, winner_count: 5, winners_selection_date: '2026-12-01T12:00:00Z', only_new_members: false, prize_description: '', chats: refs(v, [KINDS_GROUP.id, groups[9]!.id]), chats_total: 2 }) },
		{ content_type: 'call', detail: (v) => ({ kind: 'call', video: true, group: true, duration: 0, outcome: 'active', participants: refs(v, [a!, b!, x!]), participants_total: 3 }) },
		{ content_type: 'rich', detail: () => ({ kind: 'rich' }) },
		{ content_type: 'widget', detail: () => ({ kind: 'widget' }) },
		{ content_type: 'expired', detail: () => ({ kind: 'expired' }) },
		{ content_type: 'unsupported', detail: () => ({ kind: 'unsupported' }) },
		{ content_type: 'unknown', tdlib_type: 'messageFancyNewThing' },
		{ content_type: 'service', service: (v) => {
			const ids = [b!, x!, y!];
			const shown = refs(v, ids);
			const others = ids.length - shown.length;
			const names = shown.map((r) => r.title);
			const list = names.length ? `${names.join(', ')}${others ? ` and ${others} other member${others > 1 ? 's' : ''}` : ''}` : `${ids.length} members`;
			return { type: 'chat_add_members', text: `added ${list} to the group`, refs: { users: shown, chats: [], message_id: null } };
		} },
		{ content_type: 'service', service: () => ({ type: 'pin_message', text: 'pinned a message', refs: { users: [], chats: [], message_id: null } }) },
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
		if (s.content_type === 'poll' && pollId === null) pollId = m.message_id;
		if (s.service && s.content_type === 'service' && i === list.length - 1) {
			const make = s.service;
			s.service = (v) => ({ ...make(v), refs: { ...make(v).refs, message_id: pollId } });
		}
		byKey!.set(`group${KINDS_GROUP.id}:${m.message_id}`, s);
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

export const sampleOf = (m: { chat: { id: number; kind: string }; message_id: number }) =>
	(m.chat.kind === 'group' && m.chat.id === PUBLIC_POLLS_GROUP.id ? publicPolls().get(m.message_id) : table().get(`${m.chat.kind}${m.chat.id}:${m.message_id}`)) ?? null;

// The MessageKinds of a message for a viewer, and what a sample changes
// of the message itself.
export function kindsOf(m: { chat: { id: number; kind: string }; message_id: number; content_type: string; text?: string }, v: View) {
	const s = sampleOf(m);
	const svc = s?.service?.(v);
	const base = {
		detail: s?.detail ? s.detail(v) : null,
		// Every service message says its action; the stand-in's own ones
		// name nobody.
		service: svc ? { type: svc.type, refs: svc.refs } : null,
		topic_id: topicOf(m), pinned: false, views: null as number | null, forwards: null as number | null, replies: null as number | null,
		reactions: [] as Kinds['reactions'],
		...(s?.kinds ?? {}),
	};
	return { base, sample: s, serviceText: svc?.text };
}
