// The role a file plays in its message on /files: a video's cover, a
// link preview's picture, a game's photo and so on (the archive's
// telegram_message_files, docs/design/message-types.md section 4.3, and
// a link preview's picture), drawn as a small tag, and the list's role
// filter.
import { Link } from '../nav.tsx';
import { CONTENT_LABELS } from './labels.ts';
import type { S } from '../api/schema.ts';

// FileRow.role: the role and the kind of the message the row names;
// null (or absent) for a message's own file.
export type FileRole = S['FileRoleRef'] | null;
export const roleOf = (f: { role?: FileRole }): FileRole => f.role ?? null;

// Roles are not a closed list (the daemon adds them with new message
// kinds), so one missing here shows its raw name.
const ROLES: Readonly<Record<string, string>> = {
	cover: 'Cover',
	live_video: 'Live photo video',
	paid: 'Paid item',
	preview: 'Preview',
	game: 'Game',
	invoice: 'Invoice',
	chat_photo: 'Chat photo',
	thumbnail: 'Thumbnail',
	poll: 'Poll media',
	rich: 'Rich message media',
	gift: 'Gift',
	background: 'Chat background',
	link_preview: 'Link preview',
	preview_media: 'Link preview media',
};

export const roleLabel = (r: string): string => (Object.hasOwn(ROLES, r) ? ROLES[r]! : r);

// The role filter's values and their labels. preview is reserved in
// the archive (other thumbnails and video qualities) and nothing writes
// it yet, so the filter does not offer it.
export const ROLE_VALUES = Object.keys(ROLES).filter((r) => r !== 'preview');
export const ROLE_LABELS = ROLES;

const kindLabel = (k: string): string => (Object.hasOwn(CONTENT_LABELS, k) ? CONTENT_LABELS[k]! : k);

// The tag: the role, and the message's kind in its title.
export function RoleTag({ role }: { role: FileRole }) {
	if (!role) return null;
	const title = role.message_kind ? `${roleLabel(role.role)} of its message (${kindLabel(role.message_kind)})` : roleLabel(role.role);
	return <span className="a-pill a-pill-info a-roletag" title={title}>{roleLabel(role.role)}</span>;
}

// The preview's line for it: the tag, then the message it belongs to,
// linked when the viewer sees that message.
export function RoleDetail({ role, href }: { role: NonNullable<FileRole>; href: string | null }) {
	const of = role.message_kind ? `of its message (${kindLabel(role.message_kind)})` : 'of its message';
	return <span><RoleTag role={role} />{href ? <Link to={href} className="a-rowlink">{of}</Link> : <span className="a-muted">{of}</span>}</span>;
}
