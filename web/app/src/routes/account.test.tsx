// @vitest-environment happy-dom
// Signing in, the account's own settings, and the pages email links
// lead to.
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import type { PreferencesProps, SessionsProps } from './settings.tsx';

beforeEach(setup);
const fill = (name: string, value: string, root: HTMLElement = document.body) =>
	fireEvent.change(root.querySelector(`input[name="${name}"]`)!, { target: { value } });
const stubAssign = () => vi.spyOn(location, 'assign').mockImplementation(() => {});

describe('sign-in', () => {
	it('signs in with the pre-login token and goes to `next`', async () => {
		const go = stubAssign();
		const calls = fakeApi('public', {
			'GET /api/login': () => ({ signed_in: false, csrf: 'pre-1' }),
			'POST /api/login': () => ({ user: { id: 1 }, csrf: 'sess' }),
		});
		renderPage('/login?next=%2Fgroups', 'public');
		await waitFor(() => expect((screen.getByRole('button', { name: /Sign in/ }) as HTMLButtonElement).disabled).toBe(false));
		fill('username', 'admin'); fill('password', 'standin');
		fireEvent.submit(document.querySelector('form')!);
		await waitFor(() => expect(go).toHaveBeenCalledWith('/groups'));
		const post = callsTo(calls, 'POST', '/api/login')[0]!;
		expect(post.headers.get('x-csrf-token')).toBe('pre-1');
		expect(post.body).toEqual({ username: 'admin', password: 'standin' });
	});

	it('shows the error and retries with the fresh token the error carries', async () => {
		const calls = fakeApi('public', {
			'GET /api/login': () => ({ signed_in: false, csrf: 'pre-1' }),
			'POST /api/login': () => json(401, { error: 'invalid_credentials', message: 'Wrong username or password.', csrf: 'pre-2' }),
		});
		renderPage('/login', 'public');
		await waitFor(() => expect((screen.getByRole('button', { name: /Sign in/ }) as HTMLButtonElement).disabled).toBe(false));
		fill('username', 'admin'); fill('password', 'nope');
		fireEvent.submit(document.querySelector('form')!);
		await screen.findByText('Wrong username or password.');
		fireEvent.submit(document.querySelector('form')!);
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/login')).toHaveLength(2));
		expect(callsTo(calls, 'POST', '/api/login')[1]!.headers.get('x-csrf-token')).toBe('pre-2');
	});

	it('goes to the account\'s landing page when no page was asked for', async () => {
		const go = stubAssign();
		fakeApi('public', {
			'GET /api/login': () => ({ signed_in: false, csrf: 'pre-1' }),
			'POST /api/login': () => ({ user: { id: 1, username: 'admin', role: 'superadmin', must_change_password: false }, csrf: 'sess', landing: '/groups' }),
		});
		renderPage('/login', 'public');
		await waitFor(() => expect((screen.getByRole('button', { name: /Sign in/ }) as HTMLButtonElement).disabled).toBe(false));
		fill('username', 'admin'); fill('password', 'standin');
		fireEvent.submit(document.querySelector('form')!);
		await waitFor(() => expect(go).toHaveBeenCalledWith('/groups'));
	});

	it('never follows an off-site landing page either', async () => {
		const go = stubAssign();
		fakeApi('public', { 'GET /api/login': () => ({ signed_in: true }), 'GET /api/session': () => ({ user: { id: 1, username: 'a', role: 'superadmin', must_change_password: false }, csrf: 's', landing: '//evil.example' }) });
		renderPage('/login', 'public');
		await waitFor(() => expect(go).toHaveBeenCalledWith('/'));
	});

	it('never follows an off-site `next`', async () => {
		const go = stubAssign();
		fakeApi('public', { 'GET /api/login': () => ({ signed_in: true }) });
		renderPage('/login?next=%2F%2Fevil.example', 'public');
		await waitFor(() => expect(go).toHaveBeenCalledWith('/'));
	});
});

