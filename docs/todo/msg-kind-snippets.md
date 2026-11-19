# msg-kind-snippets

Summarise a message without text by its kind's detail in reply quotes
and message list snippets: the poll's question, the venue's title, the
contact's name, the call's outcome, instead of "[poll]".

## Why

A reply to a poll, a location or a contact quotes it as "[poll]" or
"[location]" (webapi/internal/server/chats.go, `snippetOf`), and the
message lists show the same for a textless row
(webapi/internal/server/lists.go, `listSnippet`). The detail that would
say more is already stored.

## Scope

- Go (webapi): one summary function for a textless message by kind:
  poll and quiz question, venue title, "Location", contact name, call
  title and outcome, dice emoji and value, game and invoice title,
  "Story by <poster>", giveaway prize, a file's name for media; used
  by `snippetOf` and `listSnippet`, and wherever else "[<kind>]" is
  built. The kind's label from `msg-type-labels` is the fallback.
- React: nothing; the snippets are text.

## Design notes

Names in a summary go through the request scope like any name. A
summary needs the detail loaded for the quoted message, so the reply
query loads it where it is not loaded yet; measure that the chat
window's query stays as fast.

## Depends on

Nothing.

## Done when

- A reply to a poll quotes the poll's question; a venue row in
  /group_messages shows the venue's title.
- A Go test per kind.
