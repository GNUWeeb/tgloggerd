import Shell from '../ui/Shell.jsx';
import { Btn } from '../ui/primitives.jsx';

export default function NotFound({ what = 'page' }) {
	return (
		<Shell crumbs={[['Not found', null]]}>
			<div className="a-notfound">
				<h1>404</h1>
				<p className="a-muted">This {what} is not in the archive.</p>
				<Btn to="/">Back to the dashboard</Btn>
			</div>
		</Shell>
	);
}

export function Pending({ title, crumbs }) {
	return (
		<Shell crumbs={crumbs || [[title, null]]}>
			<div className="a-notfound">
				<h1>{title}</h1>
				<p className="a-muted">This page is still being designed.</p>
			</div>
		</Shell>
	);
}
