# group-polls-tab

A Polls tab on the group page: the group's polls, newest first, with
their question, options and results, sender, and links to the
message.

## Why

Polls are stored with their options and totals (telegram_polls) and
shown in the chat one by one; there is no way to see a group's polls
together.

## Scope

- A Polls tab on the group page listing the group's polls by the
  Telegram message id that carried them, newest first: question,
  type (regular or quiz), anonymous or not, closed or open (a poll
  past its close date counts as closed), total voters, the options
  with their results (the quiz's correct one marked), the sender
  (name and photo), the date, and links to the message in the chat
  and to the message page.
- Go: a page route for the tab and its list from the group's poll
  messages and the poll tables, within the request scope, paged by
  message id, without a query per poll.
- React: the tab and a compact poll card, reusing the chat's poll
  rendering.

## Depends on

Nothing.

## Done when

- A group's Polls tab lists its polls newest first with results,
  sender and both links, for every role that may see the group.
