# forwarding-per-user

Redesign platform forwarding so that each configuration has an owner
and a federation, and members can set up their own.

## Why

Forwarding is a shared admin page today; the project owner wants
per-user configurations (docs/design/web-accounts.md section 6).

## Scope

The Go service serves all of it (web/docs/platform-forwarding.md):
owners, federations chosen at creation (`GET
/api/forwarding/federations`, web migration 000018), the transfer
operations with their candidates, and the "owner disabled" flag
(`owner.active`). What is left is the React app
(web/app/src/routes/forwarding.tsx):

- Ask for the federation only when `GET
  /api/forwarding/federations?chat_id=` answers more than one for the
  picked chat, offering those choices, instead of showing the select
  whenever the page's `federations` has two or more.
- Serve the transfer dialog (`ROUTE_TRANSFER_SERVED`): offer
  `transfer_candidates`, and show 409 `bot_not_owned` on a route given
  to a member that does not own its bots.

## Design notes

- The daemons keep reading the same tables; the federation lives in
  the web's database (`web_resource_federations`).

## Depends on

Nothing.

## Done when

- A member creates a route for a granted group and cannot see another
  member's; an admin transfers it; disabling its owner flags it and it
  keeps forwarding.
