# photo-message-pending-overlap

Link a message to its media even when the same TDLib file is also a
group or user photo waiting to be stored.

## Why

`handle_file_update` (src/tgloggerd/TDLib.cpp) checks the pending maps
in turn and returns at the first that holds the completed file. A file
waiting both as a photo (`pending_photo_` or `pending_group_photo_`)
and as message media (`pending_message_file_`), for example the message
that set a chat's photo, is emitted only as the photo; the message is
never linked to its file.

## Scope

- Emit every pending use of a completed file, keeping the store holds
  of the prune race fix (StoreHolds) and the download-failure
  bookkeeping consistent.

## Depends on

Nothing.

## Done when

- A file that is both a chat photo and a message's media is stored
  once and linked as both; a harness scenario covering it fails before
  the change and passes after.
