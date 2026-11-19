// The stand-in's list search, as the Go lists run it (ListState,
// web/docs/search-api.md): a list's own filter keys and the fields of
// its search registry, joined by AND and OR (AND binds tighter), with a
// refused search or one that runs out of time answering the plain list
// and the reason. Only what the sample data can answer is modelled.

// value is a list for `in` (one of), a string for the others.
export type Cond = { field: string; op: string; value: string | string[]; n?: 'AND' | 'OR' };
type Row = Record<string, unknown>;
type FieldType = 'text' | 'int' | 'bool' | 'datetime' | 'enum';
// needs_chat: no index of its own, so only beside a chat_id = filter.
export interface SField { key: string; label: string; type: FieldType; sortable: boolean; operators: string[]; enum?: string; needs_chat?: boolean; get?: (r: Row) => unknown }

const TEXT = ['=', '!=', 'LIKE', 'NOT LIKE', '%LIKE%', 'NOT %LIKE%'];
const CMP = ['=', '!=', '<', '>', '<=', '>='];
const BOOL = ['=', '!='];
const NULLS = ['IS NULL', 'IS NOT NULL'];
const OWN_OPS = ['=', '!=', '>', '<', '>=', '<=', 'contains', 'empty', 'is', 'in'];

// The known service and TDLib types, as Go lists them: a sample of the
// daemon's names (docs/design/message-types.md). A value not listed is
// still taken, as Go takes a type newer than its list.
const SERVICE_TYPES = ['chat_create', 'chat_change_title', 'chat_change_photo', 'chat_delete_photo', 'chat_add_members', 'chat_join_by_link', 'chat_join_by_request', 'chat_delete_member', 'chat_upgrade_to', 'chat_upgrade_from', 'pin_message', 'screenshot_taken', 'chat_set_message_auto_delete_time', 'forum_topic_created', 'forum_topic_edited', 'contact_registered', 'video_chat_started', 'video_chat_ended', 'gift', 'custom_service_action'];
const TDLIB_TYPES = ['messageText', 'messagePhoto', 'messageVideo', 'messageVideoNote', 'messageDocument', 'messageAudio', 'messageVoiceNote', 'messageSticker', 'messageAnimation', 'messagePaidMedia', 'messageLocation', 'messageLiveLocation', 'messageVenue', 'messageContact', 'messageDice', 'messageStakeDice', 'messagePoll', 'messageChecklist', 'messageStory', 'messageCall', 'messageChatAddMembers', 'messagePinMessage'];

// The registries: the fields each list's search offers.
export const REGISTRY: Record<string, SField[]> = {
	users: [
		{ key: 'first_name', label: 'First name', type: 'text', sortable: true, operators: TEXT },
		{ key: 'last_name', label: 'Last name', type: 'text', sortable: true, operators: TEXT },
		{ key: 'phone', label: 'Phone', type: 'text', sortable: false, operators: [...TEXT, ...NULLS] },
		{ key: 'bio', label: 'Bio', type: 'text', sortable: false, operators: [...TEXT, ...NULLS] },
		{ key: 'is_premium', label: 'Premium', type: 'bool', sortable: true, operators: BOOL },
		{ key: 'is_verified', label: 'Verified', type: 'bool', sortable: true, operators: BOOL },
		{ key: 'is_contact', label: 'Contact', type: 'bool', sortable: false, operators: BOOL },
		{ key: 'has_photo', label: 'Has photo', type: 'bool', sortable: false, operators: NULLS, get: (r) => (r.has_photo ? 1 : null) },
		{ key: 'created_at', label: 'Created', type: 'datetime', sortable: true, operators: CMP },
	],
	// A member's users: only these, as in Go.
	users_member: [
		{ key: 'has_photo', label: 'Has photo', type: 'bool', sortable: false, operators: NULLS, get: (r) => (r.has_photo ? 1 : null) },
	],
	groups: [
		{ key: 'title', label: 'Title', type: 'text', sortable: true, operators: TEXT },
		{ key: 'description', label: 'Description', type: 'text', sortable: false, operators: [...TEXT, ...NULLS] },
		{ key: 'updated_at', label: 'Updated', type: 'datetime', sortable: true, operators: CMP },
		{ key: 'admin_count', label: 'Admins', type: 'int', sortable: true, operators: CMP },
	],
	messages: [
		{ key: 'text', label: 'Text', type: 'text', sortable: false, operators: ['matches', '%LIKE%', 'NOT %LIKE%', ...NULLS] },
		{ key: 'message_id', label: 'Message ID', type: 'int', sortable: true, operators: CMP },
		{ key: 'tdlib_type', label: 'TDLib type', type: 'enum', sortable: false, operators: ['=', '!='], enum: TDLIB_TYPES.join(','), needs_chat: true },
		{ key: 'service_type', label: 'Service type', type: 'enum', sortable: false, operators: ['=', '!='], enum: SERVICE_TYPES.join(','), needs_chat: true },
	],
};

