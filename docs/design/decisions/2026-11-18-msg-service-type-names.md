# Decision log: 2026-11-18, Service type names follow the TDLib class

Proposed by the TDLib session on 2026-11-18, for
[../message-types.md](../message-types.md) section 3.5; takes effect
when the plan is approved:

- The `service_type` of every newly recognised service message is its
  TDLib class name without the `message` prefix, in snake case
  (`messageChatOwnerChanged` becomes `chat_owner_changed`).
- The existing names stay as they are, including `chat_create` for
  both group creation classes, so stored rows and the web's filters
  keep working.
- `messagePaymentSuccessfulBot` gets `payment_successful_bot` for new
  rows; old rows keep `payment_successful`, and `tdlib_type` tells
  them apart where it is known.
- Reason: the existing names already follow this rule, so it extends
  without a lookup table and stays predictable as TDLib adds classes.
