# TODO

Open work for tgloggerd. Each item is one row in this file plus one
detail file under [todo/](todo/). The design an item implements is in
the files under [design/](design/), which the items point into
("storage-cache.md section 6"). An item changes the design file it
touches, and each decision it makes is a new file under
[design/decisions/](design/decisions/) and nothing else, rather than
text appended to a shared file, so that items worked on in parallel
never edit the same file for bookkeeping.

## Rules

### Identifiers

- Every item has a slug: lowercase letters, digits and hyphens
  (`[a-z0-9]+(-[a-z0-9]+)*`), unique across the list. Never a number,
  so removing an item never renumbers the others.
- The detail file is `docs/todo/<slug>.md`, named exactly after the
  slug, and its first line is `# <slug>`.
- Refer to items by slug in commit messages and discussion, for
  example "Implements `storage-janitor` from docs/TODO.md". The slug
  is also the name of the branch that works on the item.

### Rows

- One row per item, in exactly this form:

      - [`<slug>`](todo/<slug>.md): <short description>

- The description is one sentence. Everything else (why, scope, design
  notes, dependencies, when it is done) belongs in the detail file.
- Rows sit in the section of their area, in suggested working order,
  dependencies first. Items that go beyond what was asked for are
  marked "(optional)".

### Adding an item

1. Pick a slug that is not in use (`ls docs/todo/`).
2. Create `docs/todo/<slug>.md` from the template below.
3. Add the row to the right section.
4. Commit the row and the file together.

Work in progress often turns up more work: a bug nearby, a missing
test, a follow-up. Add it as an item rather than widening the item
being worked on.

### Removing an item

- When an item is done, delete its row and its detail file in the
  commit that completes the work, or in the last commit of the series.
- When only part of it is done, edit the detail file down to what is
  left and keep the row.
- When an item is dropped without being done, delete both and say why
  in the commit message.
- Before deleting, look for other items that name it under "Depends
  on" (`grep -rn '<slug>' docs/`) and update them.
- Git history keeps finished items; this file lists open work only.

### Checking the list

`scripts/check-todo.sh` checks that rows and detail files match one to
one, that slugs are unique, that each row's slug matches its link,
that each detail file starts with its own slug, that every item a file
lists under "Depends on" has a row above it, and that this file, the
detail files and the design are plain ASCII. It prints nothing when
all is well. Run it from the repository root before every commit that
touches the list.

### Detail file template

    # <slug>

    <One paragraph: what this item delivers.>

    ## Why

    <Why it matters; what is missing or breaks without it.>

    ## Scope

    <What is in, and what is explicitly left to other items.>

    ## Design notes

    <Decisions, constraints, pointers into the design
    ("storage-cache.md section N"), known pitfalls, anything already
    measured or ruled out.>

    ## Depends on

    <Slugs of items that must land first, or "Nothing.">

    ## Done when

    <Checkable conditions: commands that pass, behaviour observed.>

## Storage

The file store as a capped cache in front of Telegram:
[design/storage-cache.md](design/storage-cache.md).

- [`storage-cutover`](todo/storage-cutover.md): Deploy the storage cache, dry-run the first sweep, and enable eviction once the owner agrees.
- [`orphan-blob-audit`](todo/orphan-blob-audit.md): Act on the store audit's list of blobs no row describes, as the owner agrees (optional).

## Logging

## Message types

Storing every kind of Telegram message faithfully:
[design/message-types.md](design/message-types.md).

