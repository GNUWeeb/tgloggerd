// Messages: one generator per chat gives a time-ordered stream (used by
// the chat views); the message search lists are built from a few streams.
import { mulberry32, NOW, DAY, iso, queryRows } from './seed.js';
import { users, featuredUser, getUser } from './users.js';
import { groups, featuredGroup, getGroup } from './groups.js';

const TEXTS = [
	'anyone tried the new kernel rc yet?', 'just use `perf record -g` and look at the flame graph',
	'that segfault is a use-after-free, check the free path', 'lol', 'thanks, that fixed it',
	'is io_uring faster than epoll for this workload?', 'good morning everyone',
	'can someone review my patch? it is a two-liner', 'the build is broken on arm64 again',
	'I think the bug is in the error path, not the hot path', 'this is why we cannot have nice things',
	'ok merged', 'try `git bisect run ./test.sh`, it finds it in 10 steps',
	'which distro for an old thinkpad?', 'debian stable, no question',
	'the benchmark numbers look suspicious, did you pin the CPU frequency?',
	'meeting moved to 8pm', 'nice, works on my machine now', 'see the docs at https://kernel.org/doc/html/latest/',
	'https://github.com/torvalds/linux/commit/1a2b3c4d', 'mantap', 'wkwkwk', 'sip, thanks bang',
	'Hari ini ada yang mau ngoding bareng?', 'who broke CI', 'compiling...', '+1',
];
const FILES = [
	['document', 'kernel-config.txt', 'txt', 14 << 10], ['document', 'slides-io_uring.pdf', 'pdf', 3 << 20],
	['document', 'patch-v3.diff', 'diff', 9 << 10], ['document', 'dump.tar.gz', 'gz', 48 << 20],
	['audio', 'podcast-ep12.mp3', 'mp3', 22 << 20], ['document', 'notes.md', 'md', 4 << 10],
];
const STICKERS = ['😂', '👍', '🔥', '🫡', '😭', '🤔', '💀', '🙏'];
export const CONTENT_TYPES = ['text', 'photo', 'video', 'document', 'audio', 'voice', 'sticker', 'animation', 'service'];

const SERVICE = ['joined the group', 'left the group', 'pinned a message', 'changed the group photo', 'was added by an admin'];

function members(chat) {
	if (chat.kind === 'user') return [chat];
	const g = mulberry32(Number(-chat.id % 4993));
	return Array.from({ length: 14 }, () => users[Math.floor(g() * 200)]).filter((u) => u.type !== 'deleted');
}

let nextFile = 880000;
const streams = new Map();

// A chat's messages, oldest first. Private chats use the logger account
// as the other side (is_outgoing).
export function chatStream(chat) {
	const key = chat.kind + chat.id;
	if (streams.has(key)) return streams.get(key);
	const g = mulberry32(Number(Math.abs(chat.id) % 1000003) + 17);
	const pick = (a) => a[Math.floor(g() * a.length)];
	const ppl = members(chat);
	const n = chat.kind === 'user' ? 140 : 260;
	const out = [];
	// Send times first, shifted so the newest message is minutes old.
	const ts = [];
	let acc = 0;
	for (let i = 0; i < n; i++) { acc += Math.floor(g() < 0.15 ? g() * DAY * 0.6 : g() * 900000); ts.push(acc); }
	const shift = NOW - Math.floor(g() * 1800000) - 120000 - acc;
	let t;
	let msgId = 400000 + Math.floor(g() * 300000);
	let prevSender = null;
	for (let i = 0; i < n; i++) {
		t = ts[i] + shift;
		msgId += 1 + Math.floor(g() * 3);
		const outgoing = chat.kind === 'user' && g() < 0.45;
		let sender = chat.kind === 'user' ? (outgoing ? null : chat) : (g() < 0.5 && prevSender ? prevSender : pick(ppl));
		const roll = g();
		const m = {
			id: (chat.kind === 'user' ? 7000000 : 90000000) + Math.abs(chat.id % 9973) * 1000 + i,
			scope: chat.kind === 'user' ? 'private' : 'group',
			chat, sender, is_outgoing: outgoing,
			message_id: msgId,
			date: iso(t),
			content_type: 'text',
			text: '',
		};
		if (chat.kind === 'group' && roll < 0.04) {
			m.content_type = 'service';
			m.text = pick(SERVICE);
		} else if (roll < 0.14) {
			m.content_type = 'photo';
			m.media = { hue: Math.floor(g() * 360), w: g() < 0.5 ? 4 : 3, h: 3, size: Math.floor(80000 + g() * 900000), stored: g() > 0.08, fileId: nextFile++ };
			m.text = g() < 0.4 ? pick(TEXTS) : '';
			if (g() < 0.25) {
				m.album = Array.from({ length: 2 + Math.floor(g() * 3) }, () => ({ hue: Math.floor(g() * 360), fileId: nextFile++ }));
			}
		} else if (roll < 0.18) {
			m.content_type = 'video';
			m.media = { hue: Math.floor(g() * 360), w: 16, h: 9, size: Math.floor(2e6 + g() * 4e7), duration: Math.floor(5 + g() * 300), stored: g() > 0.2, fileId: nextFile++ };
			m.text = g() < 0.3 ? pick(TEXTS) : '';
		} else if (roll < 0.22) {
			const f = pick(FILES);
			m.content_type = f[0];
			m.media = { name: f[1], ext: f[2], size: f[3], stored: g() > 0.1, fileId: nextFile++ };
		} else if (roll < 0.26) {
			m.content_type = 'sticker';
			m.media = { emoji: pick(STICKERS), fileId: nextFile++, stored: true };
		} else if (roll < 0.28) {
			m.content_type = 'voice';
			m.media = { duration: Math.floor(3 + g() * 90), size: Math.floor(20000 + g() * 400000), stored: true, fileId: nextFile++, wave: Array.from({ length: 40 }, () => g()) };
		} else {
			m.text = pick(TEXTS);
			if (m.text.startsWith('https://') || m.text.includes('https://')) {
				m.link_preview = m.text.includes('github')
					? { site: 'GitHub', title: 'io_uring: fix a race in the completion path', desc: 'A use-after-free could happen when a request completed while it was being cancelled.' }
					: { site: 'kernel.org', title: 'The Linux Kernel documentation', desc: 'The following is a collection of user-oriented documents.' };
			}
		}
		if (m.content_type !== 'service') {
			if (out.length > 3 && g() < 0.14) {
				const target = out[out.length - 1 - Math.floor(g() * Math.min(12, out.length - 1))];
				if (target.content_type !== 'service') m.reply_to = target;
			}
			if (g() < 0.04) m.forward = { from: pick(['Linux Weekly News', 'Rust Indonesia', 'Hacker News Bot', 'Someone']) };
			if (g() < 0.07 && m.text) {
				m.edits = [{ at: iso(t), text: m.text.length > 10 ? m.text.slice(0, -5) + '...' : m.text + ' ??' }];
				m.edited_at = iso(t + 120000);
			}
			if (g() < 0.05) m.deleted_at = iso(Math.min(NOW, t + Math.floor(g() * DAY)));
		}
		prevSender = sender;
		out.push(m);
	}
	streams.set(key, out);
	return out;
}

