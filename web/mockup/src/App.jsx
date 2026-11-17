import { RouterProvider, useRouter, Link } from './router.jsx';
import { useTheme } from './shared.jsx';
import { featuredUserId } from './data.js';
import { Sun, Moon, Monitor } from 'lucide-react';
import ConsoleApp from './a/ConsoleApp.jsx';
import TelegramApp from './b/TelegramApp.jsx';
import DataApp from './c/DataApp.jsx';

const DIRS = [
	{
		key: 'a', name: 'A - Console', App: ConsoleApp,
		blurb: 'Linear/Vercel-style admin console. Collapsible sidebar, neutral greys with one indigo accent, dense sortable table with filter chips and a command bar. Profile is a two-column page with tabs.',
	},
	{
		key: 'b', name: 'B - Messenger', App: TelegramApp,
		blurb: 'Feels like Telegram itself. Blue accent, rounded cards, big avatars. Lists read like a contact list; the profile is a Telegram-style info sheet; history uses date pills like a chat.',
	},
	{
		key: 'c', name: 'C - Data desk', App: DataApp,
		blurb: 'Grafana-like data desk. Dark-first, monospace numbers, a typed query bar (premium:yes msgs:>100). Master-detail: the profile opens beside the table, so you can step through rows.',
	},
];

function Picker() {
	return (
		<div className="picker">
			<header>
				<h1>tgd web - design directions</h1>
				<p>Three directions for the users table and the user profile/history. All data is synthetic. Use the switcher at the bottom of every screen to jump between directions on the same page, and to flip light/dark.</p>
			</header>
			<div className="picker-grid">
				{DIRS.map((d) => (
					<div key={d.key} className={`picker-card pc-${d.key}`}>
						<div className="pc-swatch" />
						<h2>{d.name}</h2>
						<p>{d.blurb}</p>
						<div className="pc-links">
							<Link to={`/${d.key}/users`}>Users table</Link>
							<Link to={`/${d.key}/users/${featuredUserId}`}>Profile</Link>
							<Link to={`/${d.key}/users/${featuredUserId}/history`}>History</Link>
						</div>
					</div>
				))}
			</div>
		</div>
	);
}

function Switcher({ dir }) {
	const { path, search } = useRouter();
	const [mode, cycle] = useTheme();
	const rest = path.replace(/^\/[abc]/, '');
	const Icon = mode === 'light' ? Sun : mode === 'dark' ? Moon : Monitor;
	return (
		<div className="switcher" role="toolbar" aria-label="Mockup controls">
			<Link to="/" className="sw-home" title="All directions">Mockup</Link>
			{DIRS.map((d) => (
				<Link key={d.key} to={`/${d.key}${rest || '/users'}${search}`} className={d.key === dir ? 'on' : ''} title={d.name}>
					{d.key.toUpperCase()}
				</Link>
			))}
			<button type="button" onClick={cycle} title={`Theme: ${mode}`} aria-label={`Theme: ${mode}`}>
				<Icon size={15} />
			</button>
		</div>
	);
}

function Routes() {
	const { path } = useRouter();
	const m = path.match(/^\/([abc])(\/.*)?$/);
	if (!m) return <Picker />;
	const d = DIRS.find((x) => x.key === m[1]);
	const sub = m[2] || '/users';
	return (
		<div className={`dir dir-${d.key}`}>
			<d.App base={`/${d.key}`} sub={sub} />
			<Switcher dir={d.key} />
		</div>
	);
}

export default function App() {
	return (
		<RouterProvider>
			<Routes />
		</RouterProvider>
	);
}
