// The logging account: who sent a message the archive has no sender for.
// A signed-in viewer reads it as "You"; a signed-out visitor of a public
// chat is not that account, so sees its name.
import type { Viewer } from '../page.ts';

// LOCAL until webapi/openapi.yaml has Viewer.logger: then use
// S['Viewer']['logger'] and read v.logger directly.
export interface Logger {
	id: number;
	name: string;
	username: string | null;
	photo: string | null;
}
export type ViewerWithLogger = Viewer & { logger?: Logger | null };

export const loggerOf = (v: Viewer): Logger | null => (v as ViewerWithLogger).logger ?? null;

// The logging account's name at the start of a line.
export const loggerName = (v: Viewer): string => (v.signed_in ? 'You' : loggerOf(v)?.name ?? 'The archive account');
