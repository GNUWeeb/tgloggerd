ALTER TABLE discord_telegram_sent_messages
	ADD KEY idx_dts_discord_msg (discord_message_id);
