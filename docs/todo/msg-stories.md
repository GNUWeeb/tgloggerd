# msg-stories

Archive the stories that messages share or reply to while they are
still live (optional, waiting for a decision).

## Why

A shared story disappears after a day; the reference alone then shows
nothing.

## Scope

- `getStory` for each shared or replied story, a stories table, and
  its media through the storage cache.

## Design notes

message-types.md open question 3.

## Depends on

`msg-structured-content`, `msg-meta`, `msg-message-files`

## Done when

- A shared story's caption and media are kept after it expires.
