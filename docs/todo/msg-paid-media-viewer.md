# msg-paid-media-viewer

Let a paid media message's items open: photos in the viewer, videos in
the player, as an album would.

## Why

A paid media message shows its star count and a strip of small thumbs,
but the items are not clickable, their `media.url` is never used, a
video item cannot be played, and an item of kind `unsupported` shows
the same star icon as a photo without a thumb.

## Scope

- React (frontend): the items of a paid media detail drawn with the
  chat's media components (Picture, VideoBox) as an album, opening the
  viewer and playing videos; an item that was not paid for keeps its
  lock and blurred preview; an `unsupported` item gets its own icon and
  title. The same on the message page.
- Go: nothing (the items already carry their media).

## Depends on

Nothing.

## Done when

- In a paid media message, a paid-for photo opens in the viewer and a
  video plays; an unpaid item shows its lock; an unsupported item says
  so.
- React tests cover the three.
