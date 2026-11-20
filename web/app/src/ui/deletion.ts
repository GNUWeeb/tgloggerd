// How a deletion reads, in the chat, on the message page, in the
// message lists and on the dashboard. A deletion found on recheck was
// not seen as it happened: the daemon rechecked a stretch of the chat
// and Telegram no longer had the message, so its time is when it was
// found, not when it was deleted. Live deletions keep their old words.
type Deleted = { deleted_on_recheck?: boolean };

const RECHECK = 'deleted (found on recheck)';

export const onRecheck = (d: Deleted): boolean => d.deleted_on_recheck === true;

// The short mark: "deleted", or "deleted (found on recheck)".
export const deletedTag = (d: Deleted): string => onRecheck(d) ? RECHECK : 'deleted';

// The same as a label or tooltip of its own.
export const deletedLabel = (d: Deleted): string => onRecheck(d) ? 'Deleted (found on recheck)' : 'Deleted';

// The mark's tooltip, with the time as given.
export const deletedTitle = (d: Deleted, at: string): string => onRecheck(d) ? `Found deleted on recheck at ${at}` : `Deleted ${at}`;

// The name of the time's field on the message page.
export const deletedField = (d: Deleted): string => onRecheck(d) ? 'Found deleted' : 'Deleted';

// The message page's banner.
export const deletedBanner = (d: Deleted, at: string): string => onRecheck(d)
	? `Found deleted on Telegram by a recheck at ${at}; it was deleted some time before. The archive keeps the last version it saw.`
	: `Deleted on Telegram at ${at}. The archive keeps the last version it saw.`;
