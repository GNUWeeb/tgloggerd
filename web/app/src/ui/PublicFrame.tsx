// The frame of the pages a signed-out visitor can open (an exposed
// group's chat and its page): the site's name, the theme button and a
// way to sign in, with no menu.
import type { ReactNode } from 'react';
import { LogIn } from 'lucide-react';
import { ThemeButton } from './Shell.tsx';
import type { Viewer } from '../page.ts';

export function PublicFrame({ viewer, url, className = '', children }: { viewer: Viewer; url: string; className?: string; children: ReactNode }) {
	return (
		<div className={`app c-publicpage ${className}`}>
			<header className="c-pubtop">
				<span className="a-logo">tg</span><b>tgloggerd</b><span className="c-sub">public archive</span>
				<span className="a-grow" />
				<ThemeButton viewer={viewer} />
				<a href={`/login?next=${encodeURIComponent(url)}`} className="a-btn a-btn-ghost"><LogIn size={14} />Sign in</a>
			</header>
			{children}
		</div>
	);
}
