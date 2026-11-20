// Which service texts are a whole sentence, so go without an actor.
import { describe, expect, it } from 'vitest';
import { capitalise, isSentence } from './serviceline.ts';

describe('service lines', () => {
	it('take the sentence types without an actor', () => {
		expect(isSentence('chat_upgrade_from', 'the supergroup was created from a group')).toBe(true);
		expect(isSentence('suggested_post_paid', '')).toBe(true);
	});

	it('keep the actor before a verb phrase', () => {
		expect(isSentence('pin_message', 'pinned a message')).toBe(false);
		expect(isSentence('suggested_post_approved', 'approved a suggested post')).toBe(false);
		expect(isSentence(undefined, '')).toBe(false);
	});

	it('tell an expired gift offer from a rejected one by its text', () => {
		expect(isSentence('upgraded_gift_purchase_offer_rejected', 'the offer to buy a gift expired')).toBe(true);
		expect(isSentence('upgraded_gift_purchase_offer_rejected', 'rejected an offer to buy a gift')).toBe(false);
	});

	it('capitalise the first letter only', () => {
		expect(capitalise('the giveaway was completed')).toBe('The giveaway was completed');
		expect(capitalise('')).toBe('');
	});
});
