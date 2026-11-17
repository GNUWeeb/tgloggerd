# msg-classify

Give every ingested message its exact TDLib class and a content kind,
and keep unmapped classes as TDLib JSON instead of dropping them.

## Why

Today any class outside eight media kinds and about thirty service
messages becomes `unknown` with nothing else stored.

## Scope

- `extract_message_content` sets `tdlib_type` for every class and the
  planned kind of message-types.md section 3 for every class.
- `messageAnimatedEmoji` stored as text (decision
  [2026-11-18-msg-animated-emoji-text](../design/decisions/2026-11-18-msg-animated-emoji-text.md)).
- Captions and entities for every class that has them (paid media,
  poll description, widget, invoice paid-media caption, checklist
  title).
- Classes without a curated mapping: `content_extra` = `{"tdlib":
  ...}` from TDLib's JSON serializer (link `tdjson_private`), without
  local paths.
- Edit snapshots and change detection include `tdlib_type` and
  `content_extra`.
- Upserts write the new columns under the same `edit_date` guard as
  the other content columns.
- Not in scope: the curated objects themselves (`msg-media-detail`,
  `msg-structured-content`, `msg-service-detail`).

## Design notes

message-types.md sections 3 and 4.1-4.2. Keep `forward_kind_str` and
the Discord forwarder working (new kinds map to `fwd_other`).

## Depends on

`msg-schema-kinds`

## Done when

- A table-driven test feeds one object of every one of the 109 classes
  and checks the kind, `tdlib_type`, text and that no class yields
  `unknown`; an unmapped class yields a `tdlib` object without paths.
