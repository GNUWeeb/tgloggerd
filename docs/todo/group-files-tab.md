# group-files-tab

A Files tab on the group page: the group's files, newest message
first, in sub tabs by kind, each with its sender and links to its
message.

## Why

The files list (/files) spans every chat; to browse one group's media
there is no page, and no way to see who sent a file or jump to the
message that carried it.

## Scope

- A Files tab beside Overview, Admins, History and Messages, with sub
  tabs All, Photos, Videos, GIF, Audio, Voice, Documents, Stickers,
  Video notes (only the kinds the group has, with counts where cheap).
- The listing follows the /files page's grid and list views (the
  frontend session is reworking them; reuse its components), ordered
  by the Telegram message id that carried the file, newest first.
- Each file shows its sender (name and photo) and links to the
  message in the group's chat (?at=) and to the message page.
- Go: a page route for the tab (e.g. /groups/{id}/files?kind=) and
  the list from the messages' files of that group (group messages
  joined to their files and telegram_message_files), within the
  request scope, paged by message id; one query per page, no query
  per row; the sender batched like other lists.
- React: the tab, the sub tabs, the grid and list, the links.

## Depends on

Nothing.

## Done when

- A group's Files tab lists its files newest first with each sender
  and both links, per kind, for every role that may see the group.
