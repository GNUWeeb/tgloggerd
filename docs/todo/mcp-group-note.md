# mcp-group-note

Let an admin edit the note of an MCP exposed group on
/mcp-admin/groups.

## Why

The note says why a group is exposed to MCP clients, for whoever
reviews the list later. It can only be written when the group is
exposed; changing it means withdrawing the group and exposing it
again, which also changes who exposed it and when.

## Scope

- Go (webapi): `PATCH /api/mcp/groups/{id}` with `{note}`, under the
  same permission as exposing a group, clipped to the note's limit,
  audited as `mcp_note` with the old and new note; it changes neither
  who exposed the group nor when. OpenAPI and web/docs/mcp.md follow.
- React (frontend): an edit action on each row of /mcp-admin/groups
  that opens the note in a small form, saved in place.

## Depends on

Nothing.

## Done when

- A note edited on /mcp-admin/groups is saved and shown, the group
  keeps its exposer and date, and the audit log records the change.
- Go tests cover the edit, the clip, an unknown group and a viewer
  without the permission; React tests cover the form.
