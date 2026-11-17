# seed-file-access

A one-off script that raises each file's `accessed_at` from its created
time to the latest recorded sign that the file was used.

## Why

Retention evicts by `accessed_at`, but nothing recorded access before
this work, so migration 000032 starts every file at `created_at`.
Without seeding, the first sweep would evict files people did look at.

## Scope

- `scripts/seed_file_access.sql`, run once as root at the cutover
  because it reads both schemas, idempotent, never lowering
  `accessed_at` and keeping `updated_at`. Signals, as storage-cache.md
  section 9 lists them:
  - `tgloggerd_web.web_audit` rows with `action = 'media'` and
    `detail = 'file <id>'`;
  - the newest `date` of a message (group or private, live or edit)
    referencing the file;
  - Discord media forwards of a message referencing the file.
- A dry-run form (or a companion SELECT) that reports how many files
  and bytes would still be past 90 days after seeding.
- Not in scope: running it (storage-cutover).

## Design notes

- `web_audit.detail` is free text; parse ids defensively
  (`SUBSTRING_INDEX`, `CAST ... UNSIGNED`) and ignore rows that do not
  match.
- Work in batches by id range to keep transactions small on 980k rows.

## Depends on

Nothing.

## Done when

- On a throwaway database with sample rows in both schemas, the script
  raises exactly the expected rows and a second run changes nothing.
