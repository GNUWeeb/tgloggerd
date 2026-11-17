# msg-link-preview-extras

Keep a link preview's description formatting, its media position, and
the message's link preview options.

## Why

The description is stored as plain text and the options (disabled,
forced url, size, position) are dropped (message-types.md section 4.8).

## Scope

- Additive migration on `telegram_link_previews`:
  `description_entities JSON`, `show_media_above_description`,
  `options JSON`; ingest fills them.

## Design notes

message-types.md section 4.8.

## Depends on

Nothing.

## Done when

- A text message with a formatted preview description and custom
  options stores both.
