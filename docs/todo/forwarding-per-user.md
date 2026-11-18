# forwarding-per-user

Redesign platform forwarding so that each configuration has an owner
and a federation, and members can set up their own.

## Why

Forwarding is a shared admin page today; the project owner wants
per-user configurations (docs/design/web-accounts.md section 6).

## Scope

- Owner and federation for Telegram to Discord webhooks, Discord to
  Telegram routes and Telegram bots. Owners are recorded already
  (webapi/internal/webdb/ownership.go: set on creation, answered in
  the lists, TransferOwner and OwnedBy ready), and a member with
  `forward` already sees and changes only the configurations it owns,
  in its granted groups (webapi/internal/server/forwarding.go,
  fwdVisible); what is left is the federation and the transfer
  operations.
- A member with `forward` creates configurations for its granted
  groups only and sees only its own; bot tokens never reveal or reuse
  another owner's bot.
- Admins see all configurations except those touching a group hidden
  from them (404), and transfer ownership within their federations;
  superadmins see and transfer all.
- A disabled owner's configurations keep running, flagged "owner
  disabled".
- A configuration's federation is asked for at creation only when more
  than one of the owner's federations holds its group. Transfer
  candidates: for an admin, the federation's admins and its members
  holding `forward`; for a superadmin, any active account.
- Existing configurations go into the Default federation; their
  owner, the superadmin, is given by `webapi assign-owner` at
  deployment (web/docs/accounts.md, Ownership).
- `web/docs/platform-forwarding.md` updated.

## Design notes

- The daemons keep reading the same tables; only the web changes,
  unless an owner or federation column in the logger DB proves
  simpler (decide when building, additive migration).

## Depends on

Nothing.

## Done when

- A member creates a route for a granted group and cannot see another
  member's; an admin transfers it; disabling its owner flags it and it
  keeps forwarding.
