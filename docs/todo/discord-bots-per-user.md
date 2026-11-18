# discord-bots-per-user

Keep Discord bot tokens in the database, one or more bots per account,
and have discordd run every enabled bot, instead of the single token in
the environment.

## Why

The Discord bot token lives in `.env` (DISCORD_BOT_TOKEN), so there is
one bot for the whole service and changing it means editing the
environment and recreating the container. Accounts are to be separate,
each with its own Telegram and Discord bots
(decisions/2026-11-18-ownership-and-bots.md).

## Scope

- A daemon migration (additive) adding a Discord bots table beside
  `telegram_bots`: name, token, enabled, status as discordd last saw
  it; Discord to Telegram routes refer to the bot that reads them.
- discordd loads the enabled bots from the database, runs one gateway
  connection per bot, reloads on changes as it reloads routes, and
  records each bot's state (connected, intent missing, token refused);
  a bot that Discord refuses is disabled, not retried in a loop (the
  IDENTIFY budget).
- The Go service's operations and pages to add, rename, enable and
  remove Discord bots, owned through the ownership helpers
  (webapi/internal/webdb/ownership.go, resource type `discord_bot`,
  reserved): tokens write-only, validated in shape, never answered
  back; a member with `forward` sees and uses only its own bots, as for
  Telegram bots. The new bots table also gets its line in the
  assign-owner command (webapi/cmd/webapi/assign.go, `ownable`).
- Import: at deployment, the token in DISCORD_BOT_TOKEN becomes a bot
  owned by `superadmin`, and the existing routes refer to it; then the
  variable is removed from `.env` and `.env.example`.
- `web/docs/platform-forwarding.md` and the discordd docs updated.

## Design notes

- Tokens are stored as the Telegram bot tokens are today. Whether to
  encrypt bot tokens at rest (both kinds) is a separate decision.
- The gwdiscord library keeps one connection per client; several bots
  means several clients in discordd, each with its own rate limits.
- Migration number: the next free daemon one when this starts.

## Depends on

Nothing.

## Done when

- Two accounts each run their own Discord bot and route through it,
  neither sees the other's bot, the environment holds no Discord
  token, and tests cover the table, the reload and the refusals.
