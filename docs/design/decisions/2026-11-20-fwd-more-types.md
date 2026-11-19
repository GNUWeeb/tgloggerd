# Decision log: 2026-11-20, More forwarding content types

Decided on 2026-11-20, for the fwd-more-types item
([../../todo/fwd-more-types.md](../../todo/fwd-more-types.md)), from
the frontend's UX proposal, the daemon's per-bridge review and the
owner's answers:

- Telegram to Discord gets one flag per type an admin would separate,
  in groups:
  - Text: `text`;
  - Media: `photo`, `video`, `animation`, `video_note`, `document`,
    `sticker`;
  - Audio: `voice`, `audio`;
  - Polls and games: `poll` (quizzes too), `checklist`, `dice`,
    `game`;
  - Places and contacts: `location` (venues and live locations too),
    `contact`;
  - Telegram extras: `story`, `paid_media`, `invoice`;
  - Service messages: `service`;
  - Everything else: `other`.
- `other` stays the catch-all: rich messages (decided by the owner on
  2026-11-20), calls and giveaways (the owner: no flags of their own
  until someone needs them) and every kind added later follow it, so
  a new kind is never silently dropped.
- Discord to Telegram gains `audio` (audio/* attachments, a document
  today) and `animation` (image/gif attachments, a photo today) beside
  its five flags. Every Discord sticker, animated or not, stays under
  `sticker`, as it is today, so no route changes what it forwards.
  Voice messages, embeds, polls and system messages need gwdiscord
  to parse more of a message (flags, embeds, poll, type) and discordd
  to render them; that is the follow-up item fwd-discord-kinds, not
  this one.
- Part by part, as before: a message's own text or caption (a text
  message, a message with a file, paid media) follows `text`, a file
  its type's flag, and a kind's summary line (location, contact, poll,
  story, ...; message-types.md section 4.10) its kind's flag. A kind's
  own text follows the kind's flag too, so turning a kind off drops
  all of it: a poll's description, a game's text, an invoice's
  description, a checklist's title, a service line, and a rich
  message's text (under `other`).
  Edits are filtered like the message.
- Storage: one TINYINT(1) column per flag, as migration 000031 did;
  both daemons keep turning them into a bitmask in memory. Each new
  column is added with DEFAULT 1, then one UPDATE copies the flag the
  type followed until now: Telegram to Discord from `fwd_other`;
  Discord to Telegram `audio` from `fwd_document` and `animation`
  from `fwd_photo`. No route changes what it forwards.
- API: the routes keep one set field, `types`, whose enum grows; each
  direction's page sends `type_options`, the types its bridge honours,
  in order. A write takes the whole set; an unknown type or an empty
  set is refused (disabling a route is how it stops). Groups, labels
  and presets live in the app; a type the app does not know is shown
  under "Other kinds" by its name.
- The route form: presets on top ("Everything", "Messages and media",
  "Text only", "Custom", the selector always showing the preset the
  set equals), then the groups, each with a tri-state checkbox and
  "n of m", open only when mixed. "Messages and media" is Text, Media,
  Audio, Polls and games, and Places and contacts (the owner). The
  route list shows one short summary per route ("All content", a
  preset's name, "All except X, Y", or "Text, Media (4/6), Audio").
- Reasons: one set field keeps the API stable as the list grows,
  columns keep the flags readable in SQL and avoid a bit-number
  contract between C++, Go and tests, and copying each type's former
  flag keeps every existing route as it is.
