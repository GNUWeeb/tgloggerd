// @vitest-environment happy-dom
// What a member sees of the archive: its granted groups only, and empty
// pages, not errors, while nothing is granted to it.
import { beforeEach, describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { fakeApi, renderPage, setup } from '../test/page.tsx';
import { accounts } from '../../../mockup/src/data/accounts.js';

beforeEach(setup);

describe('a member with no group granted', () => {
	beforeEach(() => {
		const m = accounts.find((a: { id: number }) => a.id === 5)!;
		m.grants = [{ fed: 1, groupIds: [] }];
	});

	for (const [path, noun] of [['/groups', 'groups'], ['/users', 'users'], ['/group_messages', 'messages'], ['/files', 'files']] as const) {
		it(`says why ${path} is empty`, () => {
			fakeApi('member');
			const { page } = renderPage(path, 'member');
			expect(page.status).toBe(200);
			expect(screen.getByText(`No ${noun} to show: no group is granted to you yet. An admin of your federation grants them.`)).toBeTruthy();
			expect(screen.queryByRole('alert')).toBeNull();
		});
	}

	it('blames the filters when a search finds nothing', () => {
		fakeApi('member');
		renderPage('/groups?q=nothing-like-this', 'member');
		expect(screen.getByText('No groups match these filters.')).toBeTruthy();
	});
});
