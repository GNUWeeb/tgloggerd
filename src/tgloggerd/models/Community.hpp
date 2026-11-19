// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__MODELS__COMMUNITY_HPP
#define TGLOGGERD__MODELS__COMMUNITY_HPP

#include <tgloggerd/models/File.hpp>

#include <cstdint>
#include <optional>
#include <string>
#include <vector>

namespace tgloggerd {
namespace models {

/*
 * A telegram_communities row as updateCommunity gives it (migration
 * 000049, docs/design/message-types.md section 4.13). The photo is the
 * metadata-only telegram_files row of its big file (else its small
 * one); nullopt means the community has no photo.
 */
struct Community {
	int64_t			id = 0;
	std::string		name;
	bool			have_access = false;
	int64_t			date = 0;	/* Unix seconds; 0 unknown */
	std::optional<File>	photo;
};

/* A telegram_community_chats row. */
struct CommunityChat {
	int64_t		chat_id = 0;
	bool		is_hidden = false;
	bool		can_view_history = false;
};

/*
 * What a community's full info adds to its row: the big photo (nullopt:
 * none), the administrator count (nullopt unless the account administers
 * the community, when TDLib reports 0) and the chats, which replace the
 * stored list.
 */
struct CommunityFullInfo {
	int64_t				community_id = 0;
	std::optional<File>		photo;
	std::optional<int32_t>		administrator_count;
	std::vector<CommunityChat>	chats;
};

} /* namespace models */
} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__MODELS__COMMUNITY_HPP */
