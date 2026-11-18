# Decision log: 2026-11-18, Ownership of existing resources, bots per account

Decided with the project owner on 2026-11-18, for
[../web-accounts.md](../web-accounts.md) section 6:

- Accounts are separated: every resource that can be owned (MCP
  exposed groups, MCP tokens, Telegram bots, Discord bots and the
  forwarding configurations in both directions) has an owner.
- Everything that exists when ownership is introduced becomes owned by
  the first superadmin, the account named `superadmin`, which the
  owner creates.
- Discord bot tokens are kept in the database, not in the
  environment, like Telegram bot tokens. Every account may have its
  own Telegram bots and Discord bots, each owned by it; tokens are
  write-only and never shown again once saved.
- The Discord bot token that is in the environment today moves into
  the database, owned by `superadmin`, and is then removed from the
  environment.
