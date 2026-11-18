# msg-forward-new-kinds

Forward the new content kinds to Discord as readable text plus their
main file.

## Why

Without it the new kinds forward as empty messages or not at all.

## Scope

- `forward_kind_str` and the Discord forwarder render location,
  venue, contact, dice, poll, game, story, invoice, checklist, call
  and giveaway as short text, under the route's `fwd_other` flag.

## Design notes

message-types.md section 4.10.

## Depends on

Nothing.

## Done when

- Forwarder tests cover each kind's rendering and the `fwd_other`
  filter.