describe('settings', () => {
	it('checks the new passwords match before asking the server', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/settings');
		fill('current_password', 'standin'); fill('new_password', 'longenough1'); fill('confirm_password', 'longenough2');
		fireEvent.submit(document.querySelector('input[name="new_password"]')!.closest('form')!);
		await screen.findByText('The new passwords do not match.');
		expect(callsTo(calls, 'POST', '/api/account/password')).toHaveLength(0);
	});

	it('takes the new CSRF token a password change returns for the next change', async () => {
		const calls = fakeApi('superadmin', {
			'POST /api/account/password': () => ({ csrf: 'rotated' }),
			'POST /api/account/email': () => json(202, { pending: 'x@example.org' }),
		});
		renderPage('/settings');
		fill('current_password', 'standin'); fill('new_password', 'longenough1'); fill('confirm_password', 'longenough1');
		fireEvent.submit(document.querySelector('input[name="new_password"]')!.closest('form')!);
		await screen.findByText(/Password changed/);
		expect(callsTo(calls, 'POST', '/api/account/password')[0]!.headers.get('x-csrf-token')).toBe('test-csrf');
		fireEvent.click(screen.getByRole('button', { name: /^(Change|Add)$/ }));
		const dialog = screen.getByRole('dialog');
		fill('email', 'x@example.org', dialog); fill('password', 'longenough1', dialog);
		fireEvent.submit(dialog.querySelector('form')!);
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/account/email')).toHaveLength(1));
		expect(callsTo(calls, 'POST', '/api/account/email')[0]!.headers.get('x-csrf-token')).toBe('rotated');
	});

	it('says the address changed at once when no verification link was sent', async () => {
		fakeApi('superadmin', { 'POST /api/account/email': () => ({ verification_sent: false }) });
		renderPage('/settings');
		fireEvent.click(screen.getByRole('button', { name: /^(Change|Add)$/ }));
		const dialog = screen.getByRole('dialog');
		fill('email', 'new@example.org', dialog); fill('password', 'standin', dialog);
		fireEvent.submit(dialog.querySelector('form')!);
		expect((await screen.findByRole('status')).textContent).toMatch(/now new@example.org. No verification link was sent/);
	});

	it('saves preferences, with the site default as null', async () => {
		const calls = fakeApi('superadmin', { 'PUT /api/account/preferences': (c) => ({ prefs: c.body }) });
		renderPage('/settings/preferences');
		const theme = screen.getAllByRole('combobox').find((s) => [...(s as HTMLSelectElement).options].some((o) => o.value === 'dark'))!;
		fireEvent.change(theme, { target: { value: 'dark' } });
		fireEvent.click(screen.getByRole('button', { name: 'Save preferences' }));
		await waitFor(() => expect(callsTo(calls, 'PUT', '/api/account/preferences')).toHaveLength(1));
		const body = callsTo(calls, 'PUT', '/api/account/preferences')[0]!.body as Record<string, unknown>;
		expect(body.theme).toBe('dark');
	});

	it('unpins a group and deletes a saved search from Preferences', async () => {
		const p = fixture<PreferencesProps>('/settings/preferences').props;
		const calls = fakeApi('superadmin', { 'DELETE /api/account/pinned/:id': () => undefined, 'DELETE /api/account/saved-searches/:id': () => undefined });
		renderPage('/settings/preferences');
		fireEvent.click(screen.getByRole('button', { name: `Unpin ${p.pinned[0]!.title}` }));
		fireEvent.click(screen.getByRole('button', { name: `Delete ${p.saved_searches[0]!.title}` }));
		await waitFor(() => expect(callsTo(calls, 'DELETE', `/api/account/pinned/${p.pinned[0]!.id}`)).toHaveLength(1));
		await waitFor(() => expect(callsTo(calls, 'DELETE', `/api/account/saved-searches/${p.saved_searches[0]!.id}`)).toHaveLength(1));
	});

	it('signs out one other session, or all others', async () => {
		const p = fixture<SessionsProps>('/settings/sessions').props;
		const other = p.sessions.find((s) => !s.current)!;
		const calls = fakeApi('superadmin', { 'DELETE /api/account/sessions/:id': () => undefined, 'POST /api/account/sessions/sign-out-others': () => ({ csrf: 'reissued' }) });
		renderPage('/settings/sessions');
		const row = screen.getByText(other.device).closest('li')!;
		fireEvent.click(within(row).getByRole('button', { name: 'Sign out' }));
		fireEvent.click(screen.getByRole('button', { name: /Sign out all others/ }));
		await waitFor(() => expect(callsTo(calls, 'DELETE', `/api/account/sessions/${other.id}`)).toHaveLength(1));
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/account/sessions/sign-out-others')).toHaveLength(1));
		// This session's cookie is reissued: later changes carry the new token.
		await waitFor(() => expect(document.querySelector<HTMLMetaElement>('meta[name="csrf-token"]')!.content).toBe('reissued'));
	});
});

