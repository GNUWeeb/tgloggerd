# discord-sender-own-name

Show a sender's own name, not the account's contact name for them, in
messages forwarded to Discord.

## Why

contact-real-name keeps a contact's own name in `telegram_users`
(`first_name`, `last_name`) and the contact name apart. The Telegram to
Discord forwarder builds the sender's display name from TDLib's user
cache instead (TDLib.cpp, where `ForwardMessage.sender_name` is set),
and TDLib reports a contact under the name the account saved, so a
forwarded message from a contact still carries the contact name.

## Scope

- Take the sender name for forwards from the logged own name (the
  database, or a cache of it) when the sender is a contact, falling
  back to TDLib's name when the own name is not known.
- Not in scope: the reply previews' author names beyond using the same
  source.

## Design notes

- Found while building contact-real-name.
- The forward path runs on the loop thread; keep any lookup off it or
  cached, as the rest of the forwarder does.

## Depends on

Nothing (contact-real-name is merged).

## Done when

- A message from a saved contact forwarded to Discord shows the
  contact's own name.
