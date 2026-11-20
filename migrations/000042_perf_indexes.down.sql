ALTER TABLE telegram_files DROP INDEX idx_files_on_disk_size;
ALTER TABLE telegram_users DROP INDEX idx_users_created_at;
ALTER TABLE telegram_group_messages
	DROP INDEX idx_group_messages_sender_date,
	DROP INDEX idx_group_messages_sender_chat;