export const fieldsOf = (fs: SField[]) => fs.map(({ get: _g, ...f }) => f);

const like = (pattern: string, s: string) => new RegExp('^' + pattern.replace(/[.*+?^${}()|[\]\\]/g, '\\$&').replace(/%/g, '.*').replace(/_/g, '.') + '$', 'is').test(s);
const has = (s: string, part: string) => s.toLowerCase().includes(part.toLowerCase());
// MySQL boolean full-text, roughly: +word and plain words must appear,
// -word must not, "a phrase" appears as is.
function matches(text: string, q: string): boolean {
	const terms = q.match(/[-+]?"[^"]+"|\S+/g) ?? [];
	return terms.length > 0 && terms.every((t) => {
		const neg = t.startsWith('-');
		const w = t.replace(/^[-+]/, '').replace(/^"|"$/g, '');
		return neg ? !has(text, w) : has(text, w);
	});
}

function ownMatch(v: unknown, cond: Cond): boolean {
	if (cond.op === 'in') return (cond.value as string[]).includes(String(v));
	const c = { ...cond, value: String(cond.value) };
	const num = typeof v === 'number';
	switch (c.op) {
		case 'contains': return has(String(v ?? ''), c.value);
		case '=': return String(v) === String(c.value);
		case '!=': return String(v) !== String(c.value);
		case '>': return num ? (v as number) > Number(c.value) : String(v) > c.value;
		case '<': return num ? (v as number) < Number(c.value) : String(v) < c.value;
		case '>=': return num ? (v as number) >= Number(c.value) : String(v) >= c.value;
		case '<=': return num ? (v as number) <= Number(c.value) : String(v) <= c.value;
		case 'is': return Boolean(v) === (c.value === 'yes' || c.value === 'true');
		case 'empty': return !v;
		default: return false;
	}
}

function searchMatch(f: SField, row: Row, c: Cond): boolean {
	const raw = f.get ? f.get(row) : row[f.key];
	if (c.op === 'in') return raw !== null && raw !== undefined && (c.value as string[]).includes(String(raw));
	if (c.op === 'IS NULL') return raw === null || raw === undefined || raw === '';
	if (c.op === 'IS NOT NULL') return !(raw === null || raw === undefined || raw === '');
	if (f.type === 'bool') return (c.op === '=') === (Boolean(raw) === (c.value === '1'));
	if (f.type === 'int') return ownMatch(Number(raw), c);
	const s = String(raw ?? '');
	const v = String(c.value);
	switch (c.op) {
		case 'LIKE': return like(v, s);
		case 'NOT LIKE': return !like(v, s);
		case '%LIKE%': return has(s, v);
		case 'NOT %LIKE%': return !has(s, v);
		case 'matches': return matches(s, v);
		default: return ownMatch(s, c);
	}
}

export type ListError = { code: 'bad_search' | 'search_timeout'; message: string };

