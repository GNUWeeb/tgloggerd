# fwd-more-types

Let each forwarding route choose more of the message types it carries
than today's six (sticker, text, photo, video, document, other), and
lay out the route form so the longer list stays easy to use.

## Why

Migration 000031 gave each route six content flags. Everything else
(voice notes, audio, animations, video notes, polls, locations,
contacts, service messages, rich messages and the other new kinds of
docs/design/message-types.md section 4.10) falls under `fwd_other`, so
a route cannot, for example, carry polls but drop service messages,
or carry GIFs but not voice notes. Rich messages stay under "other"
for now (decided 2026-11-20); this item is where they may get a flag
of their own.

## Scope

- The design is decided:
  [../design/decisions/2026-11-20-fwd-more-types.md](../design/decisions/2026-11-20-fwd-more-types.md)
  (the flags and groups, storage, API shape, form and list).
- Daemon (TDLib session): both bridges (tgloggerd's Telegram to
  Discord forwarder and discordd's Discord to Telegram routes) honour
  the new flags part by part, as today, a summary line of
  message-types.md section 4.10 following its kind's flag; kinds with
  no flag of their own keep following `fwd_other`. A migration adds
  the flags, each copying the flag its type followed until now, so no
  route changes what it forwards.
- Go (webapi): the forwarding API reads and writes the new flags;
  OpenAPI and web/docs/platform-forwarding.md follow.
- React (frontend): the route form and the route list show the new
  flags in the agreed layout.

## Depends on

Nothing.

## Done when

- A route can carry polls and drop service messages (and each other
  new flag works on its own), in both directions where it applies.
- Existing routes forward exactly what they forwarded before.
- Daemon, Go and React tests cover the flags, the migration's
  defaults and the form.
