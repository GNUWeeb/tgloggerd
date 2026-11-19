# msg-repair-census

Once the repair pass over the old rows has finished, count which rows
it gave a kind and detail and which it left as they were, turn the
pass off, and adjust the other message type items to what is left.

## Why

The rows logged before classification (`tdlib_type` NULL: the old
`unknown` rows and the service rows without `content_extra`) are being
re-read with `getMessage` by the opt-in repair pass (message-types.md
section 4.9). Several items below are sized on those rows: the linked
names of `msg-service-links`, the pinned snippets of
`msg-service-detail`, the unknown wording of `msg-kind-leftovers`.
After the pass most of them should carry a kind, a `tdlib_type` and
their curated keys; the rest will stay as they are, since Telegram no
longer returns them. While the flag stays on, every start tries those
again.

## Scope

- Daemon (TDLib session): when the log says the pass is done, count
  per message table the rows still without `tdlib_type`, by
  `content_type` and, for service rows, by `service_type`; and how
  often the pass met FLOOD_WAIT. Read-only queries.
- Turn `TG_REPAIR_MESSAGES_ON_START` off in the deployment once the
  counts say the pass is done.
- Write the outcome into message-types.md section 4.9 as shares (what
  part of the old rows was repaired, what part stayed), not as row
  counts of one deployment.
- Update the items of this section whose "Why" or size depended on the
  old rows, and drop any gap the pass closed.
- Left out: doing anything with the rows that stay; if they need more
  than the placeholder `msg-kind-leftovers` gives them, that is a new
  item.

## Design notes

The pass does not re-read media rows (decision
[2026-11-18-msg-repair-scope](../design/decisions/2026-11-18-msg-repair-scope.md)),
and rows that already have a `tdlib_type` (the rich messages, which are
stored with TDLib's JSON) are not touched either; `msg-rich-store` has
its own pass for those. A re-read service row gets the current
description as its `text`, which differs from the old one only for the
topic and boost notices.

## Depends on

Nothing (the repair pass is already running).

## Done when

- The counts are taken and the shares are in message-types.md section
  4.9.
- `TG_REPAIR_MESSAGES_ON_START` is off in the deployment, and a
  restart logs no repair pass.
- Every item in this section names the old rows correctly (repaired,
  or staying without detail).
