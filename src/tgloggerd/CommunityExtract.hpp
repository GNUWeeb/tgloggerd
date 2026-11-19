// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__COMMUNITY_EXTRACT_HPP
#define TGLOGGERD__COMMUNITY_EXTRACT_HPP

#include <tgloggerd/models/Community.hpp>
#include <tgloggerd/models/File.hpp>

#include <td/telegram/td_api.h>

#include <cstdint>
#include <optional>
#include <vector>

/*
 * What the community tables (message-types.md section 4.13) keep of
 * TDLib's community, communityFullInfo and the service lines that name
 * a community. Pure functions over td_api objects, shared by ingest and
 * by the fetcher, which takes a photo out of a fresh full info the same
 * way.
 */

namespace tgloggerd {

namespace td_api = td::td_api;

/*
 * The metadata-only telegram_files row of a community photo file
 * (file_type photo, extension jpg, on_disk false), or nullopt when the
 * file carries no remote id to record.
 */
std::optional<models::File> community_photo_meta(const td_api::file *f);

/*
 * The telegram_communities row of an updateCommunity: the photo is
 * chatPhotoInfo's big file (640x640), else its small one.
 */
models::Community community_of(const td_api::community &c);

/*
 * The big file of a full photo: its "c" (640x640) size, which is the
 * same file as chatPhotoInfo.big, else its largest size; null if it
 * has none.
 */
const td_api::file *chat_photo_big_file(const td_api::chatPhoto &p);

/* Every file of a full photo: its sizes, then its animations. */
std::vector<const td_api::file *> chat_photo_files(const td_api::chatPhoto &p);

/* What a community's full info adds to its row. */
models::CommunityFullInfo community_full_info_of(
	int64_t community_id, const td_api::communityFullInfo &fi);

/*
 * The community a message names: that of messageChatJoinFromCommunity
 * and messageChatAddedToCommunity; 0 for any other content (also
 * messageChatRemovedFromCommunity, which carries no id).
 */
int64_t message_community_id(const td_api::MessageContent *content);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__COMMUNITY_EXTRACT_HPP */
