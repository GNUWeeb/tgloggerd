# daemon-event-stream

Publish what changes in the archive as a stream of small events on the
daemon's internal API.

## Why

The web should update live (docs/design/web-live.md); only the daemon
knows when something changed.

## Scope

- `GET /v1/events?since=<seq>` on the internal API: a long-lived
  stream of the events of web-live.md section 3, each with a `seq`.
- Emitted after commit, from the serial_ and files_ workers that write
  messages, edits, deletions, users, groups and files; counters
  coalesced into at most one `stats.changed` per second.
- A bounded in-memory ring of recent events for resuming; a gap answer
  when `since` is older than the ring.
- `docs/internal-api.md` updated.

## Depends on

Nothing.

## Done when

- A subscriber sees each kind of event within a second of the change,
  resumes without loss after a short disconnect, and is told to
  resynchronise after a long one.
