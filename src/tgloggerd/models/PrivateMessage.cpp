// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#include <tgloggerd/DB.hpp>

#include <string>
#include <optional>

namespace tgloggerd {

namespace {

const char *content_type_to_string(models::PrivateMessageContentType t)
{
	switch (t) {
	case models::PrivateMessageContentType::Text:	return "text";
	case models::PrivateMessageContentType::Photo:	return "photo";
	case models::PrivateMessageContentType::Video:	return "video";
	case models::PrivateMessageContentType::Document:return "document";
	case models::PrivateMessageContentType::Audio:	return "audio";
	case models::PrivateMessageContentType::Voice:	return "voice";
	case models::PrivateMessageContentType::Sticker:return "sticker";
	case models::PrivateMessageContentType::Animation:return "animation";
	case models::PrivateMessageContentType::Unknown:return "unknown";
	}
	return "unknown";
}

const char *origin_type_to_string(models::ForwardOriginType t)
{
	switch (t) {
	case models::ForwardOriginType::User:		return "user";
	case models::ForwardOriginType::HiddenUser:	return "hidden_user";
	case models::ForwardOriginType::Chat:		return "chat";
	case models::ForwardOriginType::Channel:	return "channel";
	}
	return "user";
}

mysql::Param b(bool v)
{
	return (int64_t)(v ? 1 : 0);
}

} /* namespace */

/*
 * Upsert a private message.
 *
 * On first insert: creates the row in private_messages and, if forward
 * info is present, inserts into private_message_fwd_info.
 *
 * On update (same chat_id + message_id):
 *   - If edit_date increased and content differs, copy the old row
 *     into private_message_edits before updating private_messages.
 *   - If is_deleted changed to true, only set is_deleted = 1.
 *   - If forward_info is present and we haven't recorded it yet,
 *     insert into private_message_fwd_info.
 */
void DB::upsertPrivateMessage(const models::PrivateMessage &msg)
{
	static const char *upsert_sql =
		"INSERT INTO private_messages ("
		" chat_id, message_id, sender_id, is_outgoing, date,"
		" edit_date, content_type, text, file_id, is_deleted"
		") VALUES ("
		" ?, ?, ?, ?, ?, ?, ?, ?, ?, ?"
		") AS new ON DUPLICATE KEY UPDATE"
		" sender_id = new.sender_id,"
		" is_outgoing = new.is_outgoing,"
		" date = new.date,"
		" edit_date = new.edit_date,"
		" content_type = new.content_type,"
		" text = new.text,"
		" file_id = new.file_id,"
		" is_deleted = new.is_deleted";

	static const char *edit_insert_sql =
		"INSERT INTO private_message_edits"
		" (private_message_id, content_type, text, file_id, edit_date)"
		" VALUES (?, ?, ?, ?, ?)";

	static const char *fwd_exists_sql =
		"SELECT 1 FROM private_message_fwd_info"
		" WHERE private_message_id = ?";

	db_.transaction([&](mysql::Transaction &tx) {
		/*
		 * Fetch the current row (if any) to detect edits.
		 */
		auto old_rows = tx.query(
			"SELECT id, edit_date, content_type, text, file_id,"
			"       is_deleted"
			" FROM private_messages"
			" WHERE chat_id = ? AND message_id = ?",
			{ (int64_t)msg.chat_id, (int64_t)msg.message_id });

		mysql::Param sender_param = std::monostate{};
		if (msg.sender_id.has_value())
			sender_param = (int64_t)*msg.sender_id;

		mysql::Param text_param = std::monostate{};
		if (msg.text.has_value())
			text_param = *msg.text;

		mysql::Param file_param = std::monostate{};
		if (msg.file_id.has_value())
			file_param = (int64_t)*msg.file_id;

		if (old_rows.empty()) {
			/*
			 * First time seeing this message: insert new row.
			 */
			tx.execute(upsert_sql, {
				(int64_t)msg.chat_id,
				(int64_t)msg.message_id,
				sender_param,
				b(msg.is_outgoing),
				(int64_t)msg.date,
				(int64_t)msg.edit_date,
				std::string(content_type_to_string(msg.content_type)),
				text_param,
				file_param,
				b(msg.is_deleted),
			});

			/*
			 * Retrieve the auto-generated id for the forward info.
			 */
			auto new_rows = tx.query(
				"SELECT id FROM private_messages"
				" WHERE chat_id = ? AND message_id = ?",
				{ (int64_t)msg.chat_id,
				  (int64_t)msg.message_id });
			if (!new_rows.empty() && new_rows[0][0].has_value() &&
			    msg.forward_info.has_value()) {
				uint64_t pm_id =
					std::stoull(*new_rows[0][0]);
				insertForwardInfo(tx, pm_id, *msg.forward_info);
			}
			return;
		}

		/*
		 * Existing row: handle edits and deletions.
		 */
		auto &old = old_rows[0];
		uint64_t pm_id = std::stoull(*old[0]);
		int32_t old_edit_date = old[1].has_value() ?
			std::stoi(*old[1]) : 0;
		std::string old_ct = old[2].value_or("unknown");
		std::optional<std::string> old_text;
		if (old[3].has_value())
			old_text = *old[3];
		std::optional<uint64_t> old_file_id;
		if (old[4].has_value())
			old_file_id = std::stoull(*old[4]);
		bool old_deleted = old[5].has_value() &&
			*old[5] == "1";

		/*
		 * If the message is now deleted and wasn't before,
		 * only update is_deleted.
		 */
		if (msg.is_deleted && !old_deleted) {
			tx.execute(
				"UPDATE private_messages SET is_deleted = 1,"
				" edit_date = ? WHERE id = ?",
				{ (int64_t)msg.edit_date, (int64_t)pm_id });
			return;
		}

		/*
		 * If the edit_date increased and content changed, copy
		 * the old content into private_message_edits first.
		 */
		std::string new_ct = content_type_to_string(msg.content_type);
		if (msg.edit_date > old_edit_date &&
		    (old_ct != new_ct ||
		     old_text != msg.text ||
		     old_file_id != msg.file_id)) {

			mysql::Param etxt = std::monostate{};
			if (old_text.has_value())
				etxt = *old_text;

			mysql::Param efid = std::monostate{};
			if (old_file_id.has_value())
				efid = (int64_t)*old_file_id;

			tx.execute(edit_insert_sql, {
				(int64_t)pm_id,
				old_ct,
				etxt,
				efid,
				(int64_t)msg.edit_date,
			});
		}

		/*
		 * Update the private_messages row.
		 */
		tx.execute(upsert_sql, {
			(int64_t)msg.chat_id,
			(int64_t)msg.message_id,
			sender_param,
			b(msg.is_outgoing),
			(int64_t)msg.date,
			(int64_t)msg.edit_date,
			new_ct,
			text_param,
			file_param,
			b(msg.is_deleted),
		});

		/*
		 * Insert forward info if present and not already recorded.
		 */
		if (msg.forward_info.has_value()) {
			auto fwd = tx.query(fwd_exists_sql,
				{ (int64_t)pm_id });
			if (fwd.empty())
				insertForwardInfo(tx, pm_id,
						  *msg.forward_info);
		}
	});
}

void DB::insertForwardInfo(mysql::Transaction &tx, uint64_t private_message_id,
			   const models::ForwardInfo &info)
{
	mysql::Param sender_id = std::monostate{};
	if (info.origin_sender_user_id.has_value())
		sender_id = (int64_t)*info.origin_sender_user_id;

	mysql::Param sender_name = std::monostate{};
	if (info.origin_sender_name.has_value())
		sender_name = *info.origin_sender_name;

	mysql::Param chat_id_param = std::monostate{};
	if (info.origin_chat_id.has_value())
		chat_id_param = (int64_t)*info.origin_chat_id;

	mysql::Param msg_id_param = std::monostate{};
	if (info.origin_message_id.has_value())
		msg_id_param = (int64_t)*info.origin_message_id;

	tx.execute(
		"INSERT INTO private_message_fwd_info"
		" (private_message_id, origin_type, origin_sender_user_id,"
		"  origin_sender_name, origin_chat_id, origin_message_id,"
		"  origin_date)"
		" VALUES (?, ?, ?, ?, ?, ?, ?)",
		{
			(int64_t)private_message_id,
			std::string(origin_type_to_string(info.origin_type)),
			sender_id,
			sender_name,
			chat_id_param,
			msg_id_param,
			(int64_t)info.origin_date,
		});
}

} /* namespace tgloggerd */
