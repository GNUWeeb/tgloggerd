import { NavProvider, useNav } from './nav.tsx';
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

function Current() {
	const { page } = useNav();
	const { Page } = routeOf(page);
	return <Page page={page} />;
}

export default function App({ page }: { page: PageData }) {
	routeOf(page); // unknown routes fail before rendering anything
	return (
		<NavProvider initial={page}>
			<Current />
		</NavProvider>
	);
}
