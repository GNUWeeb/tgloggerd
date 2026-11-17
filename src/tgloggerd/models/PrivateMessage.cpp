// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#include <tgloggerd/DB.hpp>

#include <string>
#include <optional>

namespace tgloggerd {

namespace {

mysql::Param b(bool v)
{
	return (int64_t)(v ? 1 : 0);
}

} /* namespace */

/*
 * Upsert a private message.
 *
 * On first insert: creates the row in telegram_private_messages and, if forward
 * info is present, inserts into telegram_private_message_fwd_info.
 *
 * On update (same chat_id + message_id):
 *   - If edit_date increased and content differs, copy the old content
 *     into telegram_private_message_edits before updating telegram_private_messages.
 *   - If the message became deleted, only stamp deleted_at.
 *   - If forward_info is present and not already recorded, insert it.
 */
MsgCountRefetch DB::upsertPrivateMessage(const models::PrivateMessage &msg)
{
	MsgCountRefetch refetch;
	/*
	 * file_id is intentionally omitted: media files are downloaded
	 * asynchronously and linked later via setPrivateMessageFile, so the
	 * content upsert must never touch it (a rebuild on edit would
	 * otherwise reset the link to NULL).
	 */
	static const char *upsert_sql =
		"INSERT INTO telegram_private_messages ("
		" chat_id, message_id, sender_id, is_outgoing, date,"
		" edit_date, content_type, text, entities, service_type,"
		" is_forwarded, media_album_id"
		") VALUES ("
		" ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?"
		") AS new ON DUPLICATE KEY UPDATE"
		" sender_id = new.sender_id,"
		" is_outgoing = new.is_outgoing,"
		" date = new.date,"
		" is_forwarded = new.is_forwarded,"
		" media_album_id = new.media_album_id,"
		/*
		 * The content columns are only taken from a copy that is at
		 * least as recent as the stored one. Two getMessage replies for
		 * the same message can arrive in either order -- an edit and a
		 * backfill copy race, and a reply that resolves after a later
		 * edit carries the older content -- and without this the loser
		 * of that race silently overwrote the current text with a stale
		 * version. `>=` rather than `>` so an unedited re-ingest, which
		 * carries the same edit_date, still refreshes the row.
		 *
		 * The identity columns above stay unconditional on purpose: a
		 * backfill copy may legitimately know more about the sender or
		 * the album than the live update did.
		 *
		 * edit_date is assigned last. MySQL evaluates these left to
		 * right and a later assignment reads the already-updated value,
		 * so assigning it first would leave every guard below comparing
		 * against the incoming date rather than the stored one.
		 */
		" content_type = IF(new.edit_date >= telegram_private_messages.edit_date, new.content_type, telegram_private_messages.content_type),"
		" text = IF(new.edit_date >= telegram_private_messages.edit_date, new.text, telegram_private_messages.text),"
		" entities = IF(new.edit_date >= telegram_private_messages.edit_date, new.entities, telegram_private_messages.entities),"
		" service_type = IF(new.edit_date >= telegram_private_messages.edit_date, new.service_type, telegram_private_messages.service_type),"
		" edit_date = IF(new.edit_date >= telegram_private_messages.edit_date, new.edit_date, telegram_private_messages.edit_date)";
		/* deleted_at is intentionally not upserted; a re-ingested
		 * message must not clear an existing deletion time. It is set
		 * only on the deletion path below. */

	db_.transaction([&](mysql::Transaction &tx) {
		mysql::Param entities_param = std::monostate{};
		if (msg.content.entities.has_value())
			entities_param = *msg.content.entities;

		/*
		 * Fetch the current row (if any) to detect edits. The trailing
		 * expression compares stored and incoming entities as JSON
		 * VALUES; see snapshotMessageEditIfChanged() for why that cannot
		 * be done in C++.
		 */
		auto old_rows = tx.query(
			"SELECT id, edit_date, content_type, text, file_id,"
			"       deleted_at, entities,"
			"       (entities <=> CAST(? AS JSON))"
			" FROM telegram_private_messages"
			" WHERE chat_id = ? AND message_id = ?",
			{ entities_param, (int64_t)msg.chat_id,
			  (int64_t)msg.message_id });

		mysql::Param sender_param = std::monostate{};
		if (msg.sender_id.has_value())
			sender_param = (int64_t)*msg.sender_id;

		mysql::Param text_param = std::monostate{};
		if (msg.content.text.has_value())
			text_param = *msg.content.text;

		mysql::Param service_param = std::monostate{};
		if (msg.content.service_type.has_value())
			service_param = *msg.content.service_type;

		mysql::Param album_param = std::monostate{};
		if (msg.media_album_id.has_value())
			album_param = (int64_t)*msg.media_album_id;

		std::string new_ct = models::to_string(msg.content.content_type);

		if (old_rows.empty()) {
			/*
			 * A deletion for a message we never stored has no
			 * content to preserve; skip it rather than inserting a
			 * contentless tombstone.
			 */
			if (msg.is_deleted)
				return;

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
				new_ct,
				text_param,
				entities_param,
				service_param,
				b(msg.forward_info.has_value()),
				album_param,
			});

			/*
			 * Retrieve the auto-generated id for the forward info.
			 */
			auto new_rows = tx.query(
				"SELECT id FROM telegram_private_messages"
				" WHERE chat_id = ? AND message_id = ?",
				{ (int64_t)msg.chat_id,
				  (int64_t)msg.message_id });
			if (!new_rows.empty() && new_rows[0][0].has_value() &&
			    msg.forward_info.has_value()) {
				uint64_t pm_id = std::stoull(*new_rows[0][0]);
				insertForwardInfo(tx, "telegram_private_message_fwd_info",
						  "private_message_id", pm_id,
						  *msg.forward_info);
			}

			/*
			 * Genuinely new message: bump the sender user's counter.
			 * sender_id is NULL for our own outgoing messages, which
			 * therefore count for no user -- matching the COUNT(1)
			 * prefill (telegram_private_messages WHERE sender_id = user).
			 */
			if (msg.sender_id.has_value() &&
			    bumpMsgCount(tx, "telegram_users", *msg.sender_id))
				refetch.user = *msg.sender_id;
			refetch.change = MsgChange::New;
			return;
		}

		/*
		 * Existing row: handle edits and deletions.
		 */
		auto &old = old_rows[0];
		uint64_t pm_id = std::stoull(*old[0]);
		int64_t old_edit_date = old[1].has_value() ?
			std::stoll(*old[1]) : 0;
		bool old_deleted = old[5].has_value();

		/*
		 * If the message is now deleted and wasn't before, stamp
		 * deleted_at with the observation time. edit_date is left
		 * untouched so it keeps reflecting the last real content edit;
		 * the IS NULL guard preserves the first deletion time.
		 */
		if (msg.is_deleted) {
			/*
			 * Return whether or not the stamp is applied. A
			 * deletion model carries no content -- the handler
			 * builds it with only chat_id, message_id and the flag
			 * set -- so letting a repeat deletion fall through to
			 * the upsert below overwrites the preserved text with
			 * NULL, the content_type with 'unknown' and both dates
			 * with 0, destroying the very message the tombstone
			 * exists to keep.
			 */
			if (old_deleted)
				return;
			tx.execute(
				"UPDATE telegram_private_messages SET deleted_at = NOW()"
				" WHERE id = ? AND deleted_at IS NULL",
				{ (int64_t)pm_id });
			refetch.change = MsgChange::Deleted;
			return;
		}

		/*
		 * If the edit_date increased and content changed, copy the old
		 * content into telegram_private_message_edits first.
		 */
		models::MessageContent old_content;
		old_content.content_type =
			models::message_content_type_from_string(
				old[2].value_or("unknown"));
		if (old[3].has_value())
			old_content.text = *old[3];
		if (old[6].has_value())
			old_content.entities = *old[6];
		if (old[4].has_value())
			old_content.file_id = std::stoull(*old[4]);

		/*
		 * A copy with a later edit_date: the guarded upsert below
		 * takes its content, and the message reads as edited.
		 */
		if (msg.edit_date > old_edit_date)
			refetch.change = MsgChange::Edited;

		snapshotMessageEditIfChanged(tx, "telegram_private_message_edits",
					     "private_message_id", pm_id,
					     old_content, old_edit_date,
					     msg.content, msg.edit_date,
					     old[7].value_or("0") == "1");

		/*
		 * Update the telegram_private_messages row.
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
			entities_param,
			service_param,
			b(msg.forward_info.has_value()),
			album_param,
		});

		/*
		 * Insert forward info if present and not already recorded.
		 */
		if (msg.forward_info.has_value())
			insertForwardInfo(tx, "telegram_private_message_fwd_info",
					  "private_message_id", pm_id,
					  *msg.forward_info);
	});

	return refetch;
}

