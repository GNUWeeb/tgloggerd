// @vitest-environment happy-dom
// A gift on its service line, in the chat and on its message page: the
// sticker with the chat's players, an upgraded gift's backdrop, symbol,
// title, link and rarities, the price, sender and receiver, the text,
// the original details; and a chat theme set from a gift.
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { within } from '@testing-library/react';
vi.mock('../ui/lottie.ts', () => ({ playTgs: vi.fn(() => () => {}) }));
const { playTgs } = await import('../ui/lottie.ts');
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { ALL, KINDS_GROUP, UPGRADED_GIFT_NAME, sampleOf } from '../../standin/kinds.ts';
import { chatStream, privateMessages } from '../../../mockup/src/data/messages.js';
import { nftHref, rarityText, rgb } from '../ui/servicegift.tsx';
import type { ChatProps } from './chat.tsx';
import type { MessageProps } from './message.tsx';
import type { ChatMessage } from '../types.ts';
import type { ServiceDetail } from '../ui/servicedetail.tsx';
import type { S } from '../api/schema.ts';

beforeEach(() => { setup(); vi.mocked(playTgs).mockClear(); });
const chatUrl = `/groups/${KINDS_GROUP.id}/chat`;
const SENDER = { id: 5, name: 'Sample Sender' } as ChatMessage['sender'];
const user = (id: number, title: string): S['ChatRef'] => ({ id, title, kind: 'user', photo: null });
const sticker = (url: string, f: 'tgs' | 'webm' | 'webp', emoji = '🎁') => ({ stored: true, fetchable: false, url, thumb: f === 'webp' ? url : null, render: f === 'tgs' ? 'lottie' : f === 'webm' ? 'animation' : 'image', ext: f, format: f, w: 512, h: 512, emoji, name: '', size: 1000 }) as S['ChatMedia'];
type Gift = S['ServiceGiftDetail'];
const gift = (g: Partial<Gift>): Gift => ({ kind: 'gift', sticker: null, symbol_sticker: null, star_count: null, price: null, sender: null, receiver: null, text: '', entities: [], upgraded: null, ...g });
const UPGRADED: S['UpgradedGift'] = {
	id: '601', regular_gift_id: '516', title: 'Plush Pepe', name: 'PlushPepe-1234', number: 1234, owner: user(8, 'Owner One'),
	model: { name: 'Cozy Frog', rarity_per_mille: 15, rarity: null, crafted: true },
	symbol: { name: 'Crown', rarity_per_mille: null, rarity: 'epic' },
	backdrop: { name: 'Emerald', center_color: 0x7ab46f, edge_color: 0x2e5e45, symbol_color: 0x1f4733, text_color: 0xe8f5e3, rarity_per_mille: 0, rarity: null },
	original_details: { sender: user(6, 'First Giver'), receiver: user(7, 'First Taker'), text: 'For the collection', entities: [], date: '2026-10-02T08:00:00Z' },
};

// The chat with one service message, built on a plain text one.
function line(type: string, text: string, detail: ServiceDetail) {
	const page = fixture<ChatProps>(chatUrl);
	const t = page.props.messages.find((m) => m.content_type === 'text')!;
	const service = { type, refs: { users: [], chats: [], message_id: null }, detail } as ChatMessage['service'];
	const m = { ...t, id: 1e9, message_id: 9e5, content_type: 'service', text, forward: null, reply_to: null, link_preview: null, deleted_at: null, edits: [], sender: SENDER, service } as ChatMessage;
	fakeApi('superadmin');
	renderPage(chatUrl, 'superadmin', { props: { ...page.props, messages: [m], at: null } });
	return document.getElementById('m900000')!.firstElementChild as HTMLElement;
}
const card = (l: HTMLElement) => l.querySelector('.c-gift') as HTMLElement;

