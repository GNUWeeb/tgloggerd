# Decision log: 2026-11-20, MCP tokens go to superadmin, and who manages them

Decided with the project owner on 2026-11-20, for the
mcp-token-ownership item. It supersedes
[2026-11-18-mcp-token-owners.md](2026-11-18-mcp-token-owners.md)
(existing tokens keep the accounts that minted them) and amends
[2026-11-18-ownership-and-bots.md](2026-11-18-ownership-and-bots.md):

- Every MCP token that exists when this lands is transferred to the
  first superadmin, the account named `superadmin`, as an audited
  ownership transfer: web migration 000015 changes each token's owner
  row in `web_resource_relations` and writes one `web_audit` row per
  token (`owner_transfer` from the old owner, or `owner_set` for a
  token that had none). Without an account of that name and role the
  migration changes nothing, and the `assign-owner` command, which no
  longer treats tokens as an exception, gives the unowned tokens to
  the superadmin it is told.
- Ownership is the owner row, not `web_mcp_tokens.web_user_id`. That
  column stays the account that minted the token, the one its
  requests are attributed to: disabling the minter disables the token,
  whoever owns it.
- A token is listed, minted, renamed and revoked by its owner, by an
  admin for a member it manages (filed in a federation it
  administers, the member grants' rule), and by any superadmin. A token
  without an owner row counts as the superadmins'. Only superadmins
  transfer a token to another owner. The token list shows only the
  tokens the viewer manages; anything else is 404.

Chosen when implementing it, as the item left it open:

- Members get tokens through a new member ability, `mcp_tokens`, off
  by default and given like `forward` and `fetch_files`, rather than
  every signed-in account minting: the `/mcp` endpoint's
  `telegram_get_users` is not limited to the exposed groups and
  answers every user's profile, phone numbers included, which is more
  than a member's grants let it see on the web. Superadmins and admins
  hold it by role.
- A token's owner must be an account that may hold tokens: active, and
  a superadmin, an admin, or a member holding `mcp_tokens`. Taking
  `mcp_tokens` away from a member does not revoke its tokens; whoever
  manages them revokes them.
