import { Download, Globe } from 'lucide-react';
import { Link } from '../router.jsx';
import Shell from '../ui/Shell.jsx';
import { useList } from '../ui/hooks.js';
import { Avatar, Pill, PageHead, Btn } from '../ui/primitives.jsx';
import { DataTable, FilterBar } from '../ui/DataTable.jsx';
import { Bar } from './Users.jsx';
import { queryGroups, GROUP_FIELDS } from '../data/groups.js';
import { TOTALS } from '../data/totals.js';
import { fmt, compact, ago } from '../data/seed.js';

export const TYPE_LABEL = { supergroup: 'supergroup', basic_group: 'basic group', channel: 'channel' };

export function GroupCell({ g, size = 28 }) {
	return (
		<Link to={`/groups/${g.id}`} className="a-usercell">
			<Avatar e={g} size={size} square />
			<span>
				<span className="a-uname">{g.title}</span>
				{g.username && <span className="a-handle">@{g.username}</span>}
			</span>
		</Link>
	);
}

const QUICK = [
	['Channels', [{ field: 'type', op: '=', value: 'channel' }]],
	['Public', [{ field: 'exposed', op: 'is', value: 'yes' }]],
];

export default function Groups() {
	const L = useList(queryGroups, { sort: 'msg_count' });
	const max = Math.max(1, ...L.rows.map((g) => g.msg_count));
	const cols = [
		{ key: 'title', label: 'Group', sort: true, cls: 'c-name', render: (g) => <GroupCell g={g} /> },
		{ key: 'id', label: 'ID', sort: true, cls: 'mono a-dim', render: (g) => g.id },
		{ key: 'type', label: 'Type', sort: true, render: (g) => <Pill tone={g.type === 'channel' ? 'info' : ''}>{TYPE_LABEL[g.type]}</Pill> },
		{ key: 'msg_count', label: 'Messages', sort: true, align: 'right', render: (g) => <><Bar value={g.msg_count} max={max} /><span className="num a-numcell">{fmt(g.msg_count)}</span></> },
		{ key: 'admin_count', label: 'Admins', sort: true, align: 'right', cls: 'a-hide-md', render: (g) => <span className="num">{g.admin_count}</span> },
		{ key: 'description', label: 'Description', cls: 'a-hide-md a-clip', render: (g) => g.description || <span className="a-muted">-</span> },
		{ key: 'exposed', label: '', render: (g) => g.exposed && <span className="a-public" title="Public: exposed through MCP and readable without signing in"><Globe size={13} />public</span> },
		{ key: 'created_at', label: 'First seen', sort: true, render: (g) => <span title={g.created_at}>{ago(g.created_at)}</span> },
	];
	return (
		<Shell crumbs={[['Groups', null]]}>
			<PageHead title="Groups" sub={`${fmt(TOTALS.groups)} groups and channels the logger has seen`} actions={<Btn ghost icon={Download}>Export</Btn>} />
			<FilterBar L={L} fields={GROUP_FIELDS} placeholder="Title, @username or ID" quick={QUICK} />
			<DataTable
				L={L} cols={cols} noun="groups"
				href={(g) => `/groups/${g.id}`}
				empty="No groups match these filters."
				mobile={(g) => ({
					lead: <Avatar e={g} size={36} square />,
					title: g.title,
					sub: (g.username ? '@' + g.username + ' . ' : '') + TYPE_LABEL[g.type],
					extra: g.exposed ? <Pill tone="ok">public</Pill> : null,
					side: <><span className="num">{compact(g.msg_count)}</span><span className="a-muted">msgs</span></>,
				})}
			/>
		</Shell>
	);
}
