# web-federations

Federations: named sets of Telegram groups, with admins and members
assigned, that limit what an admin manages.

## Why

Different people administer different groups
(docs/design/web-accounts.md section 3).

## Scope

- Tables for federations, their groups, admins and members (a group
  and a member may be in several); the Default federation is a stored
  row (so admins and members can be assigned to it) whose groups are
  derived as "groups in no other federation".
- Superadmin pages and API to create, rename and delete federations,
  set their groups, admins and members.
- Admin management limited to its federations: members of its
  federations, grants within its federations' groups.
- Not in scope: Join chat's federation chips (join-chat-federations).

## Design notes

- Federations limit management, not visibility (decision
  2026-11-18-web-accounts.md).

## Depends on

- `web-roles`

## Done when

- An admin of federation A manages A's members and grants only A's
  groups; a group in no federation shows under Default.
