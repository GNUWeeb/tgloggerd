# Decision log: 2026-11-18, MCP tokens keep their creators

Superseded on 2026-11-20 by
[2026-11-20-mcp-tokens-to-superadmin.md](2026-11-20-mcp-tokens-to-superadmin.md):
every existing token goes to `superadmin` instead.

Decided with the project owner on 2026-11-18, amending
[2026-11-18-ownership-and-bots.md](2026-11-18-ownership-and-bots.md):

- When ownership is introduced, an existing MCP token becomes owned by
  the account that minted it, which every token records, not by the
  first superadmin. Every other resource that exists then still goes
  to the first superadmin.
