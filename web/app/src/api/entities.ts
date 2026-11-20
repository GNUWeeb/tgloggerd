// Telegram's text entities (formatting, links, mentions) on message
// texts, edits, polls and checklist tasks, as the service sends them:
// UTF-16 offsets (JavaScript string indexes), with the type name the
// daemon stores (TDLib's textEntityType in snake_case, but
// block_quote, expandable_block_quote and email; unknown for a type it
// does not map). Types the renderer does not know are drawn as plain
// text.
import type { S } from './schema.ts';

export type TextEntity = S['TextEntity'];
