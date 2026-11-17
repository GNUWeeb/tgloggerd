import { RouterProvider, useRouter } from './router.jsx';
import Users from './pages/Users.jsx';
import User from './pages/User.jsx';
import NotFound, { Pending } from './pages/NotFound.jsx';

// The same paths the production server answers, so each screen here maps
// one to one onto a server-rendered page.
const ROUTES = [
	[/^\/$/, () => <Pending title="Dashboard" />],
	[/^\/users$/, () => <Users />],
	[/^\/users\/(-?\d+)$/, (m) => <User id={m[1]} tab="user" />],
	[/^\/users\/(-?\d+)\/history$/, (m) => <User id={m[1]} tab="history" />],
	[/^\/(groups|files|group_messages|private_messages|platform-fwd|mcp-admin|settings|login|messages|users\/-?\d+\/chat)(\/.*)?$/, (m) => <Pending title={m[1]} />],
];

function Routes() {
	const { path } = useRouter();
	const p = path.length > 1 ? path.replace(/\/+$/, '') : path;
	for (const [re, render] of ROUTES) {
		const m = p.match(re);
		if (m) return render(m);
	}
	return <NotFound />;
}

export default function App() {
	return (
		<RouterProvider>
			<Routes />
		</RouterProvider>
	);
}
