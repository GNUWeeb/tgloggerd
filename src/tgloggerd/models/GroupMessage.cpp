// SPDX-License-Identifier: GPL-2.0-or-later
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
MsgCountRefetch DB::upsertGroupMessage(const models::GroupMessage &msg)
{
	MsgCountRefetch refetch;
	/*
	 * file_id is intentionally omitted: media files are downloaded
	 * asynchronously and linked later via setGroupMessageFile, so the
	 * content upsert must never touch it (a rebuild on edit would
	 * otherwise reset the link to NULL).
	 */
	static const char *upsert_sql =
		"INSERT INTO telegram_group_messages ("
		" chat_id, message_id, sender_user_id, sender_chat_id,"
		" is_channel_post, author_signature, date,"
		" edit_date, content_type, text, entities, service_type,"
		" tdlib_type, content_extra,"
		" is_forwarded, media_album_id, topic_kind, topic_id, meta, is_pinned,"
		" view_count, forward_count, reply_count, reactions"
		") VALUES ("
		" ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?"
		") AS new ON DUPLICATE KEY UPDATE"
		" sender_user_id = new.sender_user_id,"
		" sender_chat_id = new.sender_chat_id,"
		" is_channel_post = new.is_channel_post,"
		" author_signature = new.author_signature,"
		" date = new.date,"
		" is_forwarded = new.is_forwarded,"
		" media_album_id = new.media_album_id,"
		/* What the copy says now: a fact check can be removed, a
		 * message unpinned. Not content, so no edit_date guard. */
		/* ...but a message deleted on a recheck keeps the flag that
		 * tells it from a deletion Telegram reported, for as long as it
		 * stays deleted (the restore below clears both). */
		" meta = IF(telegram_group_messages.deleted_at IS NOT NULL AND"
		"  JSON_EXTRACT(telegram_group_messages.meta, '$.deleted_on_recheck') ="
		"  CAST('true' AS JSON),"
		"  JSON_SET(IF(JSON_TYPE(new.meta) = 'OBJECT', new.meta, JSON_OBJECT()),"
		"   '$.deleted_on_recheck', CAST('true' AS JSON)),"
		"  new.meta),"
		" is_pinned = new.is_pinned,"
		/* A copy that reports no totals keeps the stored ones. */
		" view_count = COALESCE(new.view_count, telegram_group_messages.view_count),"
		" forward_count = COALESCE(new.forward_count, telegram_group_messages.forward_count),"
		" reply_count = COALESCE(new.reply_count, telegram_group_messages.reply_count),"
		" reactions = COALESCE(new.reactions, telegram_group_messages.reactions),"
		/* A copy without a topic leaves a stored one in place. */
		" topic_kind = COALESCE(new.topic_kind, telegram_group_messages.topic_kind),"
		" topic_id = COALESCE(new.topic_id, telegram_group_messages.topic_id),"
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
		" content_type = IF(new.edit_date >= telegram_group_messages.edit_date, new.content_type, telegram_group_messages.content_type),"
		" text = IF(new.edit_date >= telegram_group_messages.edit_date, new.text, telegram_group_messages.text),"
		" entities = IF(new.edit_date >= telegram_group_messages.edit_date, new.entities, telegram_group_messages.entities),"
		" service_type = IF(new.edit_date >= telegram_group_messages.edit_date, new.service_type, telegram_group_messages.service_type),"
		" tdlib_type = IF(new.edit_date >= telegram_group_messages.edit_date, new.tdlib_type, telegram_group_messages.tdlib_type),"
		" content_extra = IF(new.edit_date >= telegram_group_messages.edit_date, new.content_extra, telegram_group_messages.content_extra),"
		" edit_date = IF(new.edit_date >= telegram_group_messages.edit_date, new.edit_date, telegram_group_messages.edit_date)";
		/* deleted_at is deliberately not upserted here: a re-ingested
		 * message (e.g. fetched to resolve a reply) must not clear an
		 * existing deletion time. It is set only on the deletion path. */

	db_.transaction([&](mysql::Transaction &tx) {
		mysql::Param entities_param = std::monostate{};
		if (msg.content.entities.has_value())
			entities_param = *msg.content.entities;

		mysql::Param tdlib_type_param = std::monostate{};
		if (msg.content.tdlib_type.has_value())
			tdlib_type_param = *msg.content.tdlib_type;

		mysql::Param extra_param = std::monostate{};
		if (msg.content.content_extra.has_value())
			extra_param = *msg.content.content_extra;

		/*
		 * The trailing expression asks MySQL whether the stored entities
		 * and the incoming ones are the same JSON VALUE, which is not a
		 * question that can be answered after the fact in C++ -- see
		 * snapshotMessageEditIfChanged(). `<=>` is used rather than `=`
		 * so that NULL (a message with no entities) compares equal to
		 * NULL instead of yielding NULL.
		 */
		auto old_rows = tx.query(
			"SELECT id, edit_date, content_type, text, file_id,"
			"       deleted_at, entities,"
			"       (entities <=> CAST(? AS JSON)),"
			"       tdlib_type, content_extra, " +
			std::string(kCuratedExtraSame) +
			", JSON_EXTRACT(meta, '$.deleted_on_recheck') ="
			"  CAST('true' AS JSON)"
			" FROM telegram_group_messages"
			" WHERE chat_id = ? AND message_id = ?",
			{ entities_param, extra_param, (int64_t)msg.chat_id,
			  (int64_t)msg.message_id });

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

		mysql::Param service_param = std::monostate{};
		if (msg.content.service_type.has_value())
			service_param = *msg.content.service_type;

		mysql::Param album_param = std::monostate{};
		if (msg.media_album_id.has_value())
			album_param = (int64_t)*msg.media_album_id;

		mysql::Param meta_param = std::monostate{};
		if (msg.meta.has_value())
			meta_param = *msg.meta;

		mysql::Param views_param = std::monostate{}, forwards_param = std::monostate{},
			     replies_param = std::monostate{}, reactions_param = std::monostate{};
		if (msg.interaction.has_value()) {
			const auto &ia = *msg.interaction;
			if (ia.view_count)
				views_param = *ia.view_count;
			if (ia.forward_count)
				forwards_param = *ia.forward_count;
			if (ia.reply_count)
				replies_param = *ia.reply_count;
			if (ia.reactions)
				reactions_param = *ia.reactions;
		}

		mysql::Param topic_kind_param = std::monostate{};
		mysql::Param topic_id_param = std::monostate{};
		if (msg.topic_kind.has_value() && msg.topic_id.has_value()) {
			topic_kind_param = *msg.topic_kind;
			topic_id_param = (int64_t)*msg.topic_id;
		}

		std::string new_ct = models::to_string(msg.content.content_type);

		auto bind_all = [&]() -> std::vector<mysql::Param> {
			return {
				(int64_t)msg.chat_id,
				(int64_t)msg.message_id,
				sender_user_param,
				sender_chat_param,
				b(msg.is_channel_post),
				author_param,
				(int64_t)msg.date,
				(int64_t)msg.edit_date,
				new_ct,
				text_param,
				entities_param,
				service_param,
				tdlib_type_param,
				extra_param,
				b(msg.forward_info.has_value()),
				album_param,
				topic_kind_param,
				topic_id_param,
				meta_param,
				b(msg.is_pinned),
				views_param,
				forwards_param,
				replies_param,
				reactions_param,
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
				"SELECT id FROM telegram_group_messages"
				" WHERE chat_id = ? AND message_id = ?",
				{ (int64_t)msg.chat_id,
				  (int64_t)msg.message_id });
			if (!new_rows.empty() && new_rows[0][0].has_value() &&
			    msg.forward_info.has_value()) {
				uint64_t gm_id = std::stoull(*new_rows[0][0]);
				insertForwardInfo(tx, "telegram_group_message_fwd_info",
						  "group_message_id", gm_id,
						  *msg.forward_info);
			}

			/*
			 * Genuinely new message (only reached on first insert,
			 * on the single-threaded serial worker): bump the
			 * group's counter and, when a user sent it, that user's.
			 * One increment per recorded row -- matches the COUNT(1)
			 * prefill in scripts/prefill_msg_count.py.
			 */
			if (bumpMsgCount(tx, "`telegram_groups`", msg.chat_id))
				refetch.group = msg.chat_id;
			if (msg.sender_user_id.has_value() &&
			    bumpMsgCount(tx, "telegram_users", *msg.sender_user_id))
				refetch.user = *msg.sender_user_id;
			refetch.change = MsgChange::New;
			return;
		}

		auto &old = old_rows[0];
		uint64_t gm_id = std::stoull(*old[0]);
		int64_t old_edit_date = old[1].has_value() ?
			std::stoll(*old[1]) : 0;
		bool old_deleted = old[5].has_value();

		/*
		 * Deletion: stamp deleted_at with the time we observed it,
		 * keeping edit_date as the last real content edit time. The
		 * deleted_at IS NULL guard preserves the first deletion time.
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
				"UPDATE telegram_group_messages SET deleted_at = NOW()"
				" WHERE id = ? AND deleted_at IS NULL",
				{ (int64_t)gm_id });
			refetch.change = MsgChange::Deleted;
			return;
		}

		/*
		 * The message is here again, so a deletion an earlier recheck
		 * concluded for it was wrong: undo it. Only a deletion with the
		 * recheck flag; one Telegram reported stays. Before the expiry
		 * check below: an expired message still exists.
		 */
		if (old_deleted && old[11].value_or("0") == "1") {
			tx.execute(
				"UPDATE telegram_group_messages SET deleted_at = NULL,"
				" meta = NULLIF(JSON_REMOVE(meta, '$.deleted_on_recheck'),"
				" JSON_OBJECT())"
				" WHERE id = ? AND deleted_at IS NOT NULL",
				{ (int64_t)gm_id });
			refetch.restored = true;
			refetch.change = MsgChange::Edited;
		}

		/*
		 * Self-destructed media arrives as messageExpired* once it has
		 * been opened or its timer ran out, and every later copy of the
		 * message (reply resolution, backfill, relink) says so. That
		 * copy carries no content, only the fact that Telegram removed
		 * it, so it must neither replace the stored content nor count as
		 * an edit. Like a deletion, expiry keeps what was shown.
		 */
		if (msg.content.is_expired)
			return;

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
		if (old[8].has_value())
			old_content.tdlib_type = *old[8];
		if (old[9].has_value())
			old_content.content_extra = *old[9];

		/*
		 * A copy with a later edit_date: the guarded upsert below
		 * takes its content, and the message reads as edited.
		 */
		if (msg.edit_date > old_edit_date)
			refetch.change = MsgChange::Edited;

		snapshotMessageEditIfChanged(tx, "telegram_group_message_edits",
					     "group_message_id", gm_id,
					     old_content, old_edit_date,
					     msg.content, msg.edit_date,
					     old[7].value_or("0") == "1",
					     old[10].value_or("0") == "1");

		tx.execute(upsert_sql, bind_all());

		if (msg.forward_info.has_value())
			insertForwardInfo(tx, "telegram_group_message_fwd_info",
					  "group_message_id", gm_id,
					  *msg.forward_info);
	});

	return refetch;
}

