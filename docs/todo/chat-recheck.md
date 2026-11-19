# chat-recheck

Recheck a chat's messages against Telegram when it is opened or
scrolled, and after a daemon restart, so messages, edits and
deletions the daemon missed show up.

## Why

Opening a group chat sometimes shows the newest message but not a few
recent ones before it. The daemon learns of messages from TDLib's
updates and a one-time history walk; after a restart, Telegram can
answer a busy supergroup's catch-up with "too much changed" instead of
the missed messages, and nothing walks that stretch again. Edits and
deletions made while the daemon was away are lost the same way.

## Decisions (owner, 2026-11-20)

- Recheck when a group or private chat is opened, and when an older
  page is loaded while scrolling up, around that page's messages.
- A message found missing is stored as if it had just arrived; an edit
  found is stored as a new version, as a live edit is.
- Deletions: within the stretch Telegram returns, a stored message that
  is no longer there is marked deleted, flagged as found on recheck so
  it is told apart from a deletion seen live. Nothing outside the
  returned stretch is concluded.
- Signed-in viewers trigger rechecks. Public visitors (exposed groups)
  trigger them too, but under one shared rate limit for everyone not
  signed in, which favours the newest messages.
- After a daemon restart, the daemon rechecks the newest messages of
  its recently active chats by itself, slowly.
- Messages found by a recheck are not forwarded to Discord late, as
  the history walk does not forward either.

## Scope

Done: the daemon half. POST /v1/chats/{chat_id}/recheck queues a
recheck and answers at once; the recheck stores what getChatHistory
returns through the normal path and marks deletions within the
returned stretch with `meta.deleted_on_recheck` (no migration: the
flag lives in the existing `meta` column); the restart catch-up runs
two minutes after authorization; docs/internal-api.md has the contract.
Left: the Go and React halves.

- Daemon (TDLib session): an internal endpoint that queues a recheck
  of a chat around a message id (0 = the newest), answering at once;
  the recheck reads that stretch from Telegram (getChatHistory), stores
  what is missing or changed, marks deletions within the stretch, and
  sends the usual message events so open pages update. Paced with the
  repair and relink queue's Telegram budget, at most once a minute per
  chat for the newest stretch and once a day per older stretch. The
  restart catch-up over recently active chats. A migration for the
  "found on recheck" deletion flag if it needs a column (next free
  000051). docs/internal-api.md.
- Go (webapi): ask for a recheck in the background when a chat window
  or an older page is served, never waiting for it; per-chat and
  per-stretch limits for signed-in viewers, one shared bucket for all
  others; the deletion flag in the API where deletions show.
- React (frontend): the updates arrive through the live events already
  drawn; show a recheck-found deletion as such.

## Depends on

Nothing.

## Done when

- A message that never reached the daemon appears in an opened chat
  within moments, without a reload, and is stored.
- A message deleted while the daemon was away shows as deleted after
  the chat is opened, flagged as found on recheck.
- A burst of public visitors cannot push the account's Telegram
  requests past the shared budget; tests cover both limits.
