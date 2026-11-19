# msg-rich-store

Store a rich message so the web can search, preview and draw it: its
plain text, a short curated summary beside TDLib's JSON, and the media
inside its page blocks as files.

## Why

`messageRichMessage` is stored only as TDLib's JSON (the class has no
curated mapping), so it has no `text` (not searchable, no preview
line) and the photos, videos and other media in its blocks are not
linked as files, so they are never stored or fetchable. It is the
largest kind the web shows only as a placeholder.

## Scope

Daemon (TDLib session):

- `text`: the blocks' plain text (headings, paragraphs, captions, list
  items, table cells), for full-text search and a one-line preview.
- `content_extra`: curated `{"blocks": <count>, "is_rtl": <bool>}`
  alongside `tdlib`, which stays the render source.
- Every photo, video, animation, audio and document inside the blocks
  in `telegram_message_files` under role `rich`, `position` in order of
  appearance, stored or recorded under the storage cache rules like any
  other message file.
- A fetcher source for those files: find a rich message's media by
  unique id in a fresh `getMessage`, as the photo history sources do.
- The rows already stored: the repair pass does not re-read them (they
  have a `tdlib_type`). Either re-read them with `getMessage` through
  the normal path, or re-map them locally from the stored JSON (the
  file ids are in it); the owner chooses, and the choice is a decision
  file. The earlier decision that dropped the general local re-map
  ([2026-11-18-msg-repair-opt-in](../design/decisions/2026-11-18-msg-repair-opt-in.md))
  was about rows a new mapping would not find; these exist.
- Left to `msg-rich-render`: everything the web does with it.

## Design notes

Keeping TDLib's JSON beside curated keys is an exception to
[2026-11-18-msg-raw-json-unmapped-only](../design/decisions/2026-11-18-msg-raw-json-unmapped-only.md):
page blocks are some thirty types and TDLib's shape is the most
faithful, so the web renders from it instead of a curated copy. Record
the exception as a decision file. A message over the JSON cap (256
KiB) keeps only the class and `tdlib_omitted_bytes`; its text and its
file rows still come from the live object, so they are written before
the JSON is capped. Design: message-types.md section 3.1 and 4.2.

## Depends on

Nothing.

## Done when

- A new rich message logged by the daemon has `text`, `blocks` and
  `is_rtl`, and one `rich` row per media item in its blocks.
- The rows stored before are filled the way the decision file says,
  and a full-text search for a word of one of them finds it.
- An unstored photo of a rich message is fetched through `/files/` by
  the new source.
