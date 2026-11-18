# go-port-admin

Port the administration endpoints to the Go service: forwarding,
Join chat, contact lookup and MCP administration.

## Why

They become JSON APIs for the React application, and forwarding is
being redesigned per user anyway (forwarding-per-user).

## Scope

- Platform forwarding (both directions), Join chat, "Look up real
  name", MCP groups and tokens, as the Drogon controllers and
  `web/docs/` describe, as JSON APIs with CSRF, audit and the role
  rules of docs/design/web-accounts.md.
- The accounts and federations pages' loaders (AccountsProps and the
  federations page props), sending `editable` with every flag false
  until the items that build those parts turn theirs on.

## Depends on

Nothing.

## Done when

- Every admin action works through the Go API with the same effects
  and audit rows as the Drogon pages.
