// How a service line reads: most of the daemon's texts
// (describe_service_message in src/tgloggerd/MessageExtract.cpp) are a
// verb phrase that follows the actor ("pinned a message"), but a few
// are a whole sentence that no one did ("the giveaway was completed"):
// those go without an actor and start with a capital.
const SENTENCE_TYPES: ReadonlySet<string> = new Set([
	'chat_upgrade_from',
	'chat_upgrade_to',
	'giveaway_completed',
	'refunded_upgraded_gift',
	'suggested_post_approval_failed',
	'suggested_post_paid',
	'suggested_post_refunded',
]);

// A rejected gift offer is a verb phrase, but the same type says when
// the offer ran out, and that text is a sentence.
const SENTENCE_TEXTS: ReadonlySet<string> = new Set(['the offer to buy a gift expired']);

export const isSentence = (type: string | null | undefined, text: string): boolean => (!!type && SENTENCE_TYPES.has(type)) || SENTENCE_TEXTS.has(text);

export const capitalise = (s: string): string => s.charAt(0).toUpperCase() + s.slice(1);
