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

- First check how /files lists those files today, and write down what
  it does.
- Go (webapi): `FileRow.role` (the role and the message's kind) and a
  role filter in the search registry; web/docs/search-api.md follows.
- React (frontend): the role as a small tag in the row, the card and
  the preview, and the filter.

## Depends on

`msg-list-filters` (the same filter fields).

## Done when

- A link preview's picture on /files says it is one and links to its
  message; filtering by the role lists only those.
- Go and React tests cover the field and the filter.
