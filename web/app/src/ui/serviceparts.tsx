// A service message's text with the people and chats it names linked:
// "added Dewi Kusuma and 2 other members" links Dewi Kusuma to her user
// page. The API sends the text as parts, each name pointing at its
// entry in the action's refs, so two people of the same name or a name
// that contains the words around it still link right.
import type { ReactNode } from 'react';
import { Link } from '../nav.tsx';
import { capitalise } from './serviceline.ts';
import type { S } from '../api/schema.ts';

// user / chat is the index of the name in refs.users / refs.chats.
export type ServicePart = S['ServicePart'];

// The parts of a service text, or null to show the text as it is: none
// were sent (an older service, a type that names nobody), or they do
// not spell the text.
export function partsOf(service: S['ServiceInfo'] | null | undefined, text: string): ServicePart[] | null {
	const parts = service?.parts;
	if (!parts?.length || parts.map((p) => p.text).join('') !== text) return null;
	return parts;
}

// The text, its names as links. `plain` wraps the text between them
// (the chat links it to the message the action is about); `capital`
// starts it with a capital, for a line that is a whole sentence.
export function ServiceText({ service, text, plain = (s) => s, capital = false, linkClass }: { service: S['ServiceInfo'] | null | undefined; text: string; plain?: (s: string) => ReactNode; capital?: boolean; linkClass?: string }) {
	const up = (s: string, i: number) => (capital && i === 0 ? capitalise(s) : s);
	const parts = partsOf(service, text);
	if (!parts) return <>{plain(up(text, 0))}</>;
	const refs = service!.refs;
	return (
		<>
			{parts.map((p, i) => {
				const s = up(p.text, i);
				const user = p.user !== undefined ? refs.users[p.user] : undefined;
				const chat = p.chat !== undefined ? refs.chats[p.chat] : undefined;
				if (user) return <Link key={i} to={`/users/${user.id}`} className={linkClass}>{s}</Link>;
				if (chat) return <Link key={i} to={`/groups/${chat.id}`} className={linkClass}>{s}</Link>;
				return <span key={i}>{plain(s)}</span>;
			})}
		</>
	);
}
