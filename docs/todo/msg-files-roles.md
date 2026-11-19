# msg-files-roles

Show on /files which role a file plays in its message (cover, game,
invoice, paid item, link preview, thumbnail) and let the list filter
by it (optional).

## Why

Files linked through `telegram_message_files` (message-types.md
section 4.3) appear on /files, if at all, with no hint that they are a
video's cover or a link preview's picture rather than the message's
own file. Whether and how they are listed was not confirmed.

## Scope

- React (frontend): the role as a small tag in the row, the card and
  the preview, and the filter.

## Design notes

The Go part is done (webapi): `FileRow.role` is `{role, message_kind}`
or null (`FileRoleRef`, `FileRole` in webapi/openapi.yaml), and the
files registry has a `role` field (`=`, `!=`, `in`), on the search API
and as a files page filter (web/docs/search-api.md). `role` is
`telegram_message_files.role` as stored, or `link_preview` for a link
preview's picture (`telegram_link_previews.photo_file_id`). A file
that is no visible message's own file names the first visible message
that carries it in a role; a message's own file has `role: null` and
names that message, even when it plays a role in another (the filter
still finds it).

What stays as before: a scope that sees only some chats (`filesFilter`)
lists only the files that are the own file of a message it sees, so
for such a viewer the files that are only a cover, a thumbnail or a
link preview's picture are still not listed.

## Depends on

Nothing.

## Done when

- A link preview's picture on /files shows the role as a tag and links
  to its message; filtering by the role lists only those.
- React tests cover the tag and the filter.
