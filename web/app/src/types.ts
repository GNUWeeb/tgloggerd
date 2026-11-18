// Names for the API shapes the pages use, from the types generated from
// webapi/openapi.yaml (src/api/generated/). Only FieldType and FieldDef,
// which describe the list filters' UI, are the application's own.
import type { S } from './api/schema.ts';

export type UserSummary = S['UserSummary'];
export type GroupSummary = S['GroupSummary'];
export type GroupFull = S['GroupFull'];
export type GroupAdmin = S['GroupAdmin'];
export type GroupHistoryEntry = S['GroupHistoryEntry'];
export type HistoryEntry = S['HistoryEntry'];
export type FileRow = S['FileRow'];
export type ChatRef = S['ChatRef'];
export type MessageRow = S['MessageRow'];
export type MessageFull = S['MessageFull'];
export type ChatMedia = S['ChatMedia'];
export type ChatMessage = S['ChatMessage'];
export type ChatWindow = S['ChatWindow'];
// A chat as the pickers show it: a group or channel, or a user's private
// chat. `sub` is a short second line ("supergroup", "private chat").
export type PickItem = S['PickItem'];

export type Cond = S['Cond'];
export type Op = Cond['op'];
// A list page's props: its rows and the list state they were read with.
export type ListProps<T> = S['ListState'] & { rows: T[] };

export type FieldType = 'number' | 'text' | 'enum' | 'bool' | 'date';
// ops, when given, replaces the operators of the field's type. A field
// of the advanced search (ListState.fields) is marked search: its bools
// compare 1 and 0, its times are full date-times.
export interface FieldDef { key: string; label: string; type: FieldType; values?: string[]; ops?: Op[]; search?: boolean; sortable?: boolean }
export type SearchField = S['SearchField'];
