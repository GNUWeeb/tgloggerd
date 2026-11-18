# user-photo-carousel

On a user page, the profile photo opens a carousel of the user's photo
history; a photo change in the user's history opens the same carousel
at that photo.

## Why

The project owner asked for it (2026-11-18): a user's earlier profile
photos are recorded (telegram_user_hist_profile_photo) but can only be
seen one thumbnail at a time in the History tab.

## Scope

- Go: GET /api/users/{id}/photos, every recorded profile photo of the
  user, newest first, each with its history row id, the time it was
  set, its /files/<token> URL (null when the file cannot be served)
  and its file id, and which one is the current photo; for viewers who
  see the user in full (as the history), 404 otherwise.
- Go: a photo entry of the user's history (page data and
  GET /api/users/{id}/history) carries its photo_id, the same row id,
  so the page can open the carousel at it.
- React (the frontend session): the profile photo on /users/{id} opens
  a carousel (lightbox) over those photos, starting at the current
  one; a photo change in the History tab opens it at that photo;
  keyboard and swipe navigation, the date of each photo.
- Spec, docs and tests on both sides.

## Depends on

Nothing.

## Done when

- Clicking the profile photo shows every recorded photo in a carousel,
  and clicking a photo change in the history opens it at that photo.