export function senderOf(m) {
	return m.is_outgoing ? { kind: 'user', id: 0, name: 'You (logger account)', hue: 210, has_photo: true } : m.sender;
}

// Message search lists.
let groupMsgs = null;
let privateMsgs = null;
export function groupMessages() {
	if (!groupMsgs) {
		groupMsgs = groups.slice(0, 6).flatMap(chatStream).map(flatten);
		groupMsgs.sort((a, b) => (a.date < b.date ? 1 : -1));
	}
	return groupMsgs;
}
export function privateMessages() {
	if (!privateMsgs) {
		privateMsgs = [featuredUser, ...users.slice(0, 8)].filter((u, i, a) => a.indexOf(u) === i && u.type !== 'deleted')
			.flatMap(chatStream).map(flatten);
		privateMsgs.sort((a, b) => (a.date < b.date ? 1 : -1));
	}
	return privateMsgs;
}

function flatten(m) {
	const s = senderOf(m);
	return Object.assign(m, {
		chat_id: m.chat.id,
		chat_title: m.chat.title || m.chat.name,
		sender_id: s?.id ?? 0,
		sender_name: s?.name ?? '',
		is_forwarded: !!m.forward,
		is_deleted: !!m.deleted_at,
		is_edited: !!m.edits,
	});
}

export function queryMessages(scope, state) {
	const all = scope === 'group' ? groupMessages() : privateMessages();
	return queryRows(all, state, (m) => `${m.text} ${m.media?.name || ''}`);
}

export function getMessage(scope, id) {
	const all = scope === 'group' ? groupMessages() : privateMessages();
	return all.find((m) => String(m.id) === String(id));
}

export const MSG_FIELDS = {
	group: [
		{ key: 'text', label: 'Text', type: 'text' },
		{ key: 'content_type', label: 'Content type', type: 'enum', values: CONTENT_TYPES },
		{ key: 'chat_id', label: 'Group id', type: 'number' },
		{ key: 'sender_id', label: 'Sender user id', type: 'number' },
		{ key: 'is_forwarded', label: 'Forwarded', type: 'bool' },
		{ key: 'is_edited', label: 'Edited', type: 'bool' },
		{ key: 'is_deleted', label: 'Deleted', type: 'bool' },
		{ key: 'date', label: 'Sent', type: 'date' },
	],
	private: [
		{ key: 'text', label: 'Text', type: 'text' },
		{ key: 'content_type', label: 'Content type', type: 'enum', values: CONTENT_TYPES.filter((t) => t !== 'service') },
		{ key: 'chat_id', label: 'User (chat) id', type: 'number' },
		{ key: 'is_outgoing', label: 'Outgoing', type: 'bool' },
		{ key: 'is_forwarded', label: 'Forwarded', type: 'bool' },
		{ key: 'is_deleted', label: 'Deleted', type: 'bool' },
		{ key: 'date', label: 'Sent', type: 'date' },
	],
};

export function chatOf(kind, id) {
	return kind === 'group' ? getGroup(id) : getUser(id);
}

export { featuredGroup };
