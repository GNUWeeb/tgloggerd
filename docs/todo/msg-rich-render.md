# msg-rich-render

Draw a rich message in the chat and on its message page: its page
blocks (headings, paragraphs, lists, quotes, tables, media) instead of
the one-line placeholder.

## Why

A rich message shows "A rich message, which this archive does not
show". It is the largest kind the web shows only as a placeholder,
and its whole content is stored as TDLib's JSON, with its text and
media made available by `msg-rich-store`.

## Scope

- Go (webapi): done. `RichDetail` (webapi/openapi.yaml: RichDetail,
  RichBlock, RichText; web/docs/message-kinds.md, "Rich messages")
  sends every block type, a block of an unknown class as type
  `unknown` with its class in `name`, and null blocks when the page
  was not kept (the message text is shown then).
- React (frontend): a block renderer without innerHTML: text blocks,
  lists, quotes, preformatted code, tables, dividers, media with the
  chat's components, links with rel=noopener; collapsed to a few
  blocks in the chat with "show all", in full on the message page.
- Left out: embedded posts and embeds that need a third party's script
  (drawn as a link).

## Design notes

The API sends a block's rich text as `{text, entities}`, so the text
entities renderer (the text-entities item in the web interface
section, treated as landed) draws it; it adds the entity types
`subscript`, `superscript` and `marked`. A block of type `unknown`
shows "a block this view does not show". The app's stand-in still
sends the old placeholder for `rich` (web/app/standin/kinds.ts) and
must send a RichDetail.

## Depends on

Nothing; msg-rich-store, which stores the text and the media rows, has
landed.

## Done when

- A stored rich message shows its blocks and its media in the chat
  (collapsed) and on its message page (in full).
- React tests draw each block type; no block is drawn through
  innerHTML.