describe('a gift in the chat', () => {
	it('shows a gift\'s sticker with the chat\'s Lottie player, its price, people and text', () => {
		const text = 'Happy birthday, see you!';
		const l = line('gift', 'sent a gift', gift({ sticker: sticker('/files/g1.tgs', 'tgs'), star_count: 50, sender: user(5, 'Sample Sender'), receiver: user(9, 'Lucky Friend'), text, entities: [{ offset: 0, length: 14, type: 'bold' }] }));
		const c = card(l);
		expect(within(c).getByRole('img', { name: 'sticker 🎁' }).classList.contains('c-lottie')).toBe(true);
		expect(playTgs).toHaveBeenCalledWith(expect.any(HTMLElement), '/files/g1.tgs', expect.any(Function), expect.any(Function));
		expect(c.querySelector('.c-giftstars')!.textContent).toBe('50 Stars');
		expect(within(c).getByRole('link', { name: /Sample Sender/ }).getAttribute('href')).toBe('/users/5');
		expect(within(c).getByRole('link', { name: /Lucky Friend/ }).getAttribute('href')).toBe('/users/9');
		expect(c.querySelector('.c-giftwho')!.textContent).toMatch(/^from .*Sample Sender.*to .*Lucky Friend/);
		const bold = within(c).getByText('Happy birthday');
		expect(['B', 'STRONG']).toContain(bold.tagName);
		// A regular gift has no backdrop and no upgraded parts.
		expect(c.querySelector('.c-giftart')!.getAttribute('style')).toBeNull();
		expect(c.querySelector('.c-giftattrs')).toBeNull();
	});

	it('shows an upgraded gift: the model over its backdrop, the symbol, title, link, rarities and origin', () => {
		const l = line('upgraded_gift', 'sent an upgraded gift', gift({ sticker: sticker('/files/model.tgs', 'tgs', '🐸'), symbol_sticker: sticker('/files/symbol.tgs', 'tgs', '👑'), upgraded: UPGRADED }));
		const c = card(l);
		const art = c.querySelector('.c-giftart') as HTMLElement;
		expect(art.style.background).toContain('radial-gradient');
		expect(art.style.background.replace(/\s/g, '')).toMatch(/#7ab46f|rgb\(122,180,111\)/);
		expect(art.style.background.replace(/\s/g, '')).toMatch(/#2e5e45|rgb\(46,94,69\)/);
		expect(within(art).getByRole('img', { name: 'sticker 🐸' })).toBeTruthy();
		expect(art.querySelector('.c-giftsymbol')!.getAttribute('aria-hidden')).toBe('true');
		expect(playTgs).toHaveBeenCalledWith(expect.any(HTMLElement), '/files/symbol.tgs', expect.any(Function), expect.any(Function));
		const title = within(c).getByRole('link', { name: 'Plush Pepe #1234' });
		expect(title.getAttribute('href')).toBe('https://t.me/nft/PlushPepe-1234');
		expect(title.getAttribute('target')).toBe('_blank');
		expect(title.getAttribute('rel')).toContain('noopener');
		expect(title.style.color.replace(/\s/g, '')).toMatch(/#e8f5e3|rgb\(232,245,227\)/);
		const attrs = [...c.querySelectorAll('.c-giftattr')].map((a) => a.textContent);
		expect(attrs).toEqual(['Model Cozy Frog1.5%crafted', 'Symbol CrownEpic', 'Backdrop Emeraldunder 0.1%']);
		expect(c.querySelector('.c-giftpill.epic')).toBeTruthy();
		expect(within(c).getByRole('link', { name: /Owner One/ }).getAttribute('href')).toBe('/users/8');
		const orig = c.querySelector('.c-giftorig') as HTMLElement;
		expect(orig.textContent).toMatch(/^originally from .*First Giver.*to .*First Taker.* on .*2026.*: For the collection$/);
		expect(orig.querySelector('time')!.getAttribute('datetime')).toBe('2026-10-02T08:00:00Z');
	});

	it('plays gifted Premium\'s video sticker, shows a picture sticker and the Stars of a prize', () => {
		const p = card(line('gifted_premium', 'gifted a Telegram Premium subscription', gift({ sticker: sticker('/files/prem', 'webm', '⭐') })));
		const v = p.querySelector('video')!;
		expect(v.getAttribute('src')).toBe('/files/prem');
		expect(v.loop && v.muted).toBe(true);
		document.body.innerHTML = '';
		const g = card(line('gifted_grams', 'sent a gift of grams', gift({ sticker: sticker('/files/grams', 'webp', '💎') })));
		expect(g.querySelector('img')!.getAttribute('src')).toBe('/files/grams');
		document.body.innerHTML = '';
		const s = card(line('giveaway_prize_stars', 'won Telegram Stars in a giveaway', gift({ sticker: sticker('/files/s.tgs', 'tgs', '⭐'), star_count: 1000 })));
		expect(s.querySelector('.c-giftstars')!.textContent).toBe('1,000 Stars');
	});

	it('shows a purchase offer\'s price in grams or Stars, and a gift whose look was not stored', () => {
		const o = card(line('upgraded_gift_purchase_offer', 'offered to buy a gift', gift({ price: { gram_cents: 125050 }, upgraded: UPGRADED })));
		expect(o.querySelector('.c-giftprice')!.textContent).toBe('offered 1,250.5 grams');
		document.body.innerHTML = '';
		const r = card(line('upgraded_gift_purchase_offer_rejected', 'rejected an offer to buy a gift', gift({ price: { stars: 2500 }, upgraded: { ...UPGRADED, model: null, symbol: null, backdrop: null, original_details: null, owner: null } })));
		expect(r.querySelector('.c-giftprice')!.textContent).toBe('offered 2,500 Stars');
		expect(within(r).getByRole('img', { name: 'Gift' })).toBeTruthy();
		expect((r.querySelector('.c-giftart') as HTMLElement).style.background).toBe('');
		expect(within(r).getByRole('link', { name: 'Plush Pepe #1234' })).toBeTruthy();
		expect(r.querySelector('.c-giftattrs, .c-giftorig')).toBeNull();
	});

	it('names the chat theme, and links the gift a theme came from', () => {
		const t = line('chat_set_theme', 'changed the chat theme', { kind: 'theme', theme_name: null, gift_id: '601', gift_name: 'PlushPepe-1234' });
		expect(within(t).getByRole('link', { name: 'PlushPepe-1234' }).getAttribute('href')).toBe('https://t.me/nft/PlushPepe-1234');
		expect(t.textContent).toContain('changed the chat theme to the theme of the gift PlushPepe-1234');
		document.body.innerHTML = '';
		const e = line('chat_set_theme', 'changed the chat theme', { kind: 'theme', theme_name: '🎄', gift_id: null, gift_name: null });
		expect(e.querySelector('q')!.textContent).toBe('🎄');
		expect(e.textContent).toContain('changed the chat theme to 🎄');
	});
});

describe('gift helpers', () => {
	it('builds colours only from whole RGB numbers', () => {
		expect(rgb(0x7ab46f)).toBe('#7ab46f');
		expect(rgb(0)).toBe('#000000');
		for (const bad of [-1, 0x1000000, 1.5, 'red', '#fff', null, undefined]) expect(rgb(bad)).toBeNull();
	});

	it('gives rarity as a percentage, under 0.1%, or its name', () => {
		expect(rarityText({ rarity_per_mille: 15, rarity: null })).toBe('1.5%');
		expect(rarityText({ rarity_per_mille: 0, rarity: null })).toBe('under 0.1%');
		expect(rarityText({ rarity_per_mille: 1000, rarity: null })).toBe('100%');
		expect(rarityText({ rarity_per_mille: null, rarity: 'legendary' })).toBe('Legendary');
		expect(rarityText({ rarity_per_mille: null, rarity: null })).toBeNull();
	});

	it('keeps a gift\'s name inside its t.me/nft link', () => {
		expect(nftHref('PlushPepe-1234')).toBe('https://t.me/nft/PlushPepe-1234');
		expect(nftHref('../../evil?x=1#y')).toBe('https://t.me/nft/..%2F..%2Fevil%3Fx%3D1%23y');
		expect(nftHref('')).toBeNull();
		expect(nftHref(null)).toBeNull();
	});
});

// The stand-in's gift samples, in the sample group and the first
// private chat.
type Raw = { id: number; message_id: number; chat: { id: number; kind: string } };
const pm = privateMessages()[0]!.chat as { id: number; kind: string };
const sample = (type: string, kind = 'gift', pick: (d: Record<string, unknown>) => boolean = () => true) => {
	for (const [chat, scope] of [[KINDS_GROUP, 'group'], [pm, 'private']] as const) {
		const m = (chatStream(chat) as Raw[]).find((x) => { const s = sampleOf(x)?.service?.(ALL); const d = s?.detail as Record<string, unknown> | null | undefined; return s?.type === type && d?.kind === kind && pick(d); });
		if (m) return { scope, id: m.id };
	}
	throw new Error(`no ${type} sample`);
};
const open = (type: string, kind = 'gift', persona: 'superadmin' | 'member' = 'superadmin', pick?: (d: Record<string, unknown>) => boolean) => {
	const { scope, id } = sample(type, kind, pick);
	fakeApi(persona);
	renderPage(`/messages/${scope}/${id}`, persona);
	return fixture<MessageProps>(`/messages/${scope}/${id}`, persona).props.message;
};
const text = () => document.querySelector('.a-msgtext') as HTMLElement;

describe('a gift\'s message page', () => {
	it('draws every gift sample', () => {
		for (const type of ['gift', 'upgraded_gift', 'gifted_premium', 'gifted_stars', 'gifted_grams', 'giveaway_prize_stars', 'upgraded_gift_purchase_offer', 'upgraded_gift_purchase_offer_rejected', 'refunded_upgraded_gift']) {
			document.body.innerHTML = '';
			const m = open(type);
			expect(m.service?.detail?.kind, type).toBe('gift');
			expect(text().querySelector('.c-gift'), type).toBeTruthy();
		}
	});

	it('shows the upgraded gift with its backdrop, link and rarities', () => {
		open('upgraded_gift');
		const c = text().querySelector('.c-gift') as HTMLElement;
		expect((c.querySelector('.c-giftart') as HTMLElement).style.background).toContain('radial-gradient');
		expect(within(c).getByRole('link', { name: 'Plush Pepe #1234' }).getAttribute('href')).toBe(`https://t.me/nft/${UPGRADED_GIFT_NAME}`);
		expect([...c.querySelectorAll('.c-giftpill')].map((p) => p.textContent)).toEqual(['1.5%', 'crafted', 'Epic', 'under 0.1%']);
		expect(c.querySelector('.c-giftorig')!.textContent).toContain('For the collection');
	});

	it('hides from a member the people it may not see', () => {
		const all = open('upgraded_gift');
		const o = (all.service!.detail as Gift).upgraded!.original_details!;
		expect(o.sender).not.toBeNull();
		document.body.innerHTML = '';
		const m = open('upgraded_gift', 'gift', 'member');
		expect((m.service!.detail as Gift).upgraded!.original_details!.sender).toBeNull();
		expect(within(text().querySelector('.c-giftorig') as HTMLElement).queryByRole('link', { name: new RegExp(o.sender!.title) })).toBeNull();
	});

	it('shows the Stars of a gift and a purchase offer\'s price', () => {
		open('gift');
		expect(text().querySelector('.c-giftstars')!.textContent).toBe('50 Stars');
		document.body.innerHTML = '';
		open('upgraded_gift_purchase_offer');
		expect(text().querySelector('.c-giftprice')!.textContent).toBe('offered 1,250.5 grams');
	});

	it('links the gift a chat theme came from, and names an emoji theme', () => {
		open('chat_set_theme', 'theme', 'superadmin', (d) => d.gift_name !== null);
		expect(within(text()).getByRole('link', { name: UPGRADED_GIFT_NAME }).getAttribute('href')).toBe(`https://t.me/nft/${UPGRADED_GIFT_NAME}`);
		document.body.innerHTML = '';
		open('chat_set_theme', 'theme', 'superadmin', (d) => d.theme_name !== null);
		expect(text().querySelector('q')!.textContent).toBe('🎄');
	});
});
