# web-member-access

Limit a member to the groups granted to it, and show users only as
they appear in those groups.

## Why

Members see only what they are given (docs/design/web-accounts.md
section 5).

## Scope

- Per-member group grants, a subset of its federations' groups, set
  by the federation's admins or a superadmin.
- Enforcement in the scope object: lists, search and totals, chat
  views, `/v1` APIs, files and media; everything else answers 404.
- Users only through granted groups: name and photo, messages in
  those groups; no bio, name history, global counts or other groups;
  user search limited to people found in those groups.
- No private messages for members.
- Member abilities `forward` and `fetch_files` stored with the grants.
- Turn on the `group_grants` and `abilities` flags of AccountsProps.editable in
  the accounts page's loader, so the page offers what now works.

## Design notes

- Watch query plans on the big tables: the grant filter must use the
  chat_id indexes.
- The scope's hooks are in place (webapi/internal/scope):
  `GroupFilter`, `SeesGroup`, `SeesChat`, `SeesUser` and `Can` answer
  "nothing" for members today; fill them from the grants, loaded once
  per request. `logdb.GetFile` takes the scope but does not filter,
  since a file token works without a session too; decide how a
  member's grants and tokens combine.

## Depends on

- `web-federations`
- `web-admin-restrictions`

## Done when

- A member granted two groups sees only those groups and those
  groups' users, in every page, search, API answer and file link; any
  other id answers 404.