// The conditions as one test of a row, or why the list refuses them.
export function compile(conds: Cond[], own: string[], fields: SField[]): ((r: Row) => boolean) | ListError {
	const tests: ((r: Row) => boolean)[] = [];
	for (const c of conds) {
		// One of a list takes a non-empty list; the others a string.
		if ((c.op === 'in') !== Array.isArray(c.value) || (c.op === 'in' && !c.value.length)) return { code: 'bad_search', message: c.op === 'in' ? `${c.field} in takes a list of values.` : `${c.field} takes a single value.` };
		// A list's own key with one of its own operators keeps its own
		// meaning (the message lists' text contains); else the registry's.
		const f = own.includes(c.field) && OWN_OPS.includes(c.op) ? undefined : fields.find((x) => x.key === c.field);
		if (f) {
			// Go takes `in` on every enum field; it is not listed among the
			// operators until the schema has it.
			if (!f.operators.includes(c.op) && !(c.op === 'in' && f.type === 'enum')) return { code: 'bad_search', message: `${f.label} does not take ${c.op}.` };
			if (f.type === 'int' && !NULLS.includes(c.op) && !/^-?\d+$/.test(String(c.value))) return { code: 'bad_search', message: `${f.label} takes a whole number.` };
			if (f.type === 'datetime' && !/^\d{4}-\d{2}-\d{2}( \d{2}:\d{2}(:\d{2})?)?$/.test(String(c.value))) return { code: 'bad_search', message: `${f.label} takes a date and time, YYYY-MM-DD HH:MM:SS.` };
			tests.push((r) => searchMatch(f, r, c));
		} else if (own.includes(c.field)) {
			if (!OWN_OPS.includes(c.op)) return { code: 'bad_search', message: `${c.field} does not take ${c.op}.` };
			tests.push((r) => ownMatch(r[c.field], c));
		} else return { code: 'bad_search', message: `There is no field ${c.field} to filter by.` };
	}
	// A field that needs one chat is refused without a chat_id = filter in
	// an all-AND chain, as Go refuses it.
	const needs = conds.map((c) => fields.find((x) => x.key === c.field)).find((f) => f?.needs_chat);
	const bound = conds.some((c) => c.field === 'chat_id' && c.op === '=') && conds.slice(0, -1).every((c) => c.n !== 'OR');
	if (needs && !bound) return { code: 'bad_search', message: `field '${needs.key}' requires a 'chat_id =' condition ANDed with the others` };
	// AND binds tighter than OR: split the chain at each OR.
	const groups: ((r: Row) => boolean)[][] = [[]];
	tests.forEach((t, i) => {
		groups[groups.length - 1]!.push(t);
		if (conds[i]!.n === 'OR' && i < tests.length - 1) groups.push([]);
	});
	return (r) => groups.some((g) => g.every((t) => t(r)));
}

// The query of a Drogon list URL (search, offset, limit, sort, order)
// as this list's, or null when it is not one (Go's legacyQuery).
export function legacyQuery(q: URLSearchParams): URLSearchParams | null {
	if (!['search', 'offset', 'limit', 'order'].some((k) => q.has(k))) return null;
	if (['f', 'page', 'per', 'dir'].some((k) => q.has(k))) return null;
	const out = new URLSearchParams();
	if (q.get('q')) out.set('q', q.get('q')!);
	let conds: { c: string; o: string; v?: string; n?: string }[] = [];
	try { conds = q.get('search') ? JSON.parse(q.get('search')!) : []; } catch { conds = []; }
	if (conds.length) out.set('f', JSON.stringify(conds.map((c, i) => ({ field: c.c, op: c.o, value: c.v ?? '', ...(c.n === 'OR' && i < conds.length - 1 ? { n: 'OR' } : {}) }))));
	let per = 0;
	if (q.has('limit')) { per = Math.min(1000, Math.max(1, Number(q.get('limit')) || 1)); out.set('per', String(per)); }
	const off = Math.max(0, Number(q.get('offset')) || 0);
	if (off > 0) {
		if (!per) { per = 50; out.set('per', '50'); }
		const page = Math.floor(off / per) + 1;
		if (page > 1) out.set('page', String(page));
	}
	if (q.get('sort')) out.set('sort', q.get('sort')!);
	if (q.has('order')) out.set('dir', q.get('order')!.toLowerCase() === 'asc' ? 'asc' : 'desc');
	return out;
}

// The sample text that stands for a search running out of time.
export const SLOW = 'slowsearch';
