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

- [`web-fetch-unstored-media`](todo/web-fetch-unstored-media.md): Serve unstored files by fetching them through the daemon, with byte ranges, and show them as loadable.
- [`storage-cutover`](todo/storage-cutover.md): Deploy the storage cache, dry-run the first sweep, and enable eviction once the owner agrees.
- [`download-failure-handling`](todo/download-failure-handling.md): Notice failed or cancelled TDLib downloads and drop their pending entries (optional).
- [`orphan-blob-audit`](todo/orphan-blob-audit.md): Act on the store audit's list of blobs no row describes, as the owner agrees (optional).
- [`shared-download-prune-race`](todo/shared-download-prune-race.md): Keep one message's store job from deleting the TDLib file another message's job still has to hash.
- [`file-index-unique-id`](todo/file-index-unique-id.md): Look message media up in the file index by unique id too, and index alias rows as their target (optional).

## Logging

- [`contact-real-name`](todo/contact-real-name.md): Log a user's own name rather than the account's contact name for them, keeping the contact name apart.

## Web interface

- [`web-join-chat`](todo/web-join-chat.md): Let an admin make the logging account join a group or channel by invite link or username from the dashboard.
- [`web-accounts`](todo/web-accounts.md): Let an admin create and manage several web accounts with roles and permissions.
- [`web-access-scopes`](todo/web-access-scopes.md): Limit a member account to some pages, groups and users, enforced in every query.
- [`web-fetch-permission`](todo/web-fetch-permission.md): Make fetching an unstored file from Telegram an explicit action gated by the fetch_files permission.

## Operations

- [`log-freed-before-shutdown`](todo/log-freed-before-shutdown.md): Free the daemon's log handle only after everything that may log has stopped.