void DB::setPrivateMessageFile(int64_t chat_id, int64_t message_id,
			       uint64_t file_id)
{
	db_.execute("UPDATE telegram_private_messages SET file_id = ?"
		    " WHERE chat_id = ? AND message_id = ?",
		    { (int64_t)file_id, (int64_t)chat_id, (int64_t)message_id });
}

void DB::setPrivateMessageReply(int64_t chat_id, int64_t message_id,
				int64_t reply_to_chat_id,
				int64_t reply_to_message_id)
{
	/*
	 * Record the replied message universally, and resolve its surrogate
	 * id when it is a private message too (LEFT JOIN, so reply_to_id is
	 * NULL for a cross-table reply or an as-yet-unsaved target).
	 */
	db_.execute(
		"UPDATE telegram_private_messages AS m"
		" LEFT JOIN telegram_private_messages AS r"
		"   ON r.chat_id = ? AND r.message_id = ?"
		" SET m.reply_to_chat_id = ?, m.reply_to_msg_id = ?,"
		"     m.reply_to_id = r.id"
		" WHERE m.chat_id = ? AND m.message_id = ?",
		{ (int64_t)reply_to_chat_id, (int64_t)reply_to_message_id,
		  (int64_t)reply_to_chat_id, (int64_t)reply_to_message_id,
		  (int64_t)chat_id, (int64_t)message_id });
}

} /* namespace tgloggerd */
