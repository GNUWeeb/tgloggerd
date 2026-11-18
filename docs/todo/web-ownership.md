# web-ownership

A generic ownership table: which account owns which resource, built
to take more relations later.

## Why

Forwarding configurations become per-user, and more ownership details
may follow (docs/design/web-accounts.md section 6).

## Scope

- `web_resource_relations (resource_type, resource_id, user_id,
  relation, created_at, created_by)` in the web DB, `relation` =
  `owner` today; helper API to read, set and transfer ownership,
  audited.
- The owned resource types: MCP exposed groups, MCP tokens, Telegram
  bots, Discord bots, Telegram to Discord webhooks and Discord to
  Telegram routes (decision 2026-11-18-ownership-and-bots.md).
- A one-off command, run at deployment, that gives every resource
  without an owner to a named account (`superadmin`, the first
  superadmin), idempotent and audited; documented in web/docs/.
- New resources are owned by the account that creates them.
- Not in scope: applying it to forwarding (forwarding-per-user) and
  to Discord bots (discord-bots-per-user).

## Design notes

- Resources live in the logger's DB and accounts in the web's: no
  foreign key across them; the web keeps them consistent and treats a
  resource with no owner as owned by superadmins.

## Depends on

Nothing.

## Done when

- Ownership can be set, read and transferred for any resource type,
  each change audited, with tests.
