# photo-download-index

Stop re-downloading user and group photos that are already stored.

## Why

`maybe_download_profile_photo` and `maybe_download_group_photo` do not
consult the file index that message media uses. With TDLib's copy
pruned after each store, every `updateUser`, `getUser`, `updateNewChat`,
`getChat` or `updateChatPhoto` that sees a photo downloads and stores it
again.

## Scope

- Look the photo's remote id (or `tg_unique_id`) up before downloading,
  and link the existing row instead.
- Not in scope: anything else in the photo paths.

## Design notes

- Found while planning the storage cache (TDLib.cpp around
  `maybe_download_profile_photo` and `maybe_download_group_photo`).

## Depends on

- `files-cache-schema`

## Done when

- A photo that is already stored is linked without a download; the
  count of photo downloads per hour drops at deploy.
