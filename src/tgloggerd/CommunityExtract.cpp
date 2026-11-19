// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "CommunityExtract.hpp"

namespace tgloggerd {

std::optional<models::File> community_photo_meta(const td_api::file *f)
{
	if (!f || !f->remote_ || f->remote_->id_.empty())
		return std::nullopt;
	models::File mf;
	mf.tg_file_id = f->remote_->id_;
	mf.tg_unique_id = f->remote_->unique_id_;
	mf.file_type = "photo";
	mf.file_size = f->size_ > 0 ? (uint64_t)f->size_ : 0;
	mf.file_ext = "jpg";
	mf.on_disk = false;
	return mf;
}

models::Community community_of(const td_api::community &c)
{
	models::Community m;
	m.id = c.id_;
	m.name = c.name_;
	m.have_access = c.have_access_;
	m.date = c.date_ > 0 ? (int64_t)c.date_ : 0;
	if (c.photo_) {
		m.photo = community_photo_meta(c.photo_->big_.get());
		if (!m.photo)
			m.photo = community_photo_meta(c.photo_->small_.get());
	}
	return m;
}

const td_api::file *chat_photo_big_file(const td_api::chatPhoto &p)
{
	const td_api::file *best = nullptr;
	int64_t best_area = -1;
	for (const auto &s : p.sizes_) {
		if (!s || !s->photo_)
			continue;
		if (s->type_ == "c")
			return s->photo_.get();
		int64_t area = (int64_t)s->width_ * s->height_;
		if (area > best_area) {
			best_area = area;
			best = s->photo_.get();
		}
	}
	return best;
}

std::vector<const td_api::file *> chat_photo_files(const td_api::chatPhoto &p)
{
	std::vector<const td_api::file *> out;
	for (const auto &s : p.sizes_)
		if (s && s->photo_)
			out.push_back(s->photo_.get());
	if (p.animation_ && p.animation_->file_)
		out.push_back(p.animation_->file_.get());
	if (p.small_animation_ && p.small_animation_->file_)
		out.push_back(p.small_animation_->file_.get());
	return out;
}

models::CommunityFullInfo community_full_info_of(
	int64_t community_id, const td_api::communityFullInfo &fi)
{
	models::CommunityFullInfo m;
	m.community_id = community_id;
	if (fi.photo_)
		m.photo = community_photo_meta(chat_photo_big_file(*fi.photo_));
	/* TDLib reports 0 to an account that does not administer it. */
	if (fi.administrator_count_ > 0)
		m.administrator_count = fi.administrator_count_;
	for (const auto &ch : fi.chats_) {
		if (!ch || ch->chat_id_ == 0)
			continue;
		models::CommunityChat c;
		c.chat_id = ch->chat_id_;
		c.is_hidden = ch->is_hidden_;
		c.can_view_history = ch->can_view_history_;
		m.chats.push_back(c);
	}
	return m;
}

int64_t message_community_id(const td_api::MessageContent *content)
{
	if (!content)
		return 0;
	switch (content->get_id()) {
	case td_api::messageChatJoinFromCommunity::ID:
		return static_cast<const td_api::messageChatJoinFromCommunity &>(
			*content).community_id_;
	case td_api::messageChatAddedToCommunity::ID:
		return static_cast<const td_api::messageChatAddedToCommunity &>(
			*content).community_id_;
	default:
		return 0;
	}
}

} /* namespace tgloggerd */
