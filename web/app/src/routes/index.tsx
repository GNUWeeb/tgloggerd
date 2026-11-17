// The route registry: route ids (shared with the Go service's route
// table) mapped to their page component and head. Pages arrive here as
// they are ported from the mockup.
import type { ComponentType } from 'react';
import type { PageData } from '../page.ts';
import type { Head } from '../head.ts';

export interface RouteDef<P = any> {
	Page: ComponentType<{ page: PageData<P> }>;
	head: (page: PageData<P>) => Head;
}

function NotFound() {
	return (
		<main className="a-notfound">
			<h1>404</h1>
			<p>This page is not in the archive.</p>
		</main>
	);
}

export const routes: Record<string, RouteDef> = {
	not_found: { Page: NotFound, head: () => ({ title: 'Not found - tgloggerd', robots: 'noindex' }) },
};
