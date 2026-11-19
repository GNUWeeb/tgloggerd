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

- Go (webapi): a `RichDetail` replacing the placeholder for `rich`: the
  blocks converted from TDLib's JSON into a small documented shape
  (block type, rich text with its formatting, the media of a block as
  ChatMedia from the `rich` file rows by position, links), with
  unknown block types passed as a typed placeholder; a message whose
  JSON was over the cap sends its plain text only. OpenAPI and
  web/docs/message-kinds.md follow.
- React (frontend): a block renderer without innerHTML: text blocks,
  lists, quotes, preformatted code, tables, dividers, media with the
  chat's components, links with rel=noopener; collapsed to a few
  blocks in the chat with "show all", in full on the message page.
- Left out: embedded posts and embeds that need a third party's script
  (drawn as a link).

## Design notes

TDLib's page blocks are some thirty types (message-types.md section
3.1); start with the types the stored rows use, and send any other as its type name so the page
shows "a block this view does not show". The text formatting reuses
the text entities renderer (the text-entities item in the web
interface section, treated as landed).

## Depends on

Nothing; msg-rich-store, which stores the text and the media rows, has
landed.

## Done when

- A stored rich message shows its blocks and its media in the chat
  (collapsed) and on its message page (in full).
- Go tests convert sample JSON of each supported block type; React
  tests draw them; no block is drawn through innerHTML.
