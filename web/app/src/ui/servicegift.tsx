// A gift on its service line, as Telegram shows it (docs/todo/msg-gifts.md):
// the gift's sticker, an upgraded gift's model sticker over its
// backdrop with its symbol, title, number and rarities, the price,
// who gave it to whom and the text sent with it; and a chat theme set
// from a gift. The chat's service line and the message page both draw
// it, inside the line's text, so it is built of inline elements.
import type { CSSProperties } from 'react';
import { Gift } from 'lucide-react';
import { Sticker } from '../routes/chat-media.tsx';
import { RefLink } from '../routes/chat-kinds.tsx';
import { RichText, safeHref } from './richtext.tsx';
import { dateTime, fmt } from '../format.ts';
import type { Viewer } from '../page.ts';
import type { S } from '../api/schema.ts';

type Gift = S['ServiceGiftDetail'];
type Theme = S['ServiceThemeDetail'];
type Upgraded = S['UpgradedGift'];
type Rarity = { rarity_per_mille: S['GiftRarityPerMille']; rarity: S['GiftRarity'] };

// An RGB integer as a CSS colour; anything but a whole number in range
// gives nothing, so no other text reaches a style.
export function rgb(n: unknown): string | null {
	return typeof n === 'number' && Number.isInteger(n) && n >= 0 && n <= 0xffffff ? `#${n.toString(16).padStart(6, '0')}` : null;
}

// How rare an attribute is: a proportion as a percentage ("1.5%",
// "under 0.1%" for 0), or the rarity Telegram names ("Epic").
export function rarityText(r: Rarity): string | null {
	const pm = r.rarity_per_mille;
	if (typeof pm === 'number' && pm >= 0 && pm <= 1000) return pm === 0 ? 'under 0.1%' : `${pm / 10}%`;
	return r.rarity ? r.rarity[0]!.toUpperCase() + r.rarity.slice(1) : null;
}

// An upgraded gift's page on Telegram; its name is one path segment.
export const nftHref = (name: string | null | undefined): string | null => (name ? safeHref(`https://t.me/nft/${encodeURIComponent(name)}`) : null);

const plural = (n: number, one: string) => `${fmt(n)} ${one}${n === 1 ? '' : 's'}`;
// A purchase offer's price: Stars, or grams given in hundredths.
function priceText(p: Gift['price']): string | null {
	if (!p) return null;
	if ('stars' in p) return plural(p.stars, 'Star');
	return `${(p.gram_cents / 100).toLocaleString('en-US', { maximumFractionDigits: 2 })} grams`;
}

const EXT = { target: '_blank', rel: 'noopener noreferrer nofollow' } as const;

// The upgraded gift's title and number, linked to its page when its
// name is known.
function Title({ u, color }: { u: Upgraded; color?: string | null }) {
	const label = `${u.title || 'Upgraded gift'}${u.number > 0 ? ` #${u.number}` : ''}`;
	const href = nftHref(u.name);
	const style = color ? { color } : undefined;
	return href ? <a className="c-gifttitle" href={href} {...EXT} style={style}>{label}</a> : <span className="c-gifttitle" style={style}>{label}</span>;
}

// One attribute of an upgraded gift: what it is, its name and rarity.
function Attr({ what, name, r, crafted = false }: { what: string; name: string; r: Rarity; crafted?: boolean }) {
	const rare = rarityText(r);
	return (
		<span className="c-giftattr">
			<span className="c-sub">{what}</span> {name}
			{rare && <span className={`c-giftpill${r.rarity ? ` ${r.rarity}` : ''}`} title={r.rarity_per_mille !== null ? `${rare} of these gifts have it` : undefined}>{rare}</span>}
			{crafted && <span className="c-giftpill crafted" title="Only obtained by crafting gifts">crafted</span>}
		</span>
	);
}

// The picture: the sticker on the backdrop's radial gradient (centre to
// edge), with the symbol once in a corner. Each copy of a symbol is its
// own player (most are animated .tgs files, which no CSS can tint in
// symbol_color), so a repeated pattern would cost a player per copy on
// every gift of a long chat; one copy still says which symbol it has.
function Art({ d }: { d: Gift }) {
	const u = d.upgraded;
	const b = u?.backdrop;
	const center = rgb(b?.center_color);
	const edge = rgb(b?.edge_color);
	const style: CSSProperties | undefined = center && edge ? { background: `radial-gradient(circle at 50% 45%, ${center}, ${edge})` } : undefined;
	return (
		<span className={`c-giftart${style ? ' c-giftbackdrop' : ''}`} style={style}>
			{/* The symbol is a backdrop: no fetch button of its own. */}
			{d.symbol_sticker && <span className="c-giftsymbol" aria-hidden="true"><Sticker m={{ ...d.symbol_sticker, fetch: null }} /></span>}
			{d.sticker ? <Sticker m={d.sticker} /> : <span className="c-giftnosticker" role="img" aria-label="Gift"><Gift size={48} /></span>}
			{u && <Title u={u} color={rgb(b?.text_color)} />}
		</span>
	);
}

// Who gave it to whom, each linked as the viewer may see them.
function FromTo({ from, to }: { from: S['ChatRef'] | null; to: S['ChatRef'] | null }) {
	if (!from && !to) return null;
	return <>{from && <>from <RefLink r={from} /></>}{to && <>to <RefLink r={to} /></>}</>;
}

// The gift card after the service text.
export function GiftCard({ d, v }: { d: Gift; v: Viewer }) {
	const u = d.upgraded;
	const price = priceText(d.price);
	const o = u?.original_details;
	return (
		<span className="c-gift">
			<Art d={d} />
			{(d.star_count !== null || price) && (
				<span className="c-giftline">
					{d.star_count !== null && <span className="c-giftstars">{plural(d.star_count, 'Star')}</span>}
					{price && <span className="c-giftprice">offered {price}</span>}
				</span>
			)}
			{(d.sender || d.receiver) && <span className="c-giftline c-giftwho"><FromTo from={d.sender} to={d.receiver} /></span>}
			{d.text && <span className="c-gifttext"><RichText text={d.text} entities={d.entities} /></span>}
			{u && (u.model || u.symbol || u.backdrop) && (
				<span className="c-giftattrs">
					{u.model && <Attr what="Model" name={u.model.name} r={u.model} crafted={u.model.crafted} />}
					{u.symbol && <Attr what="Symbol" name={u.symbol.name} r={u.symbol} />}
					{u.backdrop && <Attr what="Backdrop" name={u.backdrop.name} r={u.backdrop} />}
				</span>
			)}
			{u?.owner && <span className="c-giftline c-sub">owned by <RefLink r={u.owner} /></span>}
			{o && (o.sender || o.receiver || o.text || o.date) && (
				<span className="c-giftline c-giftorig c-sub">
					originally {o.sender || o.receiver ? <FromTo from={o.sender} to={o.receiver} /> : 'sent'}{o.date && <> on <time dateTime={o.date}>{dateTime(o.date, v)}</time></>}{o.text && <>: <RichText text={o.text} entities={o.entities} /></>}
				</span>
			)}
		</span>
	);
}

// The theme a chat was set to: an emoji theme's name, or the upgraded
// gift it was made from, linked to its page.
export function ThemeDetail({ d }: { d: Theme }) {
	if (d.theme_name) return <> to <q className="c-svcquote">{d.theme_name}</q></>;
	if (d.gift_id === null && !d.gift_name) return null;
	const href = nftHref(d.gift_name);
	const name = d.gift_name || 'a gift';
	return <> to the theme of the gift {href ? <a className="c-svcref" href={href} {...EXT}>{name}</a> : name}</>;
}
