import { Globe } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Avatar, PageHead, Pill } from '../ui/primitives.tsx';
import { DataTable, FilterBar, useListNav, type Col } from '../ui/list.tsx';
import { Bar, GroupCell } from '../ui/bits.tsx';
import { ago, compact, fmt } from '../format.ts';
import type { PageData } from '../page.ts';
import type { Cond, FieldDef, GroupSummary, ListProps } from '../types.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

export type GroupRow = S['GroupsProps']['rows'][number];
export type GroupsProps = S['GroupsProps'];

export const TYPE_LABEL: Record<GroupSummary['type'], string> = { supergroup: 'supergroup', basic_group: 'basic group', channel: 'channel' };

const FIELDS: FieldDef[] = [
	{ key: 'id', label: 'Group ID', type: 'number' },
	{ key: 'title', label: 'Title', type: 'text' },
	{ key: 'username', label: 'Username', type: 'text' },
	{ key: 'type', label: 'Type', type: 'enum', values: ['supergroup', 'basic_group', 'channel'] },
	{ key: 'msg_count', label: 'Messages', type: 'number' },
	{ key: 'description', label: 'Description', type: 'text' },
	{ key: 'exposed', label: 'Public (MCP)', type: 'bool' },
	{ key: 'created_at', label: 'First seen', type: 'date' },
];
const QUICK: [string, Cond[]][] = [
	['Channels', [{ field: 'type', op: '=', value: 'channel' }]],
	['Public', [{ field: 'exposed', op: 'is', value: 'yes' }]],
];

function Groups({ page }: { page: PageData<GroupsProps> }) {
	const L = useListNav(page.props);
	const v = page.viewer;
	const member = v.account?.role === 'member';
	const max = Math.max(1, ...L.rows.map((g) => g.msg_count));
	const cols: Col<GroupRow>[] = [
		{ key: 'title', label: 'Group', sort: true, cls: 'c-name', render: (g) => <GroupCell g={g} /> },
		{ key: 'id', label: 'ID', sort: true, cls: 'mono a-dim', render: (g) => g.id },
		{ key: 'type', label: 'Type', sort: true, render: (g) => <Pill tone={g.type === 'channel' ? 'info' : ''}>{TYPE_LABEL[g.type]}</Pill> },
		{ key: 'msg_count', label: 'Messages', sort: true, align: 'right', render: (g) => <><Bar value={g.msg_count} max={max} /><span className="num a-numcell">{fmt(g.msg_count)}</span></> },
		{ key: 'admin_count', label: 'Admins', sort: true, align: 'right', cls: 'a-hide-md', render: (g) => <span className="num">{g.admin_count}</span> },
		{ key: 'description', label: 'Description', cls: 'a-hide-md a-clip', render: (g) => g.description || <span className="a-muted">-</span> },
		{ key: 'exposed', label: '', render: (g) => g.exposed && <span className="a-public" title="Exposed through MCP: readable without signing in"><Globe size={13} />public</span> },
		{ key: 'created_at', label: 'First seen', sort: true, render: (g) => <span title={g.created_at}>{ago(g.created_at, v)}</span> },
	];
	return (
		<Shell crumbs={[['Groups', null]]}>
			<PageHead title="Groups" sub={member ? `${L.total} groups granted to you` : `${fmt(page.props.archive_total ?? L.total)} groups and channels the logger has seen`} />
			<FilterBar L={L} fields={FIELDS} placeholder="Title, @username or ID" quick={QUICK} />
			<DataTable
				L={L} cols={cols} noun="groups" empty="No groups match these filters."
				href={(g) => `/groups/${g.id}`}
				mobile={(g) => ({
					lead: <Avatar id={g.id} name={g.title} photo={g.photo} size={36} square />,
					title: g.title,
					sub: (g.username ? '@' + g.username + ' . ' : '') + TYPE_LABEL[g.type],
					extra: g.exposed ? <Pill tone="ok">public</Pill> : null,
					side: <><span className="num">{compact(g.msg_count)}</span><span className="a-muted">msgs</span></>,
				})}
			/>
		</Shell>
	);
}

export const groupsRoute: RouteDef<GroupsProps> = { Page: Groups, head: () => ({ title: 'Groups - tgloggerd', robots: 'noindex' }) };
