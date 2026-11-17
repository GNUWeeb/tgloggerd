# orphan-blob-audit

Act on what the store audit reports, once the owner has agreed, and
optionally count orphan blobs toward the cap.

## Why

`storeDownloadedFile` copies the bytes before it inserts the row; when
the insert fails (the database down, an over-long extension), the blob
stays with no row. The cap counts rows, so such blobs are invisible to
it, and they piled up to 52.9 GiB once before.

## Scope

- Run `scripts/audit_store.py` read-only against production (its
  header says how) and show the owner the numbers: orphan blobs by
  age, extension mismatches, blobs left by `on_disk = 0` rows, rows
  whose blob is missing, empty directories and misnamed entries.
- Once the owner agrees, act on the lists it wrote and on nothing
  else: for example delete orphans older than a day that show up in
  two runs, unlink blobs left by `on_disk = 0` rows, and remove empty
  fan-out directories. Missing blobs need no action beyond the count
  (the web treats them as files to fetch); their rows could be set to
  `on_disk = 0` if the owner wants the cap total to be true.
- Optionally: count orphan bytes toward `TG_STORAGE_MAX_BYTES`, or
  have the StorageJanitor report them.

## Design notes

- Found while planning the storage cache; the 2026-10 cleanup removed
  9,950 such files by hand.
- The audit itself is done (`scripts/audit_store.py`, report only). It
  reads the rows first and walks the store second, so a blob stored
  during the run shows up as an orphan in the "after snapshot" bucket;
  act only on entries reported by two runs, or older than a day.
- The tgloggerd image has neither python3 nor a mysql client; run the
  audit on the host with the rows saved by the database container's
  client, as the script's header shows.

## Depends on

Nothing.

## Done when

- The report's numbers are agreed with the owner and the agreed
  action is done.
