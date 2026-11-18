// One preference at a time (PATCH /api/account/preferences): the top
// bar's theme and sidebar buttons keep the account's choice, which the
// server then renders in every page. Signed out, the theme stays in
// this browser instead.
import { api, ok } from './client.ts';
import type { S } from './schema.ts';

type Prefs = S['Prefs'];
export type Theme = NonNullable<Prefs['theme']>;

export function savePrefs(patch: Partial<Prefs>): Promise<unknown> {
	return ok(api.PATCH('/api/account/preferences', { body: patch }));
}

const KEY = 'tgd-theme';
export function storedTheme(): Theme | null {
	try {
		const v = localStorage.getItem(KEY);
		return v === 'system' || v === 'light' || v === 'dark' ? v : null;
	} catch { return null; }
}
export function storeTheme(t: Theme): void {
	try { localStorage.setItem(KEY, t); } catch { /* private mode: this page only */ }
}

export function applyTheme(t: Theme): void {
	if (t === 'system') delete document.documentElement.dataset.theme;
	else document.documentElement.dataset.theme = t;
}
