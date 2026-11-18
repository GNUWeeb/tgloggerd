# search-engine-backend

An optional search engine (OpenSearch) behind the advanced search, for
the searches MySQL's indexes serve badly, with the extra options it
allows shown in the frontend only when it is configured.

## Why

MySQL's FULLTEXT indexes handle simple word searches only: no
relevance tuning, phrase proximity, typo tolerance, highlighting or
facets, a minimum token length, and slow counts on large chats. A
search engine does these well, but it costs memory and disk, so it
stays optional and the dashboard keeps working without it.

## Scope

- A second backend for the full-text fields of the advanced search's
  field registry (webapi/internal/logdb/search_entities.go); column
  and exists fields stay on MySQL. Without the engine configured,
  nothing changes.
- A capabilities answer in the search API telling the frontend which
  options the backend offers (phrase, proximity, fuzzy, highlighting,
  facets); the React application shows them only when offered, and
  `web/docs/search-api.md` documents them.
- Indexing fed from the daemon's event stream (`GET /v1/events`): new,
  edited and deleted messages, renames. A backfill job for the
  existing archive and a periodic reconcile that repairs what the
  stream missed (resync, downtime).
- If the engine is down or behind, search falls back to MySQL and
  says so.
- The engine's container, configuration in the environment and its
  memory and disk needs documented.
- Left out: replacing MySQL as the source of truth; search across
  other platforms' tables.

## Design notes

- OpenSearch, not Elasticsearch: OpenSearch is Apache-2.0, while
  Elasticsearch's licences (AGPL, SSPL, ELv2) sit awkwardly beside the
  project's GPL-2.0-or-later. The engine runs as a separate service
  and is reached over HTTP, behind an interface so another engine can
  replace it.
- Every query carries the request scope of web-accounts.md section 9
  as a filter on the visible chat ids, applied by the engine, never
  after it: hit counts, facet totals and highlights must not reveal
  hidden groups or private messages.
- Index size is roughly the text's own size; plan the disk with the
  file store's cap in mind.
- Decision: [design/decisions/2026-11-18-search-engine.md](../design/decisions/2026-11-18-search-engine.md).

## Depends on

- `web-member-access`

## Done when

- With the engine configured, phrase, proximity and fuzzy searches
  work and the frontend shows their options; without it, the search
  behaves as before and shows none of them.
- A message created, edited or deleted is found, changed or gone in
  search within seconds, and the reconcile repairs a deliberately
  missed event.
- Tests show that an admin with hidden groups and a member see no
  hits, counts or facets from chats outside their scope.
- Stopping the engine makes search fall back to MySQL.
