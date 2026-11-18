# msg-interaction

Choose the interval at which interaction totals are written, from a
day of the rate the live daemon reports.

## Why

Views, forwards, replies and reaction totals are stored and kept
current, coalesced per message and written every
`TG_INTERACTION_FLUSH_SECS` (default 60). The default is a guess: the
design asked for the update rate to be measured first, and that needs
the live account.

## Scope

- After the next tgloggerd restart, collect a day of the
  "interaction: N update(s) for M message(s) ... W write(s)" lines the
  daemon logs every ten minutes.
- Record the measured rate (updates and distinct messages per minute,
  peak and typical) in message-types.md section 4.7, and choose the
  interval from it: as long as keeps the writes well below the rest of
  the daemon's database load, and short enough that the web shows
  totals that are minutes, not hours, old.
- Change the default if the measurement says so.

## Design notes

message-types.md section 4.7; decision
[2026-11-18-msg-interaction-latest-totals](../design/decisions/2026-11-18-msg-interaction-latest-totals.md).
Reading the logs is read-only; changing TG_INTERACTION_FLUSH_SECS in
production is the owner's call.

## Depends on

Nothing.

## Done when

- The measured rate is recorded in the design and the interval is set
  from it.
