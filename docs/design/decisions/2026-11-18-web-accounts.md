# Decision log: 2026-11-18, Web accounts and federations

Decided with the project owner on 2026-11-18, for
[../web-accounts.md](../web-accounts.md):

- Three roles: superadmin (creates superadmins and admins), admin
  (created by a superadmin, restricted per admin), member (created by
  an admin or superadmin).
- Federations: named sets of groups defined by superadmins. They limit
  what an admin manages, not what it sees. Groups and members may
  belong to several federations. Groups in no federation form the
  Default federation, including newly joined ones.
- Only superadmins change a federation's groups, except that an admin
  given `join_chat` may add a group it joins to its own federations.
- A superadmin can hide private messages and specific groups from an
  admin; hidden things answer 404.
- A member sees users only through its granted groups.
- Forwarding configurations are owned per user; members see only their
  own, admins see all except those touching groups hidden from them,
  and admins transfer ownership. A disabled owner's configurations
  keep running, flagged for transfer.
- Ownership is a generic relation table so that more relations can be
  added later.
- Per-user settings, starting with timezone.

Proposed by the coordinator and part of the design unless the owner
objects: the last active superadmin is protected; "Look up real name"
is superadmin-only; accounts are disabled, never deleted; members never
see private messages; Telegram bots are owned per user.
