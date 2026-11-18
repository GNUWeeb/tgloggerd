# parity-people

Bring the Go user and group page data, and the list pages' search,
up to the Drogon pages.

## Why

The parity audit (2026-11-18) found profile fields, username detail,
history paging and much of the advanced search missing from the new
stack; it is deployed only at parity.

## Scope

- User page: birthday, personal chat, paid stars, emoji status,
  accent colours, restriction, sensitive and restricts-chats flags,
  mutual contact, the own-name and lookup-pending state; all
  usernames with kind and collectible, for users and groups; links to
  full-size photos. Within the scope (a member sees name and photo
  only, web-accounts.md section 5).
- History pages paged by cursor instead of the 200-entry cap.
- List pages: every field, operator (OR, NOT LIKE, IS NULL, >=, <=)
  and sort the Drogon search offers, through the existing search
  engine; `total_exact` and the timeout state in the list state;
  `rows_per_page` applied as the default and page sizes up to the
  Drogon maximum; errors for bad filters; old `?search=`, `offset`
  and `limit` URLs translated.
- webapi/openapi.yaml exact for everything added.

## Depends on

Nothing.

## Done when

- Every user, group and list row of the audit is served, tested with
  dummy data, and the contract test passes.
