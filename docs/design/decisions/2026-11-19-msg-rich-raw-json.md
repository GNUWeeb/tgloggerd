# Decision log: 2026-11-19, Rich messages keep TDLib's JSON too

Decided on 2026-11-19, for
[../message-types.md](../message-types.md) section 4.2 and the
msg-rich-store item, as an exception to
[2026-11-18-msg-raw-json-unmapped-only](2026-11-18-msg-raw-json-unmapped-only.md):

- A rich message (`messageRichMessage`) has a curated `content_extra`,
  `{"blocks": <number of top-level blocks>, "is_rtl": <bool>}`, and
  keeps TDLib's JSON of the content under `tdlib` beside it. Its plain
  text is in `text` and the media in its blocks are message files
  (role `rich`).
- The web draws a rich message from the `tdlib` JSON. The curated keys
  are a summary, not a render source.
- A message whose JSON is over the 256 KiB cap keeps only the class
  and `tdlib_omitted_bytes` under its curated keys, like any other;
  its text, its curated keys and its file rows come from the live
  object, so they are complete even then.
- Reason: page blocks are some thirty types with nested rich text,
  tables and lists, and TDLib's shape is the most faithful description
  of them. A curated copy would repeat it key for key and fall behind
  whenever TDLib adds a block type. Rich messages are rare, so the
  size argument of the general rule does not apply to them.
