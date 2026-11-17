# shared-download-prune-race

Stop one message's store job from deleting the TDLib file that another
message's job still has to hash, which leaves that message unlinked.

## Why

When several messages wait on the same TDLib file (the same media
posted or forwarded more than once, or reprocessed by the backfiller),
`handle_file_update` emits every one of them, and each becomes its own
`files_` job. Each job hashes TDLib's local copy, stores it, and with
`TG_PRUNE_TDLIB_FILES` on deletes TDLib's copy. The first job to finish
deletes the file under the others: they log "Failed to hash file" and
their messages are never linked to the file. Production logs about 280
such errors a day.

## Scope

- Store a completed TDLib file once and link every waiting message to
  the resulting row, or prune TDLib's copy only after the last job
  waiting on that TDLib file is done.
- Not in scope: other causes of hashing failures.

## Design notes

- Found while building the on-demand fetch service, which delays its
  own prune by a grace period for the same reason.
- The relink sweep (`TG_RELINK_MEDIA_ON_START`) can repair messages
  already left unlinked; the item can say how many there are.

## Depends on

Nothing.

## Done when

- The daemon's "Failed to hash file" errors drop to near zero at
  deploy, and a file posted in two messages at once is linked to both.
