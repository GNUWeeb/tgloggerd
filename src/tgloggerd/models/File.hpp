// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef TGLOGGERD__MODELS__FILE_HPP
#define TGLOGGERD__MODELS__FILE_HPP

#include <string>
#include <cstdint>
#include <optional>

namespace tgloggerd {
namespace models {

/*
 * A downloaded file recorded in the files table. Files are de-duplicated
 * by their SHA-256 digest.
 */
struct File {
	/* TDLib persistent remote file identifier (remoteFile.id_). */
	std::string	tg_file_id;
	/*
	 * TDLib remoteFile.unique_id_: the same for the same file across
	 * sessions and accounts. Empty if unknown (stored as NULL).
	 */
	std::string	tg_unique_id;
	/* telegram_files.file_type enum value, e.g. "photo". */
	std::string	file_type = "unknown";
	uint64_t	file_size = 0;
	/* 64-character lowercase hex of the SHA-256 digest. */
	std::string	sha256_hex;
	/* Lowercase file extension without the leading dot; may be empty. */
	std::optional<std::string>	file_ext;
	/* Original Telegram file name; empty if the file carries none. */
	std::string	orig_file_name;
	/*
	 * Whether the file's bytes are kept in the store. false = metadata-only
	 * (too large per TG_MAX_STORE_FILE_SIZE), re-downloadable by tg_file_id.
	 */
	bool		on_disk = true;
};

/*
 * A telegram_files row as the daemon's in-memory index knows it, keyed by
 * tg_file_id.
 */
struct KnownFile {
	uint64_t	id = 0;
	/*
	 * The bytes were never seen (sha256 IS NULL): the row was recorded
	 * metadata-only (DB::upsertFileMeta), so a chat whose media is
	 * stored should still download it, which fills the row in.
	 */
	bool		meta_only = false;
};

/*
 * Where a file can be found on Telegram again: something that references
 * it and can be fetched afresh, which hands TDLib a valid file reference
 * (storage-cache.md section 8).
 */
struct FileSource {
	enum class Kind {
		Message,	/* the attachment of (chat_id, message_id) */
		LinkPreview,	/* the preview image of (chat_id, message_id) */
		GroupPhoto,	/* chat_id's current photo */
		UserPhoto,	/* user chat_id's current profile photo */
		MessageExtra,	/* a further file of the message: role, position */
		UserPhotoHistory,	/* one of user chat_id's profile photos */
		GroupPhotoHistory,	/* one of chat chat_id's photos */
		RichMedia,	/* a medium in the page blocks of a rich message */
	};
	Kind		kind = Kind::Message;
	int64_t		chat_id = 0;	/* the user id for UserPhoto */
	int64_t		message_id = 0;	/* server message id, as stored */
	std::string	role;		/* MessageExtra: telegram_message_files.role */
	int		position = 0;	/* MessageExtra: order within the role */
	/* *PhotoHistory, RichMedia: the file's Telegram unique id, which
	 * picks the photo or medium out of what the source gives. */
	std::string	unique_id;
};

} /* namespace models */
} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MODELS__FILE_HPP */
