# Decision log: 2026-11-18, The janitor reports by default

Decided by the coordinator on 2026-11-18, for
[../storage-cache.md](../storage-cache.md) section 9:

- `TG_STORAGE_DRY_RUN` defaults to 1. The janitor only reports what it
  would evict until the variable is set to 0.
- Reason: the deployed containers rebuild and apply migrations from
  the checkout on every restart, so merged work goes live at the next
  restart, before the cutover. With eviction on by default and 90 days
  of retention, a restart could delete hundreds of GiB of archived
  bytes before the access times are seeded and before the project
  owner has seen the numbers.
