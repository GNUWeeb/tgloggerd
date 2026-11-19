# msg-gifts

Keep and show gifts as Telegram shows them: the gift's sticker, an
upgraded gift's model, symbol and backdrop, a gift theme, and the
animation stickers of Premium, Stars and giveaway prizes.

## Why

Gift service messages are stored with their ids, names, number, sender,
receiver, flags and prices, but not the gift's picture (a sticker) nor
an upgraded gift's model, symbol and backdrop; a chat theme set from a
gift does not say which gift. So the web can only say "sent a gift".

## Scope

- Daemon (TDLib session): new curated keys (no migration): for an
  upgraded gift `model {name, rarity_per_mille}`, `symbol {name}`,
  `backdrop {name, center_color, edge_color, symbol_color,
  text_color}`, `original_details`; for `chat_set_theme` from a gift,
  `gift_id` and `gift_name`. The gift's sticker (and an upgraded
  gift's model sticker) in `telegram_message_files` under role `gift`;
  the same role for the animation stickers of `gifted_premium`,
  `gifted_stars`, `gifted_grams`, `giveaway_prize_stars` and the
  prize sticker of `messageGiveaway`. Fetcher sources for them.
  Done: the daemon half; the keys are in message-types.md section
  4.2.3 (an upgraded gift's object, a gift theme), the files and
  their positions in section 4.3 (role `gift`).
- Go (webapi): a gift detail on ServiceInfo (sticker as ChatMedia,
  model, symbol, backdrop, price, sender and receiver through the
  scope), and the giveaway's prize sticker; OpenAPI and web/docs
  follow.
- React (frontend): a gift card on the service line: the sticker
  (Lottie or WebM with the chat's players) over the backdrop's
  gradient, the model and symbol names with rarity, the price; the
  prize sticker on a giveaway card.

## Design notes

message-types.md section 3.5 and 4.2.3. The premium gift code itself
stays unstored (redeemable). Old rows keep what they have; the
archive holds few gift messages so far, so this item is about new
ones.

## Depends on

Nothing.

## Done when

- A new gift and a new upgraded gift show their sticker, and the
  upgraded one its model, symbol and backdrop, on the service line and
  the message page.
- A giveaway card shows its prize sticker.
- Daemon unit tests of the curated keys from sample TDLib objects; Go
  and React tests of the detail.
