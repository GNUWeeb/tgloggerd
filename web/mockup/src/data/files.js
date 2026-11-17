// Files, built from the media of the generated messages.
import { mulberry32, queryRows } from './seed.js';
import { groupMessages, privateMessages, senderOf } from './messages.js';

const EXT = { photo: 'jpg', video: 'mp4', sticker: 'webp', voice: 'ogg', animation: 'mp4', audio: 'mp3' };

let all = null;
export function files() {
	if (all) return all;
	const g = mulberry32(777);
	const out = [];
	const add = (m, media, type) => {
		const ext = media.ext || EXT[type] || 'bin';
		out.push({
			id: media.fileId,
			file_type: type,
			name: media.name || `${type}_${media.fileId}.${ext}`,
			ext,
			size: media.size || Math.floor(30000 + g() * 300000),
			hits: Math.floor(Math.pow(g(), 3) * 400),
			stored: media.stored !== false,
			tg_file_id: 'AgACAgUAAxkBAAI' + Math.floor(g() * 1e12).toString(36).toUpperCase() + 'QAC' + Math.floor(g() * 1e9).toString(36),
			created_at: m.date,
			hue: media.hue ?? Math.floor(g() * 360),
			w: media.w, h: media.h, duration: media.duration, emoji: media.emoji,
			msg: m,
			chat_title: m.chat.title || m.chat.name,
			sender_name: senderOf(m)?.name || '',
			sender_id: senderOf(m)?.id || 0,
		});
	};
	for (const m of [...groupMessages(), ...privateMessages()]) {
		if (!m.media) continue;
		const type = m.content_type === 'document' && m.media.ext === 'mp3' ? 'audio' : m.content_type;
		add(m, m.media, type);
		for (const a of m.album || []) add(m, { ...a, w: 4, h: 3, stored: true }, 'photo');
	}
	out.sort((a, b) => (a.created_at < b.created_at ? 1 : -1));
	all = out;
	return all;
}

export const FILE_TYPES = ['photo', 'video', 'document', 'audio', 'voice', 'sticker', 'animation'];

export const FILE_FIELDS = [
	{ key: 'id', label: 'File ID', type: 'number' },
	{ key: 'file_type', label: 'Type', type: 'enum', values: FILE_TYPES },
	{ key: 'name', label: 'Name', type: 'text' },
	{ key: 'ext', label: 'Extension', type: 'text' },
	{ key: 'size', label: 'Size (bytes)', type: 'number' },
	{ key: 'hits', label: 'Hits', type: 'number' },
	{ key: 'stored', label: 'Stored', type: 'bool' },
	{ key: 'sender_id', label: 'Sender id', type: 'number' },
	{ key: 'created_at', label: 'First seen', type: 'date' },
];

export function queryFiles(state) {
	return queryRows(files(), state, (f) => `${f.name} ${f.id} ${f.chat_title}`);
}

export function totalBytes() {
	return files().reduce((a, f) => a + (f.stored ? f.size : 0), 0);
}
