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
import Accounts from './pages/Accounts.jsx';
import { useQuery } from './router.jsx';
import NotFound from './pages/NotFound.jsx';
import { ViewerProvider, useViewer, allowed, PersonaSwitcher } from './viewer.jsx';

// The same paths the production server answers, so each screen here maps
// one to one onto a server-rendered page.
const ROUTES = [
	[/^\/$/, 'dashboard', () => <Dashboard />],
	[/^\/users$/, 'users', () => <Users />],
	[/^\/users\/(-?\d+)$/, 'users', (m) => <User id={m[1]} tab="user" />],
	[/^\/users\/(-?\d+)\/history$/, 'users', (m) => <User id={m[1]} tab="history" />],
	[/^\/groups$/, 'groups', () => <Groups />],
	[/^\/groups\/(-?\d+)$/, 'groups', (m) => <Group id={m[1]} tab="group" />],
	[/^\/groups\/(-?\d+)\/admins$/, 'groups', (m) => <Group id={m[1]} tab="admins" />],
	[/^\/groups\/(-?\d+)\/history$/, 'groups', (m) => <Group id={m[1]} tab="history" />],
	[/^\/files$/, 'files', () => <Files />],
	[/^\/group_messages$/, 'group_messages', () => <Messages scope="group" key="group" />],
	[/^\/private_messages$/, 'private_messages', () => <Messages scope="private" key="private" />],
	[/^\/messages\/(group|private)$/, null, (m) => <Messages scope={m[1]} key={m[1]} />],
	[/^\/messages\/(group|private)\/(\d+)$/, null, (m) => <Message scope={m[1]} id={m[2]} />],
	[/^\/groups\/(-?\d+)\/chat$/, 'groups', (m, q) => <Chat kind="group" id={m[1]} key={m[1]} publicView={q.public === '1'} />],
	[/^\/users\/(-?\d+)\/chat$/, 'private_messages', (m) => <Chat kind="user" id={m[1]} key={m[1]} />],
	[/^\/platform-fwd$/, 'platform_fwd', () => <ForwardingIndex />],
	[/^\/platform-fwd\/telegram-discord$/, 'platform_fwd', () => <TelegramDiscord />],
	[/^\/platform-fwd\/discord-telegram$/, 'platform_fwd', () => <DiscordTelegram />],
	[/^\/mcp-admin$/, 'mcp', () => <McpIndex />],
	[/^\/mcp-admin\/groups$/, 'mcp', () => <McpGroups />],
	[/^\/mcp-admin\/tokens$/, 'mcp', () => <McpTokens />],
	[/^\/settings$/, null, () => <Settings />],
	[/^\/settings\/accounts$/, 'accounts', () => <Accounts />],
	[/^\/login$/, null, () => <Login />],
];

function Routes() {
	const { path } = useRouter();
	const [q] = useQuery();
	const v = useViewer();
	const p = path.length > 1 ? path.replace(/\/+$/, '') : path;
	for (const [re, key, render] of ROUTES) {
		const m = p.match(re);
		if (!m) continue;
		// /messages/<scope>/... take their key from the scope.
		const k = key ?? (m[1] === 'private' ? 'private_messages' : m[1] === 'group' ? 'group_messages' : null);
		if (!allowed(v, k)) return <NotFound />;
		return render(m, q);
	}
	return <NotFound />;
}

export default function App() {
	return (
		<RouterProvider>
			<ViewerProvider>
				<Routes />
				<PersonaSwitcher />
			</ViewerProvider>
		</RouterProvider>
	);
}