void DB::setGroupMessageFile(int64_t chat_id, int64_t message_id,
			     uint64_t file_id)
{
	db_.execute("UPDATE telegram_group_messages SET file_id = ?"
		    " WHERE chat_id = ? AND message_id = ?",
		    { (int64_t)file_id, (int64_t)chat_id, (int64_t)message_id });
}

void DB::setGroupMessageReply(int64_t chat_id, int64_t message_id,
			      int64_t reply_to_chat_id,
			      int64_t reply_to_message_id)
{
	/*
	 * Record the replied message universally, and resolve its surrogate
	 * id when it is a group message too (LEFT JOIN, so reply_to_id is
	 * NULL for a cross-table reply or an as-yet-unsaved target).
	 */
	db_.execute(
		"UPDATE telegram_group_messages AS m"
		" LEFT JOIN telegram_group_messages AS r"
		"   ON r.chat_id = ? AND r.message_id = ?"
		" SET m.reply_to_chat_id = ?, m.reply_to_msg_id = ?,"
		"     m.reply_to_id = r.id"
		" WHERE m.chat_id = ? AND m.message_id = ?",
		{ (int64_t)reply_to_chat_id, (int64_t)reply_to_message_id,
		  (int64_t)reply_to_chat_id, (int64_t)reply_to_message_id,
		  (int64_t)chat_id, (int64_t)message_id });
}

} /* namespace tgloggerd */
