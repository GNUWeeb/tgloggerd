# msg-repair-old-rows

Re-read rows stored before the change as `unknown`, and service rows
without detail, and store them again with the new extraction.

## Why

Rows written before the change say nothing about what they were.

## Scope

- A rate-limited job on the relink re-fetch path: `getMessage` for each
  such row, re-ingested through the normal path; rows TDLib cannot
  return keep what they have.
- Progress is resumable across restarts and reported in the log.
- Rows written since classification whose `content_extra` holds only
  TDLib's JSON (`{"tdlib": ...}`) for a class that has a curated mapping
  by then are re-mapped from that JSON, without asking TDLib; media
  rows too, since this costs no requests.
- Media rows are not re-read from TDLib (decision
  [2026-11-18-msg-repair-scope](../design/decisions/2026-11-18-msg-repair-scope.md)).

## Design notes

message-types.md section 4.9. Deleted messages are not re-read.

## Depends on

`msg-service-detail`, `msg-polls`

## Done when

- A dry run reports how many rows it would re-read; the real run
  leaves no `unknown` row that TDLib can still return.
