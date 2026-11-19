
import Shell from '../ui/Shell.tsx';
import { Avatar, PageHead } from '../ui/primitives.tsx';
import { DataTable, emptyText, FilterBar, LongText, useListNav, withMatches, type Col } from '../ui/list.tsx';
import { useFreshRows } from '../live/useEntities.tsx';
import { Bar, Flags, UserCell, userFlags } from '../ui/bits.tsx';
import { ago, compact, fmt } from '../format.ts';
import type { PageData } from '../page.ts';
import type { Cond, FieldDef } from '../types.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';
import { safeFragment } from '../api/engine.ts';
import { BackendNote, EngineOptions, Highlighted, type Noun } from '../ui/SearchEngine.tsx';

export type UsersProps = S['UsersProps'];
type UserRow = S['UserRow'];

const FIELDS: FieldDef[] = [
	{ key: 'id', label: 'User ID', type: 'number' },
	{ key: 'name', label: 'Name', type: 'text' },
	{ key: 'username', label: 'Username', type: 'text' },
	{ key: 'type', label: 'Type', type: 'enum', values: ['regular', 'bot', 'deleted'] },
	{ key: 'msg_count', label: 'Messages', type: 'number' },
	{ key: 'is_premium', label: 'Premium', type: 'bool' },
	{ key: 'is_verified', label: 'Verified', type: 'bool' },
	{ key: 'is_scam', label: 'Scam', type: 'bool' },
	{ key: 'language', label: 'Language', type: 'text' },
	{ key: 'bio', label: 'Bio', type: 'text' },
	{ key: 'created_at', label: 'First seen', type: 'date' },
];
const NOUN: Noun = { one: 'user', many: 'users' };
const QUICK: [string, Cond[]][] = [
	['Premium', [{ field: 'is_premium', op: 'is', value: 'yes' }]],
	['Bots', [{ field: 'type', op: '=', value: 'bot' }]],
	['Scam', [{ field: 'is_scam', op: 'is', value: 'yes' }]],
];

function Users({ page }: { page: PageData<UsersProps> }) {
	const rows = useFreshRows('user', page.props.rows);
	const L = useListNav({ ...page.props, rows });
	const v = page.viewer;
	// Members see people only through their groups: no global counts.
	const full = v.account?.role !== 'member';
	const max = Math.max(1, ...L.rows.map((u) => u.msg_count ?? 0));
	// The engine's matched fragments of each row's bio, for a bio matches
	// search; the rows without any keep the plain bio.
	const hls = new Map((page.props.highlights ?? []).map((h, i) => [page.props.rows[i]?.id, h]));
	const hl = (u: UserRow) => (hls.get(u.id)?.some((f) => safeFragment(f) !== null) ? <Highlighted fragments={hls.get(u.id)} /> : null);
	const all: Col<UserRow>[] = [
		{ key: 'name', label: 'User', sort: true, cls: 'c-name', render: (u) => <UserCell u={u} /> },
		{ key: 'id', label: 'ID', sort: true, cls: 'mono a-dim', render: (u) => u.id },
		{ key: 'type', label: 'Type', sort: true, render: (u) => <><span className={`a-dot a-dot-${u.type}`} />{u.type}</> },
		{ key: 'msg_count', label: 'Messages', sort: true, align: 'right', render: (u) => <><Bar value={u.msg_count ?? 0} max={max} /><span className="num a-numcell">{fmt(u.msg_count ?? 0)}</span></> },
		{ key: 'flags', label: 'Flags', render: (u) => <Flags flags={userFlags(u).filter((f) => f[0] !== 'deleted' && f[0] !== 'bot')} /> },
		{ key: 'language', label: 'Lang', sort: true, cls: 'a-hide-md', render: (u) => u.language || <span className="a-muted">-</span> },
		// The Drogon list's other columns, sent to full viewers only.
		{ key: 'phone', label: 'Phone', cls: 'a-hide-md mono', render: (u) => u.phone || <span className="a-muted">-</span> },
		{ key: 'bio', label: 'Bio', cls: 'a-hide-md', render: (u) => hl(u) ?? (u.bio ? <LongText text={u.bio} title={`Bio of ${u.name}`} /> : <span className="a-muted">-</span>) },
		{ key: 'created_at', label: 'First seen', sort: true, render: (u) => (u.created_at ? <span title={u.created_at}>{ago(u.created_at, v)}</span> : null) },
	];
	const cols = full ? all : all.filter((c) => ['name', 'id', 'type'].includes(c.key));
	return (
		<Shell crumbs={[['Users', null]]}>
			<PageHead title="Users" sub={full && page.props.archive_total ? `${fmt(page.props.archive_total)} users seen across all logged chats` : `${fmt(L.total)} people in your groups`} />
			<FilterBar L={L} fields={full ? withMatches(FIELDS, L.fields, 'bio') : FIELDS.filter((f) => ['id', 'name', 'username'].includes(f.key))} placeholder="Name, @username or ID" name="Users" quick={full ? QUICK : undefined} />
			<EngineOptions p={page.props} noun={NOUN} facets={false} />
			<BackendNote b={page.props.backend} noun={NOUN} />
			<DataTable
				L={L} cols={cols} noun="users" empty={emptyText(L, !full, 'users', 'No users match these filters.')}
				href={(u) => `/users/${u.id}`}
				mobile={(u) => ({
					lead: <Avatar id={u.id} name={u.name} photo={u.photo} size={36} deleted={u.type === 'deleted'} />,
					title: u.name,
					sub: u.username ? '@' + u.username : String(u.id),
					extra: full && (userFlags(u).length || hl(u)) ? <>{userFlags(u).length > 0 && <Flags flags={userFlags(u)} />}{hl(u)}</> : null,
					side: full ? <><span className="num">{compact(u.msg_count ?? 0)}</span><span className="a-muted">msgs</span></> : null,
				})}
			/>
		</Shell>
	);
}

export const usersRoute: RouteDef<UsersProps> = {
	Page: Users,
	head: () => ({ title: 'Users - tgloggerd', robots: 'noindex' }),
};
