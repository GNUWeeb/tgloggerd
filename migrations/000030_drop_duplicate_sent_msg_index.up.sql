-- Drop an index that was a strict duplicate.
--
-- discord_telegram_sent_messages carried both
--   UNIQUE KEY uq_dts_msg_route (discord_message_id, route_id)
--   KEY idx_dts_discord_msg     (discord_message_id)
-- and a leftmost prefix of a composite key serves every query the single-column
-- key could. The duplicate bought nothing and cost a write per bridged message
-- on one of the hottest insert paths in the bridge.
ALTER TABLE discord_telegram_sent_messages
	DROP INDEX idx_dts_discord_msg;
