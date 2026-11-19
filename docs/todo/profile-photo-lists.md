# profile-photo-lists

Fetch a user's or a group's profile photos from Telegram when its page
opens, so the photo viewer starts at the current photo and lists every
photo, and stream a photo the store no longer holds from Telegram
instead of showing a broken picture.

## Why

The photo history is recorded only as the daemon sees changes, so it
can miss photos: a user whose current photo has no history entry gets
a viewer that opens on an older one. Photos evicted from the store
have no link at all, and pages show them broken.

## Scope

- The daemon lists a user's profile photos (getUserProfilePhotos) and
  a group's (its current photo and its photo changes) on request,
  records the history entries it lacks with their real dates, and
  stores each photo's file row; bytes only when asked for.
- The Go service asks for that list in the background when a user or
  group page opens (at most every ten minutes per chat), answers the
  photo list with the current photo first, and gives a photo whose
  bytes are not stored a /files/ link that fetches it from Telegram
  on the fly, as other unstored files are fetched.
- The React application's photo viewer opens at the current photo and
  shows the fetched photos; a picture that cannot be loaded shows a
  placeholder, never a broken image.

## Depends on

Nothing.

## Done when

- A user whose current photo was missing from the history opens the
  viewer on it, followed by the older photos newest first.
- A photo no longer in the store shows, fetched from Telegram.
- The same holds for groups.
