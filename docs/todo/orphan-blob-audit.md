# orphan-blob-audit

Find blobs in the store that no row describes, and remove them or
count them toward the cap.

## Why

`storeDownloadedFile` copies the bytes before it inserts the row; when
the insert fails (the database down, an over-long extension), the blob
stays with no row. The cap counts rows, so such blobs are invisible to
it, and they piled up to 52.9 GiB once before.

## Scope

- A scan of the store that lists blobs without a matching
  `telegram_files` row (sha256 and extension), as a report first.
- Deleting them only with the owner's agreement.

## Design notes

- Found while planning the storage cache; the 2026-10 cleanup removed
  9,950 such files by hand.

## Depends on

Nothing.

## Done when

- The report runs and its numbers are agreed with the owner; the agreed
  action is done.
