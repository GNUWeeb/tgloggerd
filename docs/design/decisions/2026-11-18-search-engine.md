# Decision log: 2026-11-18, Optional search engine

Decided with the project owner on 2026-11-18, for the item
search-engine-backend (built; bios and descriptions are left to
[../../todo/search-engine-profiles.md](../../todo/search-engine-profiles.md)):

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

Settled for the first version:

- The image is opensearchproject/opensearch:3.8.0, pinned by digest;
  single node, a 2 GB heap in a container of about 4 GB, the index
  under data/opensearch.
- Its security plugin is off: it runs on the compose network only,
  with no published port and no route through nginx.
- It indexes the group and private messages' text (captions
  included); bios and descriptions come later.
- Without its configuration nothing changes.
