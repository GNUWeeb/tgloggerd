# Web accounts and roles

The web has several accounts, each with one role
([../../docs/design/web-accounts.md](../../docs/design/web-accounts.md)
section 2). This page describes what is built: the roles, the request
scope they produce in the Go service (`webapi/`), and how the Drogon
pages treat them while they still serve.

## Roles

| Role | Sees | May do | Manages |
|---|---|---|---|
| superadmin | everything | everything | every account, any role |
| admin | every group, private chat and user | all but Join chat and "Look up real name" | the members of its federations |
| member | no group, private chat or user yet | nothing beyond seeing | nobody |

- A superadmin can hide private chats from an admin
  (`restrict.hide_private` on PATCH /api/accounts/{id}, the
  "Restrictions" section of the account's Access tab). From the
  admin's next request on, the private-messages menu entry is gone,
  every private page, chat window, message and search answers 404,
  and private chats drop out of the chat search, the user pages and
  the dashboard; the scope reads it from the account with the role.
  It is cleared when the account stops being an admin, and each change
  is audited (account_update, "hide_private on" or "off").
- A superadmin can also hide groups from an admin
  (`restrict.hide_groups`, a list of group ids; the "Hide groups" picker
  on the same tab). From the admin's next request on, those groups are
  in no list, search, count, chat search, dashboard card, user page or
  federation card, their pages, chat windows and messages answer 404,
  and a message elsewhere that was posted as one, forwarded from one
  or replies from one does not name it ("(hidden chat)"). Exposed
  groups stay public. The list is stored per admin
  (web_admin_hidden_groups), read with the role on every request,
  dropped when the account stops being an admin, and audited
  ("hide_groups <before>-><after>").
- The `join_chat` permission comes with `web-admin-restrictions`; until
  then an admin may not join chats (it is off by default).
- A member sees only the groups granted to it, and users only as they
  appear there. Grants come with `web-member-access`; until then a
  member sees no group, and every group, user, private chat and file
  listing answers it as if empty, every id 404.
- Federations limit what an admin manages, never what it sees (see
  Federations below): an admin manages the members filed in a
  federation it administers.

Web migration 000004 turned today's `admin` accounts into superadmins
and `viewer` accounts into members. Web migration 000006 made every
admin an admin of the Default federation and filed every member in
it, so an admin still managed every member until federations were set
up.

## Federations

A federation is a named set of groups with admins and members
([../../docs/design/web-accounts.md](../../docs/design/web-accounts.md)
section 3), stored in the web's own schema: `web_federations`, its
groups (`web_federation_groups`), admins (`web_federation_admins`) and
members (`web_federation_members`). A group and a member may be in
several.

- The Default federation is the row with id 0. Its groups are
  derived, never stored: every group in no other federation,
  including groups joined later, and a group taken out of its last
  federation is back in Default. Admins and members are assigned to
  it like to any other.
- A superadmin administers every federation without being listed.
  An admin administers those it is listed in, Default included only
  when listed; a new admin administers none until a superadmin says
  so (`admin_of`, or a federation's `admin_ids`).
- An admin manages the members filed in at least one federation it
  administers, and changes their federations only among its own; a
  member's other federations stay as they are. A member in no
  federation is managed by superadmins alone.
- Federations change no visibility: an admin sees every group whatever
  its federations (restrictions come with `web-admin-restrictions`),
  and a member's groups come from its grants (`web-member-access`).
- The scope reads an admin's federations from the account on every
  request, with its role, so a change applies to its next request.

The federations API (webapi/openapi.yaml, tag accounts) is the
superadmins', except that an admin renames and describes the
federations it administers (`PATCH` with `name` and `description`
only; `group_ids` or `admin_ids` from it answer 403 `forbidden`). To
anyone else, and to an admin for a federation it does not administer,
it answers 404:

    POST   /api/federations       {name, description?, group_ids?, admin_ids?}; 201 {id}
    PATCH  /api/federations/{id}  the same fields, each optional; 200 {id}
    DELETE /api/federations/{id}  204

- `group_ids` are logger groups (400 `bad_group` otherwise) and
  replace the federation's groups; `admin_ids` are accounts with the
  admin role (400 `bad_admin`) and replace its admins. A name is 1 to
  64 characters and unique without regard to case (409 `taken`).
- `description` (web migration 000011) is free text about the
  federation, trimmed, at most 2000 characters, with no control
  characters but newline and tab (CR LF is taken as a newline;
  otherwise 400 `bad_description`); `""` clears it. The federations
  page's rows carry it.
- Default (id 0) takes `description` and `admin_ids` only; groups or
  a new name answer 400 `bad_request` (to its admins as well), and it
  cannot be deleted.
- Deleting a federation puts its groups back in Default unless
  another federation holds them, and files a member it was the only
  federation of in Default.
- New federations get the highest id so far plus one (the ids are not
  AUTO_INCREMENT, see web migration 000006).
- Audited as `federation_create`, `federation_update` (with what
  changed: name, whether the description was set or cleared, groups
  added and removed, admins before and after)
  and `federation_delete`, with the acting account.
- The groups of a federation are listed by the pickers' chat search,
  `GET /api/chats/search?federation=<id>`; for Default those are the
  groups in no other federation.

## The request scope

The Go service builds one scope per request
(`Server.requestScope`, package `webapi/internal/scope`) from the
account as its row reads on that request: id, role, the forced
password change and whether its address is verified. Every query over the logger's schema and every
permission check takes it. Something outside it answers 404, exactly
like something that does not exist; a forbidden action on something
the request may see answers 403. The menu (`Viewer.nav` of the page
data) is the scope's list of page keys.

A role change, a disable or a sign-out-everywhere applies to the next
request: the role is read from the account, not from the cookie, and
disabling an account or changing its role also bumps its
session_epoch, which ends its sessions on both services.

## The accounts API

Served by the Go service and described in webapi/openapi.yaml (tag
accounts); every write carries the session's CSRF token.

    GET   /api/accounts[?role=]                 the accounts the viewer manages
    GET   /api/accounts/{id}                    one of them
    POST  /api/accounts                         create; 201 {id, invited, temporary_password?}
    PATCH /api/accounts/{id}                    name, email, role, admin_of, grants
    POST  /api/accounts/{id}/disable            and /enable
    POST  /api/accounts/{id}/password-reset     200 {link_sent, temporary_password?}
    POST  /api/accounts/{id}/resend-invite      204, the invitation again
    POST  /api/accounts/{id}/sign-out-everywhere

- Who manages whom is the request scope's: a superadmin every
  account and any role, an admin the members of its federations. An
  account the viewer does not manage answers 404, as a missing one
  does; to a member the whole administration is 404. A role the
  viewer may not give, or a federation it does not administer,
  answers 403 `forbidden`; a federation that does not exist, 400
  `bad_federation`.
- A member's `grants` are the federations it is filed in, each
  `{"fed": <id>, "group_ids": []}`; groups cannot be granted inside
  them yet (400 `not_supported`, `web-member-access`). An admin must
  file a new member in a federation it administers: without `grants`,
  400 `grants_required`. A superadmin's member without `grants` is
  filed in Default; `"grants": []` files it in none.
- A PATCH with `grants` replaces the member's federations among those
  the viewer administers (all of them for a superadmin); the others
  stay. A grant of a federation the member is already in passes even
  when the viewer does not administer it, so the page can send the
  member's whole list back.
- `admin_of` sets the federations an admin administers; only a
  superadmin sends it. A role change drops the federations of the old
  role: a member made admin leaves its federations, and an account
  made member is filed in Default unless the same PATCH gives
  `grants`.
- The last active superadmin cannot be disabled or demoted (409
  `last_superadmin`). The check locks the active superadmins' rows,
  so two superadmins disabling each other at once leave one.
- Accounts are disabled, never deleted. Disabling, a role change, a
  password reset and sign-out-everywhere bump the account's
  session_epoch, ending its sessions on both services.
- Every change is written to `web_audit` with the acting account
  (`web_user_id`) and the account changed (`target_user_id`):
  `account_create`, `account_update` (naming the fields, the role
  change as `role old->new`, and federation changes as
  `admin_of 1->1,2` or `federations 0->none`), `account_disable`,
  `account_enable`, `account_password_reset`, `account_invite_resend`,
  `account_signout_everywhere`. The account's row in the API carries
  its newest 20.
- The first password and resets, and the address
  ([../../docs/design/decisions/2026-11-18-email-optional.md](../../docs/design/decisions/2026-11-18-email-optional.md),
  [email.md](email.md)): see First passwords below. The address is
  optional. It is the account's own to change; an admin may correct
  it only while the account is invited, a superadmin may change any
  account's (it is then unverified and, with mail, gets a
  verification link). The old address is told when it was verified
  and mail is set up.

## The accounts and federations pages

The Go service loads the data of the accounts page
(`/settings/accounts`, AccountsProps) and of the federations page
(`/settings/federations`, FederationsProps), for superadmins and
admins; to a member they do not exist. The accounts are those of
`GET /api/accounts`; the federations part, and which parts of an
account are `editable` yet, come from the federation helpers
(`webapi/internal/server/federationprops.go`), the same ones the
federations operations use.

A federation's details page (`/settings/federations/{id}`, route
`federation`, FederationDetailProps; the same data at
`GET /api/federations/{id}`) is opened from its card on the
federations page. A superadmin sees every federation, an admin those
it administers; to anyone else, and for any other federation, it does
not exist (404). It carries:

