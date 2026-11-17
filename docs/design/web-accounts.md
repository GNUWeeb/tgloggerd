# Web accounts: roles, federations, ownership and settings

Status: designed 2026-11-18 with the project owner, not implemented.
The items that implement it are in the "Web interface" section of
[../TODO.md](../TODO.md); the decisions are in
[decisions/2026-11-18-web-accounts.md](decisions/2026-11-18-web-accounts.md).

## 1. Goal

Built in the Go web service of [web-go.md](web-go.md).

The web has one shared admin login. It becomes a multi-user
dashboard: several accounts with roles, groups of Telegram chats
administered by different people (federations), forwarding
configurations owned by the account that set them up, and settings
per account.

## 2. Roles

Every account has exactly one role.

| Role | Can do | Created by |
|---|---|---|
| superadmin | everything: all accounts, roles, permissions, federations | a superadmin |
| admin | everything a superadmin can, minus the restrictions a superadmin sets on it (section 4), and managing accounts only within its federations (section 3) | a superadmin |
| member | only the groups and abilities granted to it (section 5) | an admin of one of its federations, or a superadmin |

- The last active superadmin cannot be disabled, deleted or demoted.
- Every account change (create, edit, role, restriction, grant,
  disable, password reset, sign-out-everywhere, ownership transfer) is
  audited with who did it.
- Accounts are disabled, never deleted, so audit and ownership keep
  pointing at them.
- Migration of today's accounts: `admin` becomes superadmin, `viewer`
  becomes a member of the Default federation with no groups granted.

## 3. Federations

A federation is a named set of Telegram groups, defined by a
superadmin, with admins and members assigned to it.

- Federations limit what an admin can manage, not what it can see.
  What an admin sees is limited only by its restrictions (section 4).
- An admin manages, within its federations: the members of those
  federations, the groups it grants them (only groups of those
  federations), and the forwarding configurations of those
  federations. It never manages admins or superadmins.
- A group may be in several federations, and a member may belong to
  several; each federation's admins manage that member's grants within
  their own federation. Every change is audited.
- The Default federation holds every group that is in no other
  federation, including every group the account joins later. It is
  derived (a group in no federation is in Default), not stored.
  Superadmins decide who administers it.
- Only superadmins change which groups a federation holds, with one
  exception: an admin holding the `join_chat` permission who joins a
  group (Join chat page) may add that group to federations it
  administers, chosen as chips on the join confirmation; choosing none
  leaves the group in Default.

## 4. Admin restrictions

A superadmin can restrict an admin, per admin:

- hide private messages: no private-message menu, page or API; direct
  URLs answer 404;
- hide specific groups: the group, its messages, members, files,
  forwarding configurations and MCP exposure are invisible to that
  admin; direct URLs answer 404;
- withhold `join_chat` (granted per admin, off by default).

Restrictions are a list, so more can be added later. An admin can
never grant what it cannot see.

Admin-only actions that act on the shared Telegram account: Join chat
(admins with `join_chat`, and superadmins), MCP exposure (only groups
the admin can see), the store audit, and "Look up real name", which
changes the account's contacts and is for superadmins only.

## 5. Members

- A member sees only the groups granted to it, a subset of its
  federations' groups. Everything else answers 404.
- A member sees users only as they appear in its groups: name and
  photo, and their messages in those groups. No bio, name history,
  global counts or list of a user's other groups. User search returns
  only people found in those groups.
- Members never see private messages.
- Abilities a member can be given: `forward` (set up forwarding,
  section 6) and `fetch_files` (fetch unstored files from Telegram).

## 6. Ownership and per-user forwarding

Ownership is generic, so more relations can be added later:

    web_resource_relations (resource_type, resource_id, user_id,
                            relation, created_at, created_by)

`relation` is `owner` today; `editor` or `viewer` (sharing) can be
added without touching the resource tables. The forwarding tables live
in the logger's database and the accounts in the web's, so there is no
foreign key between them; the web keeps the two consistent.

Forwarding configurations (Telegram to Discord webhooks, Discord to
Telegram routes, and their Telegram bots) each have an owner and a
federation:

- A member with `forward` creates configurations for its own groups
  only, sees only its own, and cannot see anyone else's.
- An admin sees every configuration except those touching a group
  hidden from it (404), and can transfer ownership between accounts of
  its federations; a superadmin sees and transfers all.
- Telegram bots are owned too: pasting a bot token never reveals or
  reuses another owner's bot.
- When an owner is disabled, its configurations keep forwarding and
  are flagged "owner disabled" for an admin to transfer.

## 7. Per-user settings

One key-value table of known keys, so a new setting is cheap:

- timezone, date and time format (24 h or 12 h, ISO);
- UI language (English, Indonesian);
- theme (light, dark, system);
- rows per page; landing page;
- chat view: auto-load and live updates, media loading (automatic or
  on click), show deleted messages, edits and service messages;
- pinned groups and saved searches;
- a "my sessions" page with sign-out.

Later: TOTP two-factor sign-in, per-user MCP tokens.

## 8. Email

Every account has an email address.

- A new account gets a one-time set-password link by email (it expires
  after a configured time); no password is ever sent by email, and the
  administrator never sees it. The same link mechanism resets a
  forgotten password.
- Addresses are verified by a link before they are used for resets or
  notices. Changing an address notifies the old one.
- Accounts that have no address yet (today's) must add and verify one
  right after their next sign-in, before using the dashboard.
- Security notices, always sent: password changed or reset, address
  changed (also to the old address), sign-in from a new device, account
  disabled, role changed.
- Mail goes out through an SMTP relay (STARTTLS or TLS), configured in
  the environment (host, port, user, password, sender); nothing in the
  code is provider-specific. Sending is asynchronous with retries and
  never blocks a request; every message sent is logged without its
  links.
- Links are single-use, stored only as hashes, and expire.

## 9. Enforcement

Visibility is enforced in the database queries, through one scope
object built per request from the account (role, federations,
restrictions, grants), used by every list, search, chat view, `/v1`
API, file listing, media request, MCP admin list, forwarding list and
dashboard count, so that a forgotten check is visible in review.
Hidden things answer 404, never 403, so their existence does not leak.
