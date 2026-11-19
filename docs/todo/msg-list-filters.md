# msg-list-filters

Make the message and file lists filter by kind the way a reader
expects: service rows in private chats, a "Media" filter that means
media, service and TDLib types to pick from, and every file type.

## Why

- The private messages list's content type filter leaves out
  `service`, though private chats have service rows and the server
  accepts it.
- The "Media" quick filter is `content_type != text`, so it also
  matches service, unknown, poll, location, call and the other
  non-media kinds.
- The service type and TDLib type filters are text inputs: one has
  to know "chat_add_members" to use them.
- The files list's type filter lacks `video_note` and `thumbnail`,
  which the server offers, and its quick filters cover photos, videos
  and documents only.

## Scope

- React (frontend): `service` in the private list's filter; "Media" as
  the media kinds (photo, video, video_note, document, audio, voice,
  sticker, animation, paid_media) as one `in` filter; `video_note`
  and `thumbnail` in the files type filter; quick filters for voice,
  audio, stickers, animations and video notes on /files.
- React: `service_type` and `tdlib_type` as lists with the labels of
  `msg-type-labels`, still within one chat.
- React: `Cond.value` is now a string or, for `in`, a list of strings
  (the regenerated schema.d.ts); the filter bar's code that reads it
  as a string (ui/list.tsx) follows.

## Design notes

The Go part is on dev: the search conditions take `in` on enum
fields, `{"field": ..., "op": "in", "value": [...]}` on the list
pages (also file_type on /files), and `service_type` and `tdlib_type`
are open enums (`enum_open: true` in ListState.fields: the known
values listed in `enum`, any other value taken too).
web/docs/search-api.md describes both.

## Depends on

Nothing.

## Done when

- The private messages list filters by `service`; "Media" on the group
  messages list returns no service, poll or location rows.
- The service type filter offers a list with labels, and picking
  "Members added" returns those rows.
- /files filters video notes, voice, audio, stickers and animations.
- React tests cover the new filters (the Go tests are on dev).
