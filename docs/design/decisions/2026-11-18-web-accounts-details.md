# Decision log: 2026-11-18, Web accounts details

Settled while the redesign mockup was adjusted to
[../web-accounts.md](../web-accounts.md):

- Sessions get a server-side table (session id in the cookie, device,
  address, signed-in and last-active times), so an account can list
  its sessions and sign out one, or all others. While the Drogon web
  still serves pages, its AuthFilter checks the same table, so a
  signed-out session ends on both services.
- The Default federation's groups stay derived (groups in no other
  federation), but Default is a stored federation row so that admins
  and members can be assigned to it.
- A forwarding configuration's federation is chosen when it is created
  only if more than one of the owner's federations holds its group;
  otherwise it is the one that does.
- Transfer candidates: for an admin, the configuration federation's
  admins and its members holding `forward`; for a superadmin, any
  active account.
- An admin's accounts list shows only the members of its federations;
  a superadmin sees everyone, with a role filter.
- Page keys for the menu: dashboard, users, groups, group_messages,
  private_messages, files, platform_fwd, join_chat, mcp, accounts,
  federations; settings for every account. Pinned groups show only
  while they are in the viewer's scope.
