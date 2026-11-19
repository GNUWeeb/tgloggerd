# mcp-group-note

Let an admin edit the note of an MCP exposed group on
/mcp-admin/groups.

## Why

The note says why a group is exposed to MCP clients, for whoever
reviews the list later. It can only be written when the group is
exposed; changing it means withdrawing the group and exposing it
again, which also changes who exposed it and when.

## Scope

- React (frontend): an edit action on each row of /mcp-admin/groups
  that opens the note in a small form, saved in place through
  `PATCH /api/mcp/groups/{group_id}` (operation setMcpGroupNote in
  webapi/openapi.yaml; already served by the Go service).

## Depends on

Nothing.

## Done when

- A note edited on /mcp-admin/groups is saved and shown, the group
  keeps its exposer and date, and the audit log records the change.
- React tests cover the form.
