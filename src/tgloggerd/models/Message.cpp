// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#include <tgloggerd/DB.hpp>

#include <string>
#include <optional>

namespace tgloggerd {

namespace models {

const char *to_string(MessageContentType t)
{
	switch (t) {
	case MessageContentType::Text:		return "text";
	case MessageContentType::Photo:		return "photo";
	case MessageContentType::Video:		return "video";
	case MessageContentType::Document:	return "document";
	case MessageContentType::Audio:		return "audio";
	case MessageContentType::Voice:		return "voice";
	case MessageContentType::Sticker:	return "sticker";
	case MessageContentType::Animation:	return "animation";
	case MessageContentType::Service:	return "service";
	case MessageContentType::Unknown:	return "unknown";
	}
	return "unknown";
}

const char *to_string(ForwardOriginType t)
{
	switch (t) {
	case ForwardOriginType::User:		return "user";
	case ForwardOriginType::HiddenUser:	return "hidden_user";
	case ForwardOriginType::Chat:		return "chat";
	case ForwardOriginType::Channel:	return "channel";
	}
	return "user";
}

MessageContentType message_content_type_from_string(const std::string &s)
{
	if (s == "text")	return MessageContentType::Text;
	if (s == "photo")	return MessageContentType::Photo;
	if (s == "video")	return MessageContentType::Video;
	if (s == "document")	return MessageContentType::Document;
	if (s == "audio")	return MessageContentType::Audio;
	if (s == "voice")	return MessageContentType::Voice;
	if (s == "sticker")	return MessageContentType::Sticker;
	if (s == "animation")	return MessageContentType::Animation;
	if (s == "service")	return MessageContentType::Service;
	return MessageContentType::Unknown;
}

} /* namespace models */

/*
 * The following DB helpers implement the parts of message upserting that
 * are identical for private and group messages. They take the target
 * table and foreign-key column names (compile-time literals, never user
 * input) so the same logic serves private_message_* and group_message_*.
 */

bool DB::snapshotMessageEditIfChanged(mysql::Transaction &tx,
				      const char *edits_table,
				      const char *fk_column,
				      uint64_t message_row_id,
				      const models::MessageContent &old_content,
				      int64_t old_edit_date,
				      const models::MessageContent &new_content,
				      int64_t new_edit_date,
				      bool entities_same)
{
	/*
	 * Only a genuine edit (edit_date advanced) that actually changed the
	 * content is worth a snapshot. The edits table is append-only; the
	 * live row in the *_messages table always holds the current content.
	 */
	if (new_edit_date <= old_edit_date)
		return false;
	/*
	 * file_id is deliberately excluded from the comparison: media files
	 * are linked asynchronously after the row is written, so new_content
	 * never carries one at build time. Comparing it would flag every
	 * media message as edited. The old file_id is still snapshotted.
	 */
	/*
	 * `entities` cannot be compared here, which is why the caller passes
	 * the answer in. It is a MySQL JSON column, so what comes back out of
	 * a SELECT is the server's normalisation of what went in -- object
	 * keys reordered, a space after every colon and comma -- while
	 * new_content.entities is format_entities_json()'s compact rendering.
	 * The two spellings of one value never match as bytes, so comparing
	 * them as strings reported "changed" for every message that carries an
	 * entity, and every advance of edit_date wrote a snapshot whose content
	 * was identical to the previous one. The caller resolves it the only
	 * way that is actually correct, by asking MySQL to compare the two as
	 * JSON values.
	 *
	 * text, by contrast, is compared right here on purpose. It is a plain
	 * std::string comparison and therefore exact; the same comparison
	 * pushed into SQL would inherit the column's utf8mb4_0900_ai_ci
	 * collation and quietly treat an edit that only changed case or
	 * accents as no edit at all.
	 */
	if (old_content.content_type == new_content.content_type &&
	    old_content.text == new_content.text &&
	    entities_same)
		return false;

	appendMessageVersion(tx, edits_table, fk_column, message_row_id,
			     old_content, old_edit_date);
	return true;
}

/*
 * Append one superseded version of a message to its edits table, unconditionally.
 *
 * The stamp is the edit_date that content had while it was live -- its own
 * start, not the date of the edit that ended it -- so edit_date means the same
 * thing here as on the live row (this version came into being then, 0 for the
 * original). A message's history is then its edit rows followed by the live
 * row, each carrying its own start.
 *
 * Separate from snapshotMessageEditIfChanged because the two callers know
 * different things. That one is driven by a rebuilt message and can see both
 * edit_dates, so it decides whether an edit happened at all; this one is driven
 * by a content update that carries no date, where the fact that the content
 * differs IS the evidence.
 */
void DB::appendMessageVersion(mysql::Transaction &tx, const char *edits_table,
			      const char *fk_column, uint64_t message_row_id,
			      const models::MessageContent &content,
			      int64_t edit_date)
{
	mysql::Param text_param = std::monostate{};
	if (content.text.has_value())
		text_param = *content.text;

	mysql::Param entities_param = std::monostate{};
	if (content.entities.has_value())
		entities_param = *content.entities;

	/*
	 * Refuse to append a version identical to the newest one already
	 * recorded. Two paths can observe the same superseded version -- the
	 * inline content update, and a getMessage reply that was already in
	 * flight when the edit landed. The in-flight reply can even carry the
	 * OLD content with the OLD edit_date, sail past the ordering guard on
	 * equality, transiently revert the live row, and make the next edit
	 * re-snapshot a version that is already in the table. The revert heals
	 * on the next fetch; the duplicate row would be permanent, so it is
	 * stopped here, where every append path funnels through.
	 */
	auto newest = tx.query(
		std::string("SELECT content_type, text, file_id, edit_date,"
			    " (entities <=> CAST(? AS JSON)) FROM ") +
		edits_table + " WHERE " + fk_column + " = ?"
		" ORDER BY id DESC LIMIT 1",
		{ entities_param, (int64_t)message_row_id });
	if (!newest.empty()) {
		auto &n = newest[0];
		std::optional<std::string> n_text;
		if (n[1].has_value())
			n_text = *n[1];
		std::optional<uint64_t> n_file;
		if (n[2].has_value())
			n_file = std::stoull(*n[2]);
		int64_t n_edit = n[3].has_value() ? std::stoll(*n[3]) : 0;

		if (n[0].value_or("unknown") ==
			    models::to_string(content.content_type) &&
		    n_text == content.text &&
		    n_file == content.file_id &&
		    n_edit == edit_date &&
		    n[4].value_or("0") == "1")
			return;
	}

	mysql::Param file_param = std::monostate{};
	if (content.file_id.has_value())
		file_param = (int64_t)*content.file_id;

	std::string sql = std::string("INSERT INTO ") + edits_table + " (" +
		fk_column + ", content_type, text, entities, file_id, edit_date)"
		" VALUES (?, ?, ?, ?, ?, ?)";

	tx.execute(sql, {
		(int64_t)message_row_id,
		std::string(models::to_string(content.content_type)),
		text_param,
		entities_param,
		file_param,
		(int64_t)edit_date,
	});
}

namespace {

/* Content kinds that own a file in telegram_files. */
bool is_media_content_type(models::MessageContentType t)
{
	switch (t) {
	case models::MessageContentType::Photo:
	case models::MessageContentType::Video:
	case models::MessageContentType::Document:
	case models::MessageContentType::Audio:
	case models::MessageContentType::Voice:
	case models::MessageContentType::Sticker:
	case models::MessageContentType::Animation:
		return true;
	default:
		return false;
	}
}

} /* namespace */

/*
 * Record a message's content as observed by updateMessageContent, which hands
 * the daemon the new content directly.
 *
 * That directness is the whole point. The other way to learn an edit is
 * updateMessageEdited, which carries a date and no content and so has to answer
 * with getMessage -- and getMessage replies with the message as it stands when
 * the server gets round to it, not as it stood at the edit that asked. Someone
 * who edits twice in quick succession therefore has the first version answered
 * with the second one, and if they then edit back to the original the archive
 * sees the text it already had, concludes nothing changed, and the middle
 * version is gone with no trace that it existed. Content delivered inline has
 * no such window: it is the version, at the moment it existed.
 *
 * edit_date is deliberately not touched. This update does not carry one, and
 * guessing would defeat the ordering guard on the upsert. The version being
 * superseded is stamped with the date it held while live, which IS known, and
 * the live row's date is left for updateMessageEdited to set.
 *
 * Returns true when a version was recorded.
 */
bool DB::recordMessageContentVersion(int64_t chat_id, int64_t message_id,
				     const models::MessageContent &content,
				     bool is_group)
{
	const char *tbl   = is_group ? "telegram_group_messages"
				     : "telegram_private_messages";
	const char *edits = is_group ? "telegram_group_message_edits"
				     : "telegram_private_message_edits";
	const char *fk    = is_group ? "group_message_id"
				     : "private_message_id";

	bool recorded = false;
	db_.transaction([&](mysql::Transaction &tx) {
		mysql::Param entities_param = std::monostate{};
		if (content.entities.has_value())
			entities_param = *content.entities;

		auto rows = tx.query(
			std::string("SELECT id, edit_date, content_type, text,"
				    " file_id, entities, deleted_at,"
				    " (entities <=> CAST(? AS JSON))"
				    " FROM ") + tbl +
			" WHERE chat_id = ? AND message_id = ?",
			{ entities_param, (int64_t)chat_id,
			  (int64_t)message_id });

		/*
		 * Nothing stored yet. Not an error and not something to insert
		 * from here: this update carries content but none of the
		 * identity a row needs, and the message path will store it
		 * properly when it arrives.
		 */
		if (rows.empty())
			return;

		auto &old = rows[0];

		/* Deleted messages keep the content they were deleted with. */
		if (old[6].has_value())
			return;

		models::MessageContent prev;
		prev.content_type = models::message_content_type_from_string(
			old[2].value_or("unknown"));
		if (old[3].has_value())
			prev.text = *old[3];
		if (old[5].has_value())
			prev.entities = *old[5];
		if (old[4].has_value())
			prev.file_id = std::stoull(*old[4]);

		/* Same comparison the edit path makes; see the note there on
		 * why entities is answered by the server and text is not. */
		if (prev.content_type == content.content_type &&
		    prev.text == content.text &&
		    old[7].value_or("0") == "1")
			return;

		uint64_t row_id = std::stoull(*old[0]);
		int64_t prev_edit_date = old[1].has_value() ?
			std::stoll(*old[1]) : 0;

		appendMessageVersion(tx, edits, fk, row_id, prev,
				     prev_edit_date);

		mysql::Param text_param = std::monostate{};
		if (content.text.has_value())
			text_param = *content.text;

		mysql::Param service_param = std::monostate{};
		if (content.service_type.has_value())
			service_param = *content.service_type;

		/*
		 * Drop the file link when the new content cannot own one --
		 * an edit that turns a photo into text would otherwise leave
		 * the row pointing at the superseded version's file, which the
		 * snapshot above now holds. When the new content IS media the
		 * link is left alone for the async file pipeline to set.
		 */
		std::string sql = std::string("UPDATE ") + tbl +
			" SET content_type = ?, text = ?, entities = ?,"
			" service_type = ?";
		if (!is_media_content_type(content.content_type))
			sql += ", file_id = NULL";
		sql += " WHERE id = ?";

		tx.execute(sql, {
			std::string(models::to_string(content.content_type)),
			text_param,
			entities_param,
			service_param,
			(int64_t)row_id,
		});
		recorded = true;
	});
	return recorded;
}

void DB::insertForwardInfo(mysql::Transaction &tx, const char *table,
			   const char *fk_column, uint64_t message_row_id,
			   const models::ForwardInfo &info)
{
	/*
	 * Idempotent: fk_column is UNIQUE in the *_fwd_info tables, so skip
	 * if forward info for this message has already been recorded.
	 */
	std::string exists_sql = std::string("SELECT 1 FROM ") + table +
		" WHERE " + fk_column + " = ?";
	if (!tx.query(exists_sql, { (int64_t)message_row_id }).empty())
		return;

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

	std::string sql = std::string("INSERT INTO ") + table + " (" +
		fk_column + ", origin_type, origin_sender_user_id,"
		" origin_sender_name, origin_chat_id, origin_message_id,"
		" origin_date) VALUES (?, ?, ?, ?, ?, ?, ?)";

	tx.execute(sql, {
		(int64_t)message_row_id,
		std::string(models::to_string(info.origin_type)),
		sender_id,
		sender_name,
		chat_id_param,
		msg_id_param,
		(int64_t)info.origin_date,
	});
}

} /* namespace tgloggerd */
