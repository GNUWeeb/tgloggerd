# download-failure-handling

Notice when a TDLib download fails or is cancelled, and stop waiting
for it.

## Why

Every download is sent without a response handler, and `updateFile`
handling ignores anything not completed. A failed download is never
noticed, its pending entry stays in memory until the process exits,
and its message is never linked.

## Scope

- Handle the error reply of `downloadFile` and the cancelled or failed
  states seen in `updateFile`; drop the pending entries; log them.
- Not in scope: retrying (the relink sweep can pick such messages up).

## Design notes

- Found while planning the storage cache. The pending maps are
  `pending_message_file_`, `pending_photo_` and `pending_group_photo_`
  in TDLib.cpp.

## Depends on

Nothing.

## Done when

- A download that fails is logged once and leaves no pending entry.
