// The route registry: route ids (shared with the Go service's route
// table) mapped to their page component and head. Pages arrive here as
// they are ported from the mockup.
import type { ComponentType } from 'react';
import Shell from '../ui/Shell.tsx';
import type { PageData } from '../page.ts';
import type { Head } from '../head.ts';
import { dashboardRoute } from './dashboard.tsx';
import { usersRoute } from './users.tsx';
import { userRoute } from './user.tsx';
import { groupsRoute } from './groups.tsx';
import { groupRoute } from './group.tsx';
import { filesRoute } from './files.tsx';
import { messagesRoute } from './messages.tsx';
import { messageRoute } from './message.tsx';
import { chatRoute } from './chat.tsx';
import { loginRoute } from './login.tsx';
import { emailVerifyRoute, passwordForgotRoute, passwordSetRoute } from './password.tsx';
import { passwordChangeRoute } from './passwordchange.tsx';
import { botsRoute } from './bots.tsx';
import { settingsRoute, preferencesRoute, sessionsRoute } from './settings.tsx';
import { accountsRoute } from './accounts.tsx';
import { federationsRoute } from './federations.tsx';
import { federationRoute } from './federation.tsx';
import { forwardingIndexRoute, t2dRoute, d2tRoute } from './forwarding.tsx';
import { joinChatRoute } from './joinchat.tsx';
import { mcpIndexRoute, mcpGroupsRoute, mcpTokensRoute } from './mcp.tsx';

export interface RouteDef<P = any> {
	Page: ComponentType<{ page: PageData<P> }>;
	head: (page: PageData<P>) => Head;
}

function NotFound({ page }: { page: PageData }) {
	const body = (
		<div className="a-notfound">
			<h1>404</h1>
			<p className="a-muted">This page is not in the archive.</p>
		</div>
	);
	// Signed-in viewers keep their menu; a signed-out visitor gets the bare page.
	return page.viewer.signed_in ? <Shell crumbs={[['Not found', null]]}>{body}</Shell> : <div className="app">{body}</div>;
}

export const routes: Record<string, RouteDef> = {
	dashboard: dashboardRoute,
	users: usersRoute,
	user: userRoute,
	user_history: userRoute,
	groups: groupsRoute,
	group: groupRoute,
	group_admins: groupRoute,
	group_history: groupRoute,
	group_members: groupRoute,
	files: filesRoute,
	group_messages: messagesRoute,
	private_messages: messagesRoute,
	message: messageRoute,
	group_chat: chatRoute,
	user_chat: chatRoute,
	login: loginRoute,
	password_forgot: passwordForgotRoute,
	password_set: passwordSetRoute,
	email_verify: emailVerifyRoute,
	password_change: passwordChangeRoute,
	settings: settingsRoute,
	settings_preferences: preferencesRoute,
	settings_sessions: sessionsRoute,
	accounts: accountsRoute,
	federations: federationsRoute,
	federation: federationRoute,
	platform_fwd: forwardingIndexRoute,
	fwd_telegram_discord: t2dRoute,
	fwd_discord_telegram: d2tRoute,
	bots: botsRoute,
	join_chat: joinChatRoute,
	mcp: mcpIndexRoute,
	mcp_groups: mcpGroupsRoute,
	mcp_tokens: mcpTokensRoute,
	not_found: { Page: NotFound, head: () => ({ title: 'Not found - tgloggerd', robots: 'noindex' }) },
};
