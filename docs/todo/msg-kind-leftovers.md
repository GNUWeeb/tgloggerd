# msg-kind-leftovers

Close the small gaps in how the chat and the message page draw the
kinds that already have a detail: placeholders that say what they
stand for, a fallback for a kind whose detail is missing, and the
fields each card leaves out.

## Why

Each gap is small, but together they leave cards that say less than
the archive knows or nothing at all:

- an expired message says "Self-destructing media that has expired"
  for every kind, though `tdlib_type` says which;
- an `unknown` message's page shows "(no text)", as if it were an empty
  text message;
- a kind whose detail is null (a poll whose poll was never stored, an
  old location or contact without `content_extra`) draws an empty
  bubble;
- a game shows no `short_name` and its animation cover never plays; an
  invoice's photo does not open; a contact's avatar ignores the user's
  photo; a venue shows neither its type nor a link to its provider; a
  poll option shows a percentage but not its voters; a checklist hides
  whether others may add or mark tasks; a story shows no link;
- a video note whose media has no `render` falls through `kindOf` and
  draws nothing;
- `unknown` shares the document icon.

## Scope

React (frontend), in routes/chat-kinds.tsx, routes/chat.tsx,
routes/message.tsx, routes/chat-media.tsx and ui/media.tsx:

- Expired: "Expired photo", "Expired video", "Expired video message",
  "Expired voice message" from `tdlib_type`.
- `unknown` on the message page: the chat's wording, naming
  `tdlib_type` when known.
- Detail null: one line per kind with its icon ("Poll (its options
  were not recorded)"), in the chat and on the message page.
- Game: `short_name`; an animation cover plays (its `photo.url`).
  Invoice: the photo opens in the viewer. Contact: the avatar gets the
  user's photo and id. Venue: `venue_type` as a sub line, a link to
  the provider's page for Foursquare and Google Places venues with a
  `venue_id`. Poll: each option's voter count as its title. Checklist:
  "others can add tasks" / "others can mark tasks done".
- Story: a `t.me/<username>/s/<id>` link when the poster has a public
  username, else the id as text. Go (webapi): `StoryDetail` gains the
  poster's username (when the viewer may see the poster).
- `kindOf` maps `video_note` to the round video.
- A distinct icon for `unknown`.
- Left out: a contact's vCard (`msg-contact-vcard`), paid media
  (`msg-paid-media-viewer`), edits (`msg-edit-detail`).

## Design notes

Old rows: the `unknown` rows and the location or contact rows without
detail are mostly the rows logged before classification; after the
repair pass most `unknown` rows have a real kind, and the fallbacks
here cover what stays (`msg-repair-census`). Whether any video note
reaches the web without `render` was not confirmed; the mapping costs
nothing either way.

## Depends on

Nothing.

## Done when

- React tests cover each case above: an expired video message, an
  `unknown` message page, a poll with a null detail, a game with an
  animation cover, a contact with a photo, a venue with a provider, a
  story with and without a username, and a video note without
  `render`.
- The Go test of StoryDetail covers the username and a poster the
  viewer may not see.
