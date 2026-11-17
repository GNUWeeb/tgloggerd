import { Download } from 'lucide-react';
import { Link } from '../router.jsx';
import Shell from '../ui/Shell.jsx';
import { useList } from '../ui/hooks.js';
import { Avatar, Flags, PageHead, Btn } from '../ui/primitives.jsx';
import { DataTable, FilterBar } from '../ui/DataTable.jsx';
import { queryUsers, USER_FIELDS, userFlags, TOTAL_USERS } from '../data/users.js';
import { fmt, compact, ago } from '../data/seed.js';

export function UserCell({ u, size = 28 }) {
	return (
		<Link to={`/users/${u.id}`} className="a-usercell">
			<Avatar e={u} size={size} />
			<span>
				<span className="a-uname">{u.name}</span>
				{u.username && <span className="a-handle">@{u.username}</span>}
			</span>
		</Link>
	);
}

export function Bar({ value, max }) {
	return <span className="a-bar"><span style={{ width: `${(value / Math.max(1, max)) * 100}%` }} /></span>;
}

const QUICK = [
	['Premium', [{ field: 'is_premium', op: 'is', value: 'yes' }]],
	['Bots', [{ field: 'type', op: '=', value: 'bot' }]],
	['Scam', [{ field: 'is_scam', op: 'is', value: 'yes' }]],
];

export default function Users() {
	const L = useList(queryUsers, { sort: 'msg_count' });
	const max = Math.max(1, ...L.rows.map((u) => u.msg_count));
	const cols = [
		{ key: 'name', label: 'User', sort: true, cls: 'c-name', render: (u) => <UserCell u={u} /> },
		{ key: 'id', label: 'ID', sort: true, cls: 'mono a-dim', render: (u) => u.id },
		{ key: 'type', label: 'Type', sort: true, render: (u) => <><span className={`a-dot a-dot-${u.type}`} />{u.type}</> },
		{ key: 'msg_count', label: 'Messages', sort: true, align: 'right', render: (u) => <><Bar value={u.msg_count} max={max} /><span className="num a-numcell">{fmt(u.msg_count)}</span></> },
		{ key: 'flags', label: 'Flags', render: (u) => <Flags flags={userFlags(u).filter((f) => f[0] !== 'deleted' && f[0] !== 'bot')} /> },
		{ key: 'language', label: 'Lang', sort: true, cls: 'a-hide-md', render: (u) => u.language || <span className="a-muted">-</span> },
		{ key: 'created_at', label: 'First seen', sort: true, render: (u) => <span title={u.created_at}>{ago(u.created_at)}</span> },
		{ key: 'updated_at', label: 'Updated', sort: true, cls: 'a-hide-md', render: (u) => <span title={u.updated_at}>{ago(u.updated_at)}</span> },
	];
	return (
		<Shell crumbs={[['Users', null]]}>
			<PageHead title="Users" sub={`${fmt(TOTAL_USERS)} users seen across all logged chats`} actions={<Btn ghost icon={Download}>Export</Btn>} />
			<FilterBar L={L} fields={USER_FIELDS} placeholder="Name, @username or ID" quick={QUICK} />
			<DataTable
				L={L} cols={cols} noun="users"
				href={(u) => `/users/${u.id}`}
				empty="No users match these filters."
				mobile={(u) => ({
					lead: <Avatar e={u} size={36} />,
					title: u.name,
					sub: u.username ? '@' + u.username : u.id,
					extra: userFlags(u).length ? <Flags flags={userFlags(u)} /> : null,
					side: <><span className="num">{compact(u.msg_count)}</span><span className="a-muted">msgs</span></>,
				})}
			/>
		</Shell>
	);
}
