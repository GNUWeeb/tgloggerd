// @vitest-environment happy-dom
// The MCP tokens page per role: who owns each token and who minted it,
// minting for another owner, renaming, and (superadmins) transferring.
import { beforeEach, describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
import { fixture, renderPage, setup } from '../test/page.tsx';
import type { McpTokensProps } from './mcp.tsx';

beforeEach(setup);
// The table row of a token, by its name.
const rowOf = (name: string) => screen.getByText(name).closest('tr') as HTMLElement;
const names = () => [...document.querySelectorAll('.a-table tbody tr td:first-child')].map((x) => x.textContent);

describe('mcp token owners', () => {
	it('shows each owner as a person, shared tokens as Shared, and who minted one for someone else', () => {
		renderPage('/mcp-admin/tokens');
		expect(within(rowOf('laptop')).getByText('Administrator')).toBeTruthy();
		expect(within(rowOf('old ci job')).getByText('Shared')).toBeTruthy();
		const r = rowOf('research-agent');
		expect(within(r).getByText('Research team')).toBeTruthy();
		expect(within(r).getByText(/minted by GNU\/Weeb admins/)).toBeTruthy();
		// Minted by its owner: no second name.
		expect(within(rowOf('laptop')).queryByText(/minted by/)).toBeNull();
	});

	it('lists every token to a superadmin, newest first', () => {
		const p = fixture<McpTokensProps>('/mcp-admin/tokens').props;
		expect(p.can_transfer).toBe(true);
		renderPage('/mcp-admin/tokens');
		expect(names()).toEqual(['(unnamed)', 'old ci job', 'research-agent', 'laptop']);
	});

	it('lists an admin its own tokens and its members\' only', () => {
		const p = fixture<McpTokensProps>('/mcp-admin/tokens', 'admin').props;
		expect(p.can_transfer).toBe(false);
		expect(p.owners.map((o) => o.name)).toEqual(['GNU/Weeb admins']);
		renderPage('/mcp-admin/tokens', 'admin');
		expect(names()).toEqual(['(unnamed)', 'research-agent']);
	});
});