- the federation: id, name, description, whether it is Default, when
  and by which superadmin it was made (null for Default, and the
  creator null when that account is gone), and `group_count`, how many
  of its groups the viewer sees;
- its admins and members (`{id, username, name}`, by username);
- the first page of its groups, as `GET /api/federations/{id}/groups`
  answers it;
- `can_edit` (name, description, groups, admins, delete): a superadmin
  everything but Default's name, groups and deletion; an admin of the
  federation its name (not Default's) and description only.

`GET /api/federations/{id}/groups?q=&cursor=&limit=` pages the
federation's groups the request scope shows, busiest first (message
count, then id, both descending), 50 by default and 1 to 200:
`{items, next_cursor, total}`, each item a group summary with its
message count. `q` keeps the groups whose title or active username
contains it, without regard to case (a leading `@` is dropped), and
`total` counts every match. A cursor the service did not give answers
400 `bad_request`. Groups a superadmin hid from an admin are never
listed and never counted, here and in `group_count`.

## First passwords and resets

Email is optional, for each account and for the service
([../../docs/design/decisions/2026-11-18-email-optional.md](../../docs/design/decisions/2026-11-18-email-optional.md)).
An account needs a username and a password:

- `POST /api/accounts` takes the first password the creator typed
  (`password`, at least 8 bytes, else 400 `weak_password`), or, with
  none, the server generates a random 20-character temporary one,
  answered once as `temporary_password`. Either way the account is
  flagged `must_change_password` and chooses its own at its first
  sign-in. Only the Argon2id hash is stored; no password is logged or
  audited.
