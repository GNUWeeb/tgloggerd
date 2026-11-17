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
	not_found: { Page: NotFound, head: () => ({ title: 'Not found - tgloggerd', robots: 'noindex' }) },
};
