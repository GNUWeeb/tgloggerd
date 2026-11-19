import { Globe } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Avatar, PageHead, Pill } from '../ui/primitives.tsx';
import { DataTable, emptyText, FilterBar, LongText, useListNav, withMatches, type Col } from '../ui/list.tsx';
import { useFreshRows } from '../live/useEntities.tsx';
import { Bar, GroupCell } from '../ui/bits.tsx';
import { ago, compact, fmt } from '../format.ts';
import type { PageData } from '../page.ts';
import type { Cond, FieldDef, GroupSummary, ListProps } from '../types.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';
import { safeFragment } from '../api/engine.ts';
import { BackendNote, EngineOptions, Highlighted, type Noun } from '../ui/SearchEngine.tsx';

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
const NOUN: Noun = { one: 'group', many: 'groups' };
const QUICK: [string, Cond[]][] = [
	['Channels', [{ field: 'type', op: '=', value: 'channel' }]],
	['Public', [{ field: 'exposed', op: 'is', value: 'yes' }]],
];

function Groups({ page }: { page: PageData<GroupsProps> }) {
	const rows = useFreshRows('group', page.props.rows);
	const L = useListNav({ ...page.props, rows });
	const v = page.viewer;
	const member = v.account?.role === 'member';
	const max = Math.max(1, ...L.rows.map((g) => g.msg_count));
	// The engine's matched fragments of each row's description, for a
	// description matches search; the rows without any keep it plain.
	const hls = new Map((page.props.highlights ?? []).map((h, i) => [page.props.rows[i]?.id, h]));
	const hl = (g: GroupRow) => (hls.get(g.id)?.some((f) => safeFragment(f) !== null) ? <Highlighted fragments={hls.get(g.id)} /> : null);
	const cols: Col<GroupRow>[] = [
		{ key: 'title', label: 'Group', sort: true, cls: 'c-name', render: (g) => <GroupCell g={g} /> },
		{ key: 'id', label: 'ID', sort: true, cls: 'mono a-dim', render: (g) => g.id },
		{ key: 'type', label: 'Type', sort: true, render: (g) => <Pill tone={g.type === 'channel' ? 'info' : ''}>{TYPE_LABEL[g.type]}</Pill> },
		{ key: 'msg_count', label: 'Messages', sort: true, align: 'right', render: (g) => <><Bar value={g.msg_count} max={max} /><span className="num a-numcell">{fmt(g.msg_count)}</span></> },
		{ key: 'admin_count', label: 'Admins', sort: true, align: 'right', cls: 'a-hide-md', render: (g) => <span className="num">{g.admin_count}</span> },
		{ key: 'description', label: 'Description', cls: 'a-hide-md', render: (g) => hl(g) ?? (g.description ? <LongText text={g.description} title={`Description of ${g.title}`} /> : <span className="a-muted">-</span>) },
		{ key: 'exposed', label: '', render: (g) => g.exposed && <span className="a-public" title="Exposed through MCP: readable without signing in"><Globe size={13} />public</span> },
		{ key: 'created_at', label: 'First seen', sort: true, render: (g) => <span title={g.created_at}>{ago(g.created_at, v)}</span> },
		{ key: 'updated_at', label: 'Updated', sort: true, cls: 'a-hide-md', render: (g) => <span title={g.updated_at}>{ago(g.updated_at, v)}</span> },
	];
	return (
		<Shell crumbs={[['Groups', null]]}>
			<PageHead title="Groups" sub={member ? `${L.total} groups granted to you` : `${fmt(page.props.archive_total ?? L.total)} groups and channels the logger has seen`} />
			<FilterBar L={L} fields={withMatches(FIELDS, L.fields, 'description')} placeholder="Title, @username or ID" name="Groups" quick={QUICK} />
			<EngineOptions p={page.props} noun={NOUN} facets={false} />
			<BackendNote b={page.props.backend} noun={NOUN} />
			<DataTable
				L={L} cols={cols} noun="groups" empty={emptyText(L, member, 'groups', 'No groups match these filters.')}
				href={(g) => `/groups/${g.id}`}
				mobile={(g) => ({
					lead: <Avatar id={g.id} name={g.title} photo={g.photo} size={36} square />,
					title: g.title,
					sub: (g.username ? '@' + g.username + ' . ' : '') + TYPE_LABEL[g.type],
					extra: g.exposed || hl(g) ? <>{g.exposed && <Pill tone="ok">public</Pill>}{hl(g)}</> : null,
					side: <><span className="num">{compact(g.msg_count)}</span><span className="a-muted">msgs</span></>,
				})}
			/>
		</Shell>
	);
}

export const groupsRoute: RouteDef<GroupsProps> = { Page: Groups, head: () => ({ title: 'Groups - tgloggerd', robots: 'noindex' }) };
