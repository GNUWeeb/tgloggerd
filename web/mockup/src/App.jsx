import { RouterProvider, useRouter } from './router.jsx';
import Users from './pages/Users.jsx';
import User from './pages/User.jsx';
import Groups from './pages/Groups.jsx';
import Group from './pages/Group.jsx';
import Files from './pages/Files.jsx';
import Messages from './pages/Messages.jsx';
import Message from './pages/Message.jsx';
import Chat from './pages/Chat.jsx';
import Dashboard from './pages/Dashboard.jsx';
import { ForwardingIndex, TelegramDiscord, DiscordTelegram } from './pages/Forwarding.jsx';
import { McpIndex, McpGroups, McpTokens } from './pages/Mcp.jsx';
import { Settings, Login } from './pages/Account.jsx';
import { useQuery } from './router.jsx';
import NotFound from './pages/NotFound.jsx';

// The same paths the production server answers, so each screen here maps
// one to one onto a server-rendered page.
const ROUTES = [
	[/^\/$/, () => <Dashboard />],
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
	[/^\/groups\/(-?\d+)\/chat$/, (m, q) => <Chat kind="group" id={m[1]} key={m[1]} publicView={q.public === '1'} />],
	[/^\/users\/(-?\d+)\/chat$/, (m) => <Chat kind="user" id={m[1]} key={m[1]} />],
	[/^\/platform-fwd$/, () => <ForwardingIndex />],
	[/^\/platform-fwd\/telegram-discord$/, () => <TelegramDiscord />],
	[/^\/platform-fwd\/discord-telegram$/, () => <DiscordTelegram />],
	[/^\/mcp-admin$/, () => <McpIndex />],
	[/^\/mcp-admin\/groups$/, () => <McpGroups />],
	[/^\/mcp-admin\/tokens$/, () => <McpTokens />],
	[/^\/settings$/, () => <Settings />],
	[/^\/login$/, () => <Login />],
];

function Routes() {
	const { path } = useRouter();
	const [q] = useQuery();
	const p = path.length > 1 ? path.replace(/\/+$/, '') : path;
	for (const [re, render] of ROUTES) {
		const m = p.match(re);
		if (m) return render(m, q);
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