- With `"invite": true` the account gets no password; a set-password
  link goes to its address instead ([email.md](email.md)). That needs
  an address (409 `no_email`) and mail set up (409 `mail_off`), and
  takes no password (400). `resend-invite` sends a new one (409
  `not_invited` once the account has set its own password).
- `password-reset` ends every session of the account. With a verified
  address and mail set up, it removes the password and mails a link
  (`{"link_sent": true}`); otherwise it gives a temporary password,
  answered once (`{"link_sent": false, "temporary_password": ...}`),
  which the account must change at its next sign-in.

## Changing a password, and the forced change

`POST /api/account/password` (webapi/openapi.yaml) changes the
account's own password as the Drogon settings page does: the current
one must match, the new one must be at least 8 bytes, with no upper
limit below the body cap, and differ from the current one; it is
hashed with Argon2id. Every other session of the account ends on both
services, and this one gets a new cookie and CSRF token.

An account flagged `must_change_password` (one whose password was set
for it: typed by its creator, or a temporary one) passes a gate
first: until it has chosen its own password, every API operation
behind a session answers 403
`password_change_required`, except the session, logout and the
password change, and page data answers the `password_change` route.
The gate is one check in the request scope (`Scope.Gate`), applied by
`RequireSession`. There is no gate on a missing or unverified email
address.

