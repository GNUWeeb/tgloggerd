# Decision log: 2026-11-18, Optional search engine

Decided with the project owner on 2026-11-18, for the item
[../../todo/search-engine-backend.md](../../todo/search-engine-backend.md):

- MySQL stays the source of truth and the default search. A search
  engine is an optional second backend for full-text fields; the
  frontend shows the extra search options only when it is configured.
- The engine is OpenSearch (Apache-2.0) rather than Elasticsearch,
  whose licences sit awkwardly beside GPL-2.0-or-later.
- The index is fed from the daemon's event stream, with a backfill and
  a periodic reconcile.
- The request scope is enforced inside the engine's queries, so hits,
  counts and facets never reveal what the viewer cannot see.
- Built after the search port and the member access rules.
