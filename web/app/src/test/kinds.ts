// A message's kind fields as Go sends them for a plain message: no
// detail, no service action, outside any topic, never observed, no
// text entities, no effect. For tests that build a ChatMessage by hand.
export const PLAIN_KINDS = { detail: null, service: null, topic_id: null, pinned: false, views: null, forwards: null, replies: null, reactions: [], entities: [], effect_id: null, effect: null };