## Ownership

Every resource that can be owned has one owner, an account
([decision](../../docs/design/decisions/2026-11-18-ownership-and-bots.md),
[web-accounts.md](../../docs/design/web-accounts.md) section 6). The
table `web_resource_relations` (web
migration 000008) records it as a relation between an account and a
resource, so more relations (sharing) can be added later:

| `resource_type` | `resource_id` |
|---|---|
| `mcp_group` | an exposed group, `telegram_public_groups.group_id` |
| `mcp_token` | an MCP token, `web_mcp_tokens.id` |
| `telegram_bot` | a Telegram bot, `telegram_bots.id` |
| `discord_bot` | a Discord bot (reserved for the Discord bots table) |
| `t2d_webhook` | a Telegram to Discord webhook, `telegram_discord_webhooks.id` |
| `d2t_route` | a Discord to Telegram route, `discord_telegram_routes.id` |

- The Go service records the account that creates a resource as its
  owner, and drops the record when it deletes the resource. The
  resources other than MCP tokens are in the logger's database, so
  there is no foreign key between them; should the record fail to be
  written, the resource is left without an owner and the error is
  logged.
- A resource without an owner counts as owned by the superadmins:
  everything made before ownership existed, and whatever the Drogon
  pages create, which record no owner.
- Every change writes a `web_audit` row with who made it and, as the
  target, the owner it concerns: `owner_set` (a first owner),
  `owner_transfer` (with the previous and the new owner) and
  `resource_drop`; the detail names the type and id.
- An account that owns a resource cannot be deleted from the
  database (accounts are disabled, not deleted); a disabled owner's
  resources keep working.
- Bots: a member with `forward` sees and uses only its own Telegram
  and Discord bots; admins and superadmins see every bot and transfer
  it (platform-forwarding.md, Bots). Who sees and transfers the
  forwarding configurations is `forwarding-per-user`'s; until then the
  pages show the owner and every viewer with the ability sees every
  configuration.

### Assigning the existing resources (deployment)

Everything that exists when ownership is introduced becomes owned by
the first superadmin, the account named `superadmin`, except MCP
tokens: each becomes owned by the account that minted it. Once, at the
deployment that brings web migration 000008:

1. Restart the web service, which applies the web migrations, and the
   webapi service, which builds the new binary.
2. Make sure the account `superadmin` exists and is an active
   superadmin (create it on the accounts page, or with the Drogon
   binary's `--seed-admin`).
3. Run, from the deployment directory:

       docker compose exec webapi webapi/build/webapi assign-owner superadmin

It prints, per type, how many resources it assigned (with their ids)
and how many had an owner already, and `Assigned <n> resources.`; it
exits 0. It refuses, assigning nothing and exiting 1, an account that
does not exist, is disabled or is not a superadmin. Running it again
is harmless: it assigns only resources that still have no owner, such
as ones the Drogon pages created since, so it can be repeated while
both services run. Every assignment is a `web_audit` row
(`owner_set`, detail `<type> <id> assign-owner`).

## The Drogon pages

The Drogon pages know only "admin or not":

- superadmins and admins get today's admin pages;
- members are refused every page (403), and a member's session counts
  as none on the public chat pages, because those pages cannot apply
  grants and would show a member the whole archive;
- `--seed-admin` creates or resets a superadmin.
