// Shapes of the page props the server sends. Hand-written until the
// OpenAPI description generates them (src/api/generated/).

export interface UserSummary {
	id: number;
	name: string;
	username: string;
	type: 'regular' | 'bot' | 'deleted' | 'unknown';
	photo: string | null;
	contact_name?: string;
	// Full-access viewers only; absent for members.
	msg_count?: number;
	is_verified?: boolean;
	is_premium?: boolean;
	is_scam?: boolean;
	is_fake?: boolean;
	is_support?: boolean;
	language?: string;
	created_at?: string;
	updated_at?: string;
}

export interface GroupSummary {
	id: number;
	title: string;
	username: string;
	type: 'supergroup' | 'basic_group' | 'channel';
	photo: string | null;
	msg_count?: number;
	exposed?: boolean;
}

export type Op = '=' | '!=' | '>' | '<' | 'contains' | 'empty' | 'is';
export interface Cond { field: string; op: Op; value: string }
export type FieldType = 'number' | 'text' | 'enum' | 'bool' | 'date';
export interface FieldDef { key: string; label: string; type: FieldType; values?: string[] }

export interface ListProps<T> {
	rows: T[];
	total: number;
	page: number;
	pages: number;
	per: number;
	sort: string;
	dir: 'asc' | 'desc';
	q: string;
	conds: Cond[];
}

export interface HistoryEntry {
	kind: 'name' | 'username' | 'bio' | 'phone' | 'photo';
	at: string;
	detail?: string;
	action?: 'added' | 'removed';
	photo?: string | null;
	removed?: boolean;
}
