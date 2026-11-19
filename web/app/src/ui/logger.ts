// The logging account: who sent a message the archive has no sender for.
// A signed-in viewer reads it as "You"; a signed-out visitor of a public
// chat is not that account, so sees its name.
import type { Viewer } from '../page.ts';

export type Logger = NonNullable<Viewer['logger']>;

// The logging account's name at the start of a line.
export const loggerName = (v: Viewer): string => (v.signed_in ? 'You' : v.logger?.name ?? 'The archive account');
