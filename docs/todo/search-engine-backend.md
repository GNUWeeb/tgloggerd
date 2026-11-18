# search-engine-backend

The React application's side of the optional search engine: the extra
search options of the message searches, shown only when the engine
offers them.

## Why

The Go service answers the group and private message searches' text
from OpenSearch when it is configured (webapi/README, "Search
engine"), with phrase, proximity, prefix and fuzzy matching,
highlighting and per-chat facets. The frontend does not offer any of
it yet, so the engine's options are reachable through the API only.

## Scope

- Read `GET /api/search/capabilities` and offer, for the entities it
  lists as available, the options it names: a fuzzy toggle
  (`fuzzy=1`), highlighted fragments (`highlight=1`, the answer's
  `highlights` aligned to `rows`), per-chat facets (`facets=1`) and a
  short help on the `matches` syntax (`"a b" @3`, `word~`, `word*`).
  Without an engine, or while it is not available, show none of them
  and behave as before.
- Show when MySQL answered instead (`backend.fallback`) and which
  options were ignored (`backend.ignored`).
- Left out: the Go side, the compose service, the indexer and the
  docs, which are done (web/docs/search-api.md, "Search engine");
  indexing bios and group descriptions, which comes later
  (design/decisions/2026-11-18-search-engine.md).

## Design notes

- The contract is `searchCapabilities` and `searchEntity` in
  webapi/openapi.yaml; web/app/src/api/generated/schema.d.ts is
  regenerated from it.
- Highlights are HTML-escaped with the matches in `<mark>...</mark>`;
  service messages never get any.
- Facets come only when the engine answers the whole search
  (conditions joined by AND over the text and the carried columns);
  otherwise `backend.ignored` lists them.
- Decision: [design/decisions/2026-11-18-search-engine.md](../design/decisions/2026-11-18-search-engine.md).

## Depends on

Nothing.

## Done when

- With the engine configured, the message searches offer its options
  and show highlights and facets; without it, they behave as before
  and show none of them.
