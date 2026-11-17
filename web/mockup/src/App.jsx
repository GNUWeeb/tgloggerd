import { RouterProvider, useRouter } from './router.jsx';
import Users from './pages/Users.jsx';
import User from './pages/User.jsx';
import Groups from './pages/Groups.jsx';
import Group from './pages/Group.jsx';
import Files from './pages/Files.jsx';
import Messages from './pages/Messages.jsx';
import Message from './pages/Message.jsx';
import NotFound, { Pending } from './pages/NotFound.jsx';

// The same paths the production server answers, so each screen here maps
// one to one onto a server-rendered page.
const ROUTES = [
	[/^\/$/, () => <Pending title="Dashboard" />],
	[/^\/users$/, () => <Users />],
	[/^\/users\/(-?\d+)$/, (m) => <User id={m[1]} tab="user" />],
	[/^\/users\/(-?\d+)\/history$/, (m) => <User id={m[1]} tab="history" />],
	[/^\/groups$/, () => <Groups />],
	[/^\/groups\/(-?\d+)$/, (m) => <Group id={m[1]} tab="group" />],
	[/^\/groups\/(-?\d+)\/admins$/, (m) => <Group id={m[1]} tab="admins" />],
	[/^\/groups\/(-?\d+)\/history$/, (m) => <Group id={m[1]} tab="history" />],
	[/^\/files$/, () => <Files />],
	[/^\/group_messages$/, () => <Messages scope="group" key="group" />],
	[/^\/private_messages$/, () => <Messages scope="private" key="private" />],
	[/^\/messages\/(group|private)$/, (m) => <Messages scope={m[1]} key={m[1]} />],
	[/^\/messages\/(group|private)\/(\d+)$/, (m) => <Message scope={m[1]} id={m[2]} />],
	[/^\/(groups\/-?\d+\/chat|platform-fwd|mcp-admin|settings|login|users\/-?\d+\/chat)(\/.*)?$/, (m) => <Pending title={m[1]} />],
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
