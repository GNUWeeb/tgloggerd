import type { PageData } from './page.ts';
import { routes } from './routes/index.tsx';
import { UnknownRoute } from './page.ts';

export function routeOf(page: PageData) {
	const r = routes[page.route];
	if (!r) throw new UnknownRoute(page.route);
	return r;
}

export default function App({ page }: { page: PageData }) {
	const { Page } = routeOf(page);
	return <Page page={page} />;
}
