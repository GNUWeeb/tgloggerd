// @vitest-environment happy-dom
// The MCP tokens page per role: who owns each token and who minted it,
// minting for another owner, renaming, and (superadmins) transferring.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import type { McpTokensProps } from './mcp.tsx';
import { adminApi } from '../../standin/fixtures.ts';

beforeEach(setup);
// The table row of a token, by its name.
const rowOf = (name: string) => screen.getByText(name).closest('tr') as HTMLElement;
const dialog = () => screen.getByRole('dialog');
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

describe('minting an mcp token', () => {
	const mint = async (persona: 'superadmin' | 'admin', owner?: string) => {
		const calls = fakeApi(persona, { 'POST /api/mcp/tokens': () => json(201, { id: 9, token: 'tgmcp_test_once' }) });
		renderPage('/mcp-admin/tokens', persona);
		fireEvent.click(screen.getByRole('button', { name: /Mint a token/ }));
		fireEvent.change(dialog().querySelector('input')!, { target: { value: 'agent' } });
		if (owner) fireEvent.change(within(dialog()).getByLabelText(/Owner/), { target: { value: owner } });
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Mint' }));
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/mcp/tokens')).toHaveLength(1));
		return callsTo(calls, 'POST', '/api/mcp/tokens')[0]!.body;
	};

	it('mints for the viewer unless another owner is picked', async () => {
		expect(await mint('superadmin')).toEqual({ name: 'agent' });
	});

	it('sends the owner picked from the owners offered', async () => {
		const p = fixture<McpTokensProps>('/mcp-admin/tokens').props;
		const lead = p.owners.find((o) => o.name === 'Security lead')!;
		expect(await mint('superadmin', String(lead.id))).toEqual({ name: 'agent', owner_id: lead.id });
	});

	it('offers no owner pick to a viewer who may mint only for itself', () => {
		expect(fixture<McpTokensProps>('/mcp-admin/tokens', 'admin').props.owners).toHaveLength(1);
		renderPage('/mcp-admin/tokens', 'admin');
		fireEvent.click(screen.getByRole('button', { name: /Mint a token/ }));
		expect(within(dialog()).queryByLabelText(/Owner/)).toBeNull();
	});

	it('is refused in the stand-in for an owner the viewer may not mint for, or without a name', () => {
		// An admin manages no other admin.
		expect(adminApi('POST', 'mcp/tokens', { name: 'x', owner_id: 4 }, 'admin')[0]).toBe(400);
		expect(adminApi('POST', 'mcp/tokens', { name: '   ' }, 'superadmin')[0]).toBe(400);
		const [st] = adminApi('POST', 'mcp/tokens', { name: 'for the lead', owner_id: 4 }, 'superadmin');
		expect(st).toBe(201);
		const t = fixture<McpTokensProps>('/mcp-admin/tokens').props.tokens.find((x) => x.name === 'for the lead')!;
		expect([t.owner.name, t.minted_by.name]).toEqual(['Security lead', 'Administrator']);
	});
});
