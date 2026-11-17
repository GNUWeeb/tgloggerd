import type { ReactNode } from 'react';
import { NavProvider, useNav } from './nav.tsx';
import { LiveProvider } from './live/react.tsx';
import { PageLive } from './live/PageLive.tsx';
import { EntityProvider } from './live/useEntities.tsx';
import { routes } from './routes/index.tsx';
import { UnknownRoute, type PageData } from './page.ts';
import './styles/base.css';
import './styles/app.css';
import './styles/chat.css';

export function routeOf(page: PageData) {
	const r = routes[page.route];
	if (!r) throw new UnknownRoute(page.route);
	return r;
}

function LiveRoot({ children }: { children: ReactNode }) {
	const { viewer } = useNav();
	return <LiveProvider enabled={viewer.signed_in}><EntityProvider enabled={viewer.signed_in}><PageLive>{children}</PageLive></EntityProvider></LiveProvider>;
}

function Current() {
	const { page } = useNav();
	const { Page } = routeOf(page);
	return <Page page={page} />;
}

export default function App({ page }: { page: PageData }) {
	routeOf(page); // unknown routes fail before rendering anything
	return (
		<NavProvider initial={page}>
			<LiveRoot><Current /></LiveRoot>
		</NavProvider>
	);
}
