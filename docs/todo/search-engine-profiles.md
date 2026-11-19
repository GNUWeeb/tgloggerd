# search-engine-profiles

Offer the search engine's options for users' bios and groups'
descriptions in the web app; the service side is done.

## Why

The Go service indexes bios and descriptions in the optional search
engine (webapi/README, "Bios and descriptions") and answers their
"matches" filters from it, but the React pages of users and groups do
not offer that filter or the engine's options yet.

## Scope

- The users and groups pages (UsersProps, GroupsProps): offer
  `matches` for bio and description where the field lists it, and,
  when the page data carries `engine`, its options (fuzzy, highlight;
  no facets) as the message lists do, with `backend` and `highlights`
  shown alike.
- The advanced search: the capabilities may now list users and
  groups (web/docs/search-api.md, "Search engine").

## Design notes

- The API shape is in webapi/openapi.yaml (UsersProps, GroupsProps,
  SearchEngineEntity); a member gets no users entry, since it sees no
  bio.

## Depends on

Nothing.

## Done when

- With the engine configured, the users and groups pages search bios
  and descriptions with the engine's options; without it they look
  as before, but for the new `matches` operator.
