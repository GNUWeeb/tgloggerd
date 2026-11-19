# Decision log: 2026-11-19, Message effects get a table of their own

Decided by the project owner on 2026-11-19, for the
msg-custom-emoji-store item and
[../message-types.md](../message-types.md) section 4.12:

- A message effect (`meta.effect_id`) is stored in a sibling table of
  `telegram_custom_emoji`, `telegram_message_effects` (migration
  000047): one row per effect with its emoji, `is_premium`, its type
  and a `telegram_files` row for each of its files. It is filled on
  demand from `getMessageEffect`, as the custom emoji are from
  `getCustomEmojiStickers`.
- Reason: an effect is not a custom emoji. `getMessageEffect` answers
  a `messageEffect` with up to four files (a static icon, and either
  an emoji reaction's select and effect animations or a premium
  sticker and its premium animation), none of them keyed by a custom
  emoji id, so a row of `telegram_custom_emoji` could hold neither the
  effect's id nor its animations.
- Adjusted from the suggested shape after reading `messageEffect`:
  `effect_type` (`emoji_reaction` or `premium_sticker`) and, for a
  premium sticker, `sticker_file_id` and `sticker_format` are added;
  `effect_animation_file_id` holds the emoji reaction's effect
  animation or the premium sticker's premium animation, which play the
  same part.
