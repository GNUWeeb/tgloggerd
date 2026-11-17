# web-admin-restrictions

Let a superadmin hide private messages and specific groups from an
admin, enforced everywhere with 404.

## Why

A superadmin needs to keep some data from some admins
(docs/design/web-accounts.md section 4).

## Scope

- Per-admin restriction list: hide private messages, hide groups,
  and the `join_chat` grant (off by default).
- Enforcement in the request scope object and every query it guards:
  navigation, pages, `/v1` APIs, search and counts, chat views, file
  links and media, MCP exposure lists, forwarding lists, dashboard,
  audit entries. Hidden things answer 404.
- An admin cannot grant what it cannot see.
- Not in scope: members (web-member-access).

## Design notes

- One scope object used by every DAO query, as the MCP code gates
  every query on one constant.

## Depends on

- `web-roles`
- `go-port-chat-api`
- `go-port-search`

## Done when

- An admin with private messages hidden sees no private-message menu
  and gets 404 on every private-message URL and API; a hidden group is
  absent from every list and search and its URLs answer 404.