- [`msg-schema-kinds`](todo/msg-schema-kinds.md): Add the new content kinds, `tdlib_type` and `content_extra` to the message, edit and file tables.
- [`msg-classify`](todo/msg-classify.md): Give every ingested message its exact TDLib class and a content kind, keeping unmapped classes as TDLib JSON.
- [`msg-media-detail`](todo/msg-media-detail.md): Store the detail of media messages: size, duration, MIME type, spoiler, sticker emoji and set, audio tags.
- [`msg-structured-content`](todo/msg-structured-content.md): Store locations, venues, contacts, dice, games, shared stories, invoices, checklists, giveaways and calls.
- [`msg-service-detail`](todo/msg-service-detail.md): Recognise every service message class and store its action's fields.
- [`msg-message-files`](todo/msg-message-files.md): Record and download every file of a message, not just the main one.
- [`msg-polls`](todo/msg-polls.md): Store polls and quizzes with their options and keep their latest results current.
- [`msg-topics`](todo/msg-topics.md): Record the forum topic or thread of each group message, and the topics themselves.
- [`msg-meta`](todo/msg-meta.md): Store pinned state, via bot, reply quotes, story replies, forward source, keyboards and the other message-level fields.
- [`msg-link-preview-extras`](todo/msg-link-preview-extras.md): Keep a link preview's description formatting, media position and the message's preview options.
- [`msg-forward-new-kinds`](todo/msg-forward-new-kinds.md): Forward the new content kinds to Discord as readable text plus their main file.
- [`msg-repair-old-rows`](todo/msg-repair-old-rows.md): Re-read rows stored as unknown, and service rows without detail, with the new extraction.
- [`msg-interaction`](todo/msg-interaction.md): Store each message's latest views, forwards, reply count and reaction totals.
- [`web-message-types`](todo/web-message-types.md): Show the new content kinds, topics, polls and message files in the web and search API.

## Web backend in Go

The web backend moves from Drogon to Go:
[design/web-go.md](design/web-go.md).

- [`go-web-openapi`](todo/go-web-openapi.md): Describe the Go API in OpenAPI and generate the React application's TypeScript client.
- [`go-web-pages`](todo/go-web-pages.md): Serve pages from the Go service: page routes, a data loader per route, the render client and a browser-rendered fallback.
- [`ssr-render-service`](todo/ssr-render-service.md): Deploy the Node render service that turns a page's route and data into HTML.
- [`web-live-events`](todo/web-live-events.md): Push live updates of messages, names, photos and statistics to browsers with Server-Sent Events.
- [`go-port-media`](todo/go-port-media.md): Switch /files/<token> over to the Go service after checking it against the Drogon web.
- [`web-roles`](todo/web-roles.md): Give the web several accounts with the roles superadmin, admin and member.
- [`go-port-search`](todo/go-port-search.md): Port the advanced search API to the Go service, enforcing the request scope.
- [`go-port-chat-api`](todo/go-port-chat-api.md): Port the chat API and the user and group page data to the Go service.
- [`go-port-admin`](todo/go-port-admin.md): Port forwarding, Join chat, contact lookup and MCP administration to the Go service.

## Web interface

Accounts, roles and federations:
[design/web-accounts.md](design/web-accounts.md).

- [`web-federations`](todo/web-federations.md): Let superadmins define federations of groups that limit what each admin manages.
- [`web-admin-restrictions`](todo/web-admin-restrictions.md): Let a superadmin hide private messages and specific groups from an admin, with 404 everywhere.
- [`web-member-access`](todo/web-member-access.md): Limit a member to its granted groups and show users only as they appear in them.
- [`web-ownership`](todo/web-ownership.md): Record which account owns which resource, in a table built to take more relations later.
- [`forwarding-per-user`](todo/forwarding-per-user.md): Give each forwarding configuration an owner and a federation, and let members set up their own.
- [`join-chat-federations`](todo/join-chat-federations.md): Gate Join chat by a per-admin permission and let that admin file the joined group into its federations.
- [`web-email`](todo/web-email.md): Give every account an email address for set-password links, password resets and security notices.
- [`web-user-settings`](todo/web-user-settings.md): Keep settings per account, starting with the timezone dates are shown in.
- [`web-fetch-permission`](todo/web-fetch-permission.md): Make fetching an unstored file from Telegram an explicit action for admins and permitted members.
- [`search-engine-backend`](todo/search-engine-backend.md): Add OpenSearch as an optional search backend, with extra search options in the frontend when it is on (optional).
- [`go-port-mcp`](todo/go-port-mcp.md): Move the MCP endpoint to the Go service.
- [`drogon-retire`](todo/drogon-retire.md): Remove the Drogon web once nothing routes to it.

## Operations

