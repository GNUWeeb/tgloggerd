# log-freed-before-shutdown

Free the daemon's log handle only after everything that may log has
stopped.

## Why

`~TgLoggerd` calls `pr_free(l_)` in its body, which runs before its
members (`tdlib_`, the thread pools, the Discord forwarder) are
destroyed. Anything that logs from one of those destructors, or from a
thread still finishing, would use a freed handle. Nothing does today:
TDLib's own messages are logged only from update handlers on the loop
thread, which has stopped by then. New threads (the internal API, the
fetch service, the storage janitor) make it likely that something will.

## Scope

- Destroy or stop the members that may log before freeing the handle,
  or free it in a way that runs last.
- Not in scope: other shutdown ordering.

## Design notes

- Found while adding the daemon's log handle to TDLib
  (chat-membership-tracking).

## Depends on

Nothing.

## Done when

- The handle is freed after every member that may use it; the daemon
  still stops cleanly on SIGTERM.
