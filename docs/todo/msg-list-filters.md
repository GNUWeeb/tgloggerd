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
- `service_type` and `tdlib_type` are free-text filters: one has to
  know "chat_add_members" to use them.
- The files list's type filter lacks `video_note` and `thumbnail`,
  which the server offers, and its quick filters cover photos, videos
  and documents only.

## Scope

- React (frontend): `service` in the private list's filter; "Media" as
  the media kinds (photo, video, video_note, document, audio, voice,
  sticker, animation, paid_media); `video_note` and `thumbnail` in the
  files type filter; quick filters for voice, audio, stickers,
  animations and video notes on /files.
- Go (webapi): `service_type` and `tdlib_type` as enum fields in the
  search registry (webapi/internal/logdb/search_entities.go), with
  the known values (the daemon's service type names, the TDLib
  classes), still within one chat; React shows them as lists with the
  labels of `msg-type-labels`. web/docs/search-api.md follows.
- If the list filter cannot express "one of" for "Media", Go adds it
  (an IN operator on enum fields) in this item.

## Design notes

Whether the filter bar can already express "one of" was not confirmed;
check first. An enum filter keeps accepting a value it does not list
(a service type newer than the web), so it does not reject queries
that work today.

## Depends on

Nothing.

## Done when

- The private messages list filters by `service`; "Media" on the group
  messages list returns no service, poll or location rows.
- The service type filter offers a list with labels, and picking
  "Members added" returns those rows.
- /files filters video notes, voice, audio, stickers and animations.
- Go and React tests cover the new filters.
