// Integration settings: forwarding routes and MCP. Synthetic.
import { groups } from './groups.js';
import { users } from './users.js';
import { NOW, DAY, iso } from './seed.js';

export const t2d = [
	{ id: 1, chat: groups[0], url: 'https://discord.com/api/webhooks/1172839401/aB3dEfGh-telegram-dev', types: ['sticker', 'text', 'photo', 'video', 'document', 'other'], enabled: true, last: iso(NOW - 120000), sent: 182340 },
	{ id: 2, chat: groups[1], url: 'https://discord.com/api/webhooks/1172839555/xY9zKernel-id', types: ['text', 'photo'], enabled: true, last: iso(NOW - 3600000), sent: 40211 },
	{ id: 3, chat: groups[9], url: 'https://discord.com/api/webhooks/1180000001/ctf-feed', types: ['text', 'document'], enabled: false, last: iso(NOW - 12 * DAY), sent: 912 },
	{ id: 4, chat: users[3], url: 'https://discord.com/api/webhooks/1190000042/dm-mirror', types: ['text'], enabled: true, last: iso(NOW - 2 * DAY), sent: 77 },
];

export const d2t = [
	{ id: 1, channel: '1093447120571433021', channel_name: '#telegram-dev', chat: groups[0], bot: '@gnuweeb_bridge_bot', types: ['sticker', 'text', 'photo', 'video', 'document'], enabled: true, last: iso(NOW - 600000), sent: 5120 },
	{ id: 2, channel: '1101992283746112400', channel_name: '#announcements', chat: groups[2], bot: '@rust_id_bridge_bot', types: ['text'], enabled: true, last: iso(NOW - 5 * DAY), sent: 64 },
];

export const mcpExposed = groups.filter((g) => g.exposed).map((g, i) => ({
	group: g,
	public_now: i !== 2,
	note: ['Public community chat, approved by the admins', 'Rust ID agreed to be indexed', 'CTF writeups, archive only'][i],
	added: iso(NOW - (30 + i * 40) * DAY),
}));

export const mcpTokens = [
	{ id: 1, name: 'laptop', owner: 'admin', created: iso(NOW - 80 * DAY), last_used: iso(NOW - 900000), revoked: null },
	{ id: 2, name: 'research-agent', owner: 'admin', created: iso(NOW - 40 * DAY), last_used: iso(NOW - 3 * DAY), revoked: null },
	{ id: 3, name: 'old ci job', owner: 'admin', created: iso(NOW - 200 * DAY), last_used: iso(NOW - 120 * DAY), revoked: iso(NOW - 100 * DAY) },
	{ id: 4, name: '', owner: 'admin', created: iso(NOW - 10 * DAY), last_used: '', revoked: null },
];

export const chatOptions = [...groups.slice(0, 40), ...users.filter((u) => u.type !== 'deleted').slice(0, 40)];
