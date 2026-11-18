# search-engine-profiles

Index users' bios and groups' descriptions in the optional search
engine, beside the messages it indexes already.

## Why

The first version of the search engine indexes messages only; bios
and descriptions were left for later
(design/decisions/2026-11-18-search-engine.md), so the users' and
groups' full-text fields still search with MySQL's FULLTEXT.

## Scope

- Documents for users (bio) and groups (description), kept in step
  like the messages: renames and profile changes from the event
  stream, a backfill and the reconcile.
- The users and groups searches answer their full-text fields from
  the engine, with the request scope as a filter inside the query
  (members see only the users and groups of their granted groups).
- Capabilities and the lists' engine entry name the new entities.

## Design notes

- Follows the messages' design in webapi/README, "Search engine", and
  web/docs/search-api.md; a member's user filter (scope.UserFilter)
  has to become an engine filter, which needs the documents to carry
  the groups a user appears in, or the search to take ids from MySQL.

## Depends on

Nothing.

## Done when

- With the engine configured, bios and descriptions are found with
  the engine's options, within the scope; without it nothing changes.