describe('email link pages', () => {
	it('asks for a reset link with the pre-login token, and says the same either way', async () => {
		const calls = fakeApi('public', { 'GET /api/login': () => ({ signed_in: false, csrf: 'pre-1' }), 'POST /api/password/forgot': () => json(202) });
		renderPage('/password/forgot', 'public');
		await waitFor(() => expect((screen.getByRole('button', { name: /Send the link/ }) as HTMLButtonElement).disabled).toBe(false));
		fill('username', 'researcher');
		fireEvent.submit(document.querySelector('form')!);
		await screen.findByText(/If that account has a verified email address/);
		const c = callsTo(calls, 'POST', '/api/password/forgot')[0]!;
		expect(c.headers.get('x-csrf-token')).toBe('pre-1');
		expect(c.body).toEqual({ username: 'researcher' });
	});

	it('reads the set-password token from the fragment once, removes it, and sends it with the new password', async () => {
		const calls = fakeApi('public', { 'POST /api/password/set': () => undefined });
		history.replaceState(null, '', '/password/set#token=tok-123');
		renderPage('/password/set#token=tok-123', 'public');
		await waitFor(() => expect(location.hash).toBe(''));
		fill('new_password', 'a long passphrase'); fill('confirm_password', 'a long passphrase');
		fireEvent.submit(document.querySelector('form')!);
		await screen.findByText('Your password is set.');
		expect(callsTo(calls, 'POST', '/api/password/set')[0]!.body).toEqual({ token: 'tok-123', new_password: 'a long passphrase' });
	});

	it('explains a used or expired link', async () => {
		fakeApi('public', { 'POST /api/password/set': () => json(400, { error: 'bad_link', message: 'gone' }) });
		renderPage('/password/set#token=old', 'public');
		await screen.findByText('New password');
		fill('new_password', 'a long passphrase'); fill('confirm_password', 'a long passphrase');
		fireEvent.submit(document.querySelector('form')!);
		await screen.findByText(/This link is not valid any more/);
	});

	it('says a link without a token is incomplete, and shows no form', async () => {
		renderPage('/password/set', 'public');
		await screen.findByText(/This link is incomplete/);
		expect(document.querySelector('input[name="new_password"]')).toBeNull();
	});

	it('verifies an address only when asked', async () => {
		const calls = fakeApi('public', { 'POST /api/account/email/verify': () => undefined });
		renderPage('/account/email/verify#token=v-9', 'public');
		const btn = await screen.findByRole('button', { name: /Verify this address/ });
		expect(callsTo(calls, 'POST', '/api/account/email/verify')).toHaveLength(0); // opening the link uses nothing up
		fireEvent.click(btn);
		await screen.findByText('Your email address is verified.');
		expect(callsTo(calls, 'POST', '/api/account/email/verify')[0]!.body).toEqual({ token: 'v-9' });
	});
});

describe('choose your own password', () => {
	it('comes up on any page for an account with a temporary password, and goes on once changed', async () => {
		expect(fixture('/groups', 'newpw').route).toBe('password_change');
		const calls = fakeApi('newpw', {
			'POST /api/account/password': () => ({ csrf: 'after-change' }),
			'GET /api/page/groups': () => fixture('/groups', 'member'),
		});
		renderPage('/groups', 'newpw');
		expect(screen.getByRole('heading', { level: 1 }).textContent).toBe('Choose your own password');
		fill('current_password', 'tmp-1'); fill('new_password', 'tmp-1'); fill('confirm_password', 'tmp-1');
		fireEvent.submit(document.querySelector('form')!);
		await screen.findByText('Choose a password of your own, not the temporary one.');
		fill('new_password', 'my own passphrase'); fill('confirm_password', 'my own passphrase');
		fireEvent.submit(document.querySelector('form')!);
		await screen.findByRole('heading', { name: 'Groups', level: 1 });
		expect(callsTo(calls, 'POST', '/api/account/password')[0]!.body).toEqual({ current_password: 'tmp-1', new_password: 'my own passphrase' });
		expect(document.querySelector<HTMLMetaElement>('meta[name="csrf-token"]')!.content).toBe('after-change');
	});

	it('says when the temporary password is wrong', async () => {
		fakeApi('newpw', { 'POST /api/account/password': () => json(400, { error: 'wrong_password', message: 'x' }) });
		renderPage('/', 'newpw');
		fill('current_password', 'nope'); fill('new_password', 'my own passphrase'); fill('confirm_password', 'my own passphrase');
		fireEvent.submit(document.querySelector('form')!);
		await screen.findByText('The temporary password is not right.');
	});

	it('comes up when any call answers password_change_required', async () => {
		const gate = fixture('/settings/sessions', 'newpw');
		fakeApi('superadmin', {
			'POST /api/account/sessions/sign-out-others': () => json(403, { error: 'password_change_required' }),
			'GET /api/page/settings/sessions': () => gate,
		});
		renderPage('/settings/sessions');
		fireEvent.click(screen.getByRole('button', { name: /Sign out all others/ }));
		await screen.findByRole('heading', { name: 'Choose your own password' });
	});
});
