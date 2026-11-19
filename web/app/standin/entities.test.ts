// The stand-in's text entities, as the text-entities contract has the
// Go service send them: every type on the samples, offsets in UTF-16
// units, mentions resolved only to users the viewer may see, and []
// on plain texts and on service messages.
import { describe, expect, it } from 'vitest';
import { pageData, type Persona } from './fixtures.ts';
import { KINDS_GROUP } from './kinds.ts';

type E = { offset: number; length: number; type: string; user_id?: number | null; url?: string };
type M = { text: string; content_type: string; entities: E[]; edits: { text: string; entities: E[] }[]; detail: { kind: string; question?: string; question_entities?: E[]; explanation_entities?: E[]; tasks?: { entities: E[] }[] } | null };
const chat = (p: Persona) => (pageData(`/groups/${KINDS_GROUP.id}/chat`, p).props as { messages: M[] }).messages;
const formatted = (p: Persona) => chat(p).find((m) => m.text.startsWith('Release notes:'))!;
const part = (m: { text: string }, e: E) => m.text.slice(e.offset, e.offset + e.length);

describe('stand-in text entities', () => {
	it('gives the samples every entity type', () => {
		const types = new Set(chat('superadmin').flatMap((m) => m.entities.map((e) => e.type)));
		for (const t of ['url', 'text_url', 'mention', 'mention_name', 'hashtag', 'cashtag', 'bot_command', 'email', 'phone_number', 'bank_card_number', 'bold', 'italic', 'underline', 'strikethrough', 'spoiler', 'code', 'pre', 'pre_code', 'block_quote', 'expandable_block_quote', 'custom_emoji', 'media_timestamp']) expect(types.has(t), t).toBe(true);
	});

	it('counts offsets in UTF-16 units, past emoji', () => {
		const m = chat('superadmin').find((x) => x.entities.some((e) => e.type === 'pre_code'))!;
		expect(part(m, m.entities.find((e) => e.type === 'bold')!)).toBe('Family');
		expect(part(m, m.entities.find((e) => e.type === 'italic')!)).toBe('𝕏');
	});

	it('resolves a mention only to a user the viewer may see', () => {
		const full = formatted('superadmin').entities.filter((e) => e.type === 'mention');
		const member = formatted('member').entities.filter((e) => e.type === 'mention');
		// A poster of the group, a user in no group, and a handle no one has.
		expect(full.map((e) => typeof e.user_id)).toEqual(['number', 'number', 'undefined']);
		expect(member.map((e) => typeof e.user_id)).toEqual(['number', 'undefined', 'undefined']);
		const names = formatted('member').entities.filter((e) => e.type === 'mention_name');
		expect(typeof names[0]!.user_id).toBe('number');
		expect(names[1]!.user_id).toBeNull();
	});

	it('has an entity past the text end, an unknown type and an unsafe link', () => {
		const m = formatted('superadmin');
		expect(m.entities.some((e) => e.offset + e.length > m.text.length)).toBe(true);
		expect(m.entities.some((e) => e.type === 'unknown')).toBe(true);
		expect(m.entities.some((e) => e.url?.startsWith('javascript:'))).toBe(true);
	});

	it('gives an edit, a poll and checklist tasks their entities', () => {
		const msgs = chat('superadmin');
		const edited = msgs.find((m) => m.edits.some((e) => e.entities.length))!;
		expect(part(edited.edits[0]!, edited.edits[0]!.entities[1]!)).toBe('#meetup');
		const quiz = msgs.find((m) => m.detail?.question_entities?.length)!.detail!;
		expect(part({ text: quiz.question! }, quiz.question_entities![1]!)).toBe('Linux kernel');
		expect(quiz.explanation_entities!.length).toBe(2);
		const list = msgs.find((m) => m.detail?.kind === 'checklist')!.detail!;
		expect(list.tasks!.map((t) => t.entities.length)).toEqual([0, 0, 1, 1]);
	});

	it('sends [] for plain texts and service messages', () => {
		const msgs = chat('superadmin');
		expect(msgs.filter((m) => m.content_type === 'service').every((m) => Array.isArray(m.entities) && m.entities.length === 0)).toBe(true);
		const other = (pageData('/groups/' + (pageData('/groups', 'superadmin').props as { rows: { id: number }[] }).rows[0]!.id + '/chat', 'superadmin').props as { messages: M[] }).messages;
		expect(other.every((m) => Array.isArray(m.entities) && m.edits.every((e) => Array.isArray(e.entities)))).toBe(true);
	});
});
