# msg-custom-emoji-store

Fill the custom emoji table on demand, so a custom emoji id stored in a
text entity, a reaction, a topic icon or a message effect can be drawn
as its sticker.

## Why

Custom emoji are stored as ids wherever they appear (text entities,
reactions, forum topic icons, `meta.effect_id`), but the stickers are
not, so the web shows a diamond for a custom emoji reaction and the
plain fallback emoji (or nothing) elsewhere.

## Scope

Daemon (TDLib session):

- An internal API endpoint taking a batch of custom emoji ids (up to
  200, `getCustomEmojiStickers`'s limit) that returns their rows,
  asking TDLib only for the ids not yet in `telegram_custom_emoji`.
- For each sticker, a row in `telegram_custom_emoji` and a
  metadata-only `telegram_files` row (not downloaded), fetched by
  `/files/` like any unstored file under the storage cache rules.
- A fetcher source that finds a custom emoji's file again by its id.
- Message effects: decide whether `getMessageEffect`'s sticker shares
  the table (by its custom emoji id) or needs a sibling table; record
  the choice in message-types.md.
- Go (webapi): a daemon client call for the endpoint; nothing on the
  pages yet (`msg-custom-emoji-web`).

## Design notes

On demand rather than at ingest: most ids repeat (the same packs), and
the web asks only for the ones it is about to draw. A row once filled
is kept; a sticker's file follows the cache like other files. The
table comes from `msg-schema-prep`.

## Depends on

`msg-schema-prep` (the table).

## Done when

- Calling the endpoint with ids taken from stored entities and
  reactions returns a row for each one Telegram knows, and calling it
  again asks TDLib for none of them.
- One of those stickers' files is fetched through `/files/` and its
  file row then shows as stored.
