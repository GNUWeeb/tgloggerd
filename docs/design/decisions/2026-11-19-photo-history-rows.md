# Decision log: 2026-11-19, A photo history row is a photo that became current

Decided by the TDLib session on 2026-11-19, for the profile-photo-lists
item (docs/todo/profile-photo-lists.md):

- A row of `telegram_user_hist_profile_photo` or
  `telegram_group_hist_photo` records a photo that became the user's or
  group's photo, once per file, dated when it was first seen as such,
  or with its own date when the photo lists of Telegram supply it.
- Until now a row recorded the photo being replaced, dated when it was
  replaced, and the first photo twice; the current photo had no row.
  Those rows are left as they were: they name real photos the user or
  group had, and readers deduplicate by file.
- Reason: a reader building the photo list from the history (the web's
  photo viewer) could not find the current photo, and the dates said
  when a photo ended, not when it began. Recording the new photo makes
  each row mean what a history entry is read as, and the current photo
  is in the list without a special case.
