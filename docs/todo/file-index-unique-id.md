# file-index-unique-id

Look message media up in the file index by TDLib's unique id too, and
load metadata-only alias rows into the index as their target.

## Why

The in-memory file index that keeps ingest from downloading a known
file again is keyed by remote id (`tg_file_id`) only. A file's remote
id can differ between messages for the same file, so the index misses
and the file is downloaded and hashed again, to be deduplicated by its
sha256 only after the download. Photos already look up by unique id
too (photo-download-index); message media does not.

Also, `DB::loadFileIndex` loads a metadata-only row that has
`alias_of` set as metadata-only under its own remote id, so after a
restart a member chat's message with that file downloads it once more
before the store corrects the in-memory entry.

## Scope

- Index message media by `tg_unique_id` as well as by remote id, and
  consult it in `maybe_download_message_file` and
  `maybe_store_link_preview`.
- `loadFileIndex` follows `alias_of` to the row holding the bytes, as
  `loadPhotoUniqueIndex` does.
- Not in scope: changing what is downloaded (membership and size rules
  stay as they are).

## Design notes

- Found while building photo-download-index; see
  `known_photo_uids_` and `DB::loadPhotoUniqueIndex` for the pattern.
- Memory: one more map entry per row with a unique id (about a million
  rows); measure the daemon's RSS before and after.

## Depends on

Nothing.

## Done when

- A file posted again under a different remote id links to its row
  without a download; the daemon's memory grows by an acceptable,
  measured amount.
