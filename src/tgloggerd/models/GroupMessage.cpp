// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#include <tgloggerd/DB.hpp>

#include <string>
#include <vector>
#include <optional>

namespace tgloggerd {

namespace {

mysql::Param b(bool v)
{
	return (int64_t)(v ? 1 : 0);
}

} /* namespace */

/*
 * Upsert a group message. Structurally identical to upsertPrivateMessage
 * (see the edit/delete/forward semantics there); the differences are the
 * target tables, the split user/chat sender columns, and the channel-post
 * metadata. The shared edit-snapshot and forward-info logic is reused via
 * the DB message helpers.
 */
void DB::upsertGroupMessage(const models::GroupMessage &msg)
{
	static const char *upsert_sql =
		"INSERT INTO group_messages ("
		" chat_id, message_id, sender_user_id, sender_chat_id,"
		" is_outgoing, is_channel_post, author_signature, date,"
		" edit_date, content_type, text, file_id, is_deleted,"
		" is_forwarded"
		") VALUES ("
		" ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?"
		") AS new ON DUPLICATE KEY UPDATE"
		" sender_user_id = new.sender_user_id,"
		" sender_chat_id = new.sender_chat_id,"
		" is_outgoing = new.is_outgoing,"
		" is_channel_post = new.is_channel_post,"
		" author_signature = new.author_signature,"
		" date = new.date,"
		" edit_date = new.edit_date,"
		" content_type = new.content_type,"
		" text = new.text,"
		" file_id = new.file_id,"
		" is_deleted = new.is_deleted,"
		" is_forwarded = new.is_forwarded";

	db_.transaction([&](mysql::Transaction &tx) {
		auto old_rows = tx.query(
			"SELECT id, edit_date, content_type, text, file_id,"
			"       is_deleted"
			" FROM group_messages"
			" WHERE chat_id = ? AND message_id = ?",
			{ (int64_t)msg.chat_id, (int64_t)msg.message_id });

		mysql::Param sender_user_param = std::monostate{};
		if (msg.sender_user_id.has_value())
			sender_user_param = (int64_t)*msg.sender_user_id;

		mysql::Param sender_chat_param = std::monostate{};
		if (msg.sender_chat_id.has_value())
			sender_chat_param = (int64_t)*msg.sender_chat_id;

		mysql::Param author_param = std::monostate{};
		if (msg.author_signature.has_value())
			author_param = *msg.author_signature;

		mysql::Param text_param = std::monostate{};
		if (msg.content.text.has_value())
			text_param = *msg.content.text;

		mysql::Param file_param = std::monostate{};
		if (msg.content.file_id.has_value())
			file_param = (int64_t)*msg.content.file_id;

		std::string new_ct = models::to_string(msg.content.content_type);

		auto bind_all = [&]() -> std::vector<mysql::Param> {
			return {
				(int64_t)msg.chat_id,
				(int64_t)msg.message_id,
				sender_user_param,
				sender_chat_param,
				b(msg.is_outgoing),
				b(msg.is_channel_post),
				author_param,
				(int64_t)msg.date,
				(int64_t)msg.edit_date,
				new_ct,
				text_param,
				file_param,
				b(msg.is_deleted),
				b(msg.forward_info.has_value()),
			};
		};

		if (old_rows.empty()) {
			/*
			 * A deletion for a message we never stored has no
			 * content to preserve; skip it rather than inserting a
			 * contentless tombstone.
			 */
			if (msg.is_deleted)
				return;

			tx.execute(upsert_sql, bind_all());

			auto new_rows = tx.query(
				"SELECT id FROM group_messages"
				" WHERE chat_id = ? AND message_id = ?",
				{ (int64_t)msg.chat_id,
				  (int64_t)msg.message_id });
			if (!new_rows.empty() && new_rows[0][0].has_value() &&
			    msg.forward_info.has_value()) {
				uint64_t gm_id = std::stoull(*new_rows[0][0]);
				insertForwardInfo(tx, "group_message_fwd_info",
						  "group_message_id", gm_id,
						  *msg.forward_info);
			}
			return;
		}

		auto &old = old_rows[0];
		uint64_t gm_id = std::stoull(*old[0]);
		int32_t old_edit_date = old[1].has_value() ?
			std::stoi(*old[1]) : 0;
		bool old_deleted = old[5].has_value() && *old[5] == "1";

		/*
		 * Deletion: only flag is_deleted, keeping edit_date as the last
		 * real content edit time.
		 */
		if (msg.is_deleted && !old_deleted) {
			tx.execute(
				"UPDATE group_messages SET is_deleted = 1"
				" WHERE id = ?",
				{ (int64_t)gm_id });
			return;
		}

		models::MessageContent old_content;
		old_content.content_type =
			models::message_content_type_from_string(
				old[2].value_or("unknown"));
		if (old[3].has_value())
			old_content.text = *old[3];
		if (old[4].has_value())
			old_content.file_id = std::stoull(*old[4]);

		snapshotMessageEditIfChanged(tx, "group_message_edits",
					     "group_message_id", gm_id,
					     old_content, old_edit_date,
					     msg.content, msg.edit_date);

		tx.execute(upsert_sql, bind_all());

		if (msg.forward_info.has_value())
			insertForwardInfo(tx, "group_message_fwd_info",
					  "group_message_id", gm_id,
					  *msg.forward_info);
	});
}

} /* namespace tgloggerd */
