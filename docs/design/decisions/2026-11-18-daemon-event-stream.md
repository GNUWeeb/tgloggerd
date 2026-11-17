# Decision log: 2026-11-18, The daemon's event stream

Decided while implementing `daemon-event-stream`, for
[../web-live.md](../web-live.md) sections 3 and 4. The contract is
`GET /v1/events` in [../../internal-api.md](../../internal-api.md).

- Wire format: Server-Sent Events over the internal API's HTTP/1.1
  server, chunked, the event's JSON in `data` and its seq in `id`. The
  browser side uses SSE as well, so the web relays frames without
  translating them, and any SSE or line reader can consume it.
- No handler thread per subscriber: a stream is served from the API's
  I/O thread, which pulls from the event ring whenever the ring wakes
  it. Subscribers are bounded (16) anyway, as the web holds one
  subscription and fans it out. Reason: handler threads are few and
  are held by slow fetches; a subscriber holding one forever would
  starve them.
- No queue per subscriber: every subscriber reads the one bounded ring
  at its own seq. A subscriber that falls behind the ring gets a
  `resync` and goes on from the newest event, so a slow reader costs
  no memory and never holds up the workers that publish.
- Seqs start from the daemon's start time in microseconds rather than
  from 1, so a seq from before a restart is always older than the ring
  and is answered with a `resync`, never mistaken for an event of the
  new run.
- The first frame is always `hello` or `resync` with the seq the
  stream starts after, so a client knows where it stands before any
  event arrives and can resume from there.
- Counters are coalesced on a fixed one-second cadence, as one
  `stats.changed` listing the chats and users that moved, capped at
  1000 ids per list with a `more` flag beyond.
