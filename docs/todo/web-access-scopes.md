# web-access-scopes

Limit a member account to some pages, groups and users, enforced in
every query, not just hidden in the UI.

## Why

Accounts with permissions (web-accounts) still see the whole archive.
The project owner wants members limited to the parts they need.

## Scope

- Per member, each scope either "all" or an allowlist:
  - pages: dashboard, users, groups, group_messages,
    private_messages, files, platform_fwd, mcp;
  - group ids;
  - user ids.
- Pages outside the scope are hidden from navigation and refused by
  URL (403).
- Groups and users outside the scope are excluded from every list,
  search (`/v1/search/*`), chat view, the `/v1` chat API, file listings
  and media serving: the filtering is in the DAO queries, so no code
  path can bypass it. A file is served to a member only if a message,
  photo or preview in their scope references it.
- Editing scopes in the accounts administration and the accounts API.
- `web/docs/` updated in the same commits.
- Not in scope: MCP tokens (they keep their own allowlist).

## Design notes

- The scope check belongs in one place that every DAO query uses (as
  the MCP code gates every query on one constant), so that forgetting
  it is visible in review.
- Search totals and counts must be computed under the scope too.
- Watch query plans on the big tables (group messages): an IN-list or
  a join on an allowlist table must use the chat_id indexes.

## Depends on

- `web-accounts`

## Done when

- A member scoped to two groups sees only those groups and their
  members' messages in every page, search, chat view, API answer and
  file link; any other id answers 403 or 404; an admin still sees
  everything.
