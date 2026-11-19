// Telegram's text entities (formatting, links, mentions) on message
// texts, edits, polls and checklist tasks.
//
// LOCAL TYPES: the API description does not have these fields yet.
// They follow the agreed contract and live here, in one place; once
// webapi/openapi.yaml (and so the generated schema) has them, use the
// generated ones and drop these.

// One entity: a range of the text in UTF-16 code units (JavaScript
// string indexes), with the type name the daemon stores (TDLib's
// textEntityType in snake_case, but block_quote, expandable_block_quote
// and email; unknown for a type it does not map). Types this view does
// not know are drawn as plain text.
export interface TextEntity {
	offset: number;
	length: number;
	type: string;
	// text_url: where the link goes.
	url?: string;
	// mention_name, and a mention (@username) the archive resolved: the
	// user, when the viewer may see them; null or left out otherwise.
	user_id?: number | null;
	// pre_code: the code's language.
	language?: string;
	// custom_emoji: the emoji's id (a string: it is a 64-bit number).
	custom_emoji_id?: string;
}

// The fields the contract adds; all optional, so an older service
// (without them) still draws plain text.
export interface HasEntities {
	entities?: TextEntity[] | null;
}
export interface PollEntities {
	question_entities?: TextEntity[] | null;
	explanation_entities?: TextEntity[] | null;
}

// A message's entities, or none.
export const entitiesOf = (x: unknown): TextEntity[] => ((x as HasEntities | null | undefined)?.entities ?? []);
