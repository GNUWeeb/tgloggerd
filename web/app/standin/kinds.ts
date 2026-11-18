// The message kinds of web/docs/message-kinds.md in the stand-in: one
// sample message of each kind in a sample group (and a private call in
// a private chat), the service messages' actions, the interaction counts
// and a forum group with topics. The samples replace the newest messages
// of those chats but their newest two, so the chats' sizes and the
// message lists stay as they were. Whom a detail names follows the
// viewer as Go does: hidden users and chats are null or left out, the
// *_total fields still count them, and only full viewers get a
// contact's phone.
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

type Sample = { content_type: string; text?: string; detail?: (v: View) => unknown; service?: (v: View) => { type: string; text: string; refs: { users: Ref[]; chats: Ref[]; message_id: number | null } }; media?: { round?: boolean }; tdlib_type?: string; kinds?: Partial<Kinds> };
type Kinds = { pinned: boolean; views: number | null; forwards: number | null; replies: number | null; reactions: { emoji?: string; custom_emoji_id?: string; paid?: boolean; count: number }[] };

const REACTIONS = [{ emoji: '👍', count: 12 }, { emoji: '🔥', count: 5 }, { custom_emoji_id: '5368324170671202286', count: 3 }, { paid: true, count: 2 }];
const poll = (question: string, type: 'regular' | 'quiz', opts: string[], extra = {}) => () => {
	const votes = opts.map((_, i) => (opts.length - i) * 7 + (i % 3));
	const total = votes.reduce((a, b) => a + b, 0);
	return {
		kind: 'poll', question, type, anonymous: type === 'regular', multiple: false, closed: false, total_voters: total, close_date: null, explanation: type === 'quiz' ? 'Linus wrote the first version in 1991.' : null,
		options: opts.slice(0, 20).map((text, i) => ({ text, voters: votes[i]!, percent: Math.round((votes[i]! / total) * 100), correct: type === 'quiz' && i === 1 })),
		options_total: opts.length, ...extra,
	};
};

// The samples, oldest first; the poll's message id is filled in later
// for the pin's service message.
function samples(): Sample[] {
	const [a, b, c] = posters();
	const [x, y] = strangers();
	return [
		{ content_type: 'video_note', media: { round: true } },
		{ content_type: 'location', detail: () => ({ kind: 'location', lat: -6.2088, lon: 106.8456, accuracy: null, live: null }), kinds: { reactions: [{ emoji: '📍', count: 2 }] } },
		{ content_type: 'location', detail: () => ({ kind: 'location', lat: -6.9175, lon: 107.6191, accuracy: 12, live: { period: 3600, heading: 90, proximity_radius: 0, updates: 17 } }) },
		{ content_type: 'venue', detail: () => ({ kind: 'venue', lat: -6.1754, lon: 106.8272, title: 'Monas', address: 'Gambir, Central Jakarta', provider: 'foursquare', venue_id: '4b0588f0f964a520a6d822e3', venue_type: 'arts_entertainment/monument' }) },
		{ content_type: 'contact', detail: (v) => ({ kind: 'contact', first_name: nameOf(a!).split(' ')[0] ?? 'Dewi', last_name: nameOf(a!).split(' ').slice(1).join(' '), phone: v.full ? '+62 812 0000 0101' : null, vcard: v.full ? 'BEGIN:VCARD\nVERSION:3.0\nTEL:+6281200000101\nEND:VCARD' : null, user: refOf(v, a!) }) },
		{ content_type: 'dice', detail: () => ({ kind: 'dice', emoji: '🎲', value: 4, stake: false }) },
		{ content_type: 'dice', detail: () => ({ kind: 'dice', emoji: '', value: 6, stake: true }) },
		{ content_type: 'game', text: 'Beat my score!', detail: () => ({ kind: 'game', title: 'Lumberjack', description: 'Chop the tree, dodge the branches.', short_name: 'lumberjack', photo: null }) },
		{ content_type: 'poll', text: 'For the next meetup.', detail: poll('Which editor do you use?', 'regular', ['Vim', 'Neovim', 'Emacs', 'VS Code', 'Helix']), kinds: { pinned: true, views: 1534, forwards: 12, replies: 4, reactions: REACTIONS } },
		{ content_type: 'poll', detail: poll('Who wrote the first Linux kernel?', 'quiz', ['Richard Stallman', 'Linus Torvalds', 'Andrew Tanenbaum']) },
		{ content_type: 'poll', detail: poll('Pick your favourite syscall', 'regular', Array.from({ length: 25 }, (_, i) => `syscall_${i + 1}`)) },
		{ content_type: 'checklist', text: 'Release checklist', detail: (v) => ({
			kind: 'checklist', tasks_total: 4, others_can_add: true, others_can_mark: true,
			tasks: [
				{ id: 1, text: 'Tag the release', done: true, done_by: refOf(v, b!), done_at: '2026-11-16T09:00:00Z' },
				{ id: 2, text: 'Write the changelog', done: true, done_by: refOf(v, x!), done_at: '2026-11-16T10:30:00Z' },
				{ id: 3, text: 'Build the packages', done: false, done_by: null, done_at: null },
				{ id: 4, text: 'Announce it', done: false, done_by: null, done_at: null },
			],
		}) },
		{ content_type: 'story', detail: (v) => ({ kind: 'story', poster: refOf(v, c!), story_id: 42 }) },
		{ content_type: 'paid_media', text: 'Behind the scenes', detail: () => ({ kind: 'paid_media', star_count: 50, items: [{ kind: 'preview', media: null }, { kind: 'photo', media: null }, { kind: 'video', media: null }] }) },
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
	}
	return byKey;
}

export const sampleOf = (m: { chat: { id: number; kind: string }; message_id: number }) => table().get(`${m.chat.kind}${m.chat.id}:${m.message_id}`) ?? null;

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
