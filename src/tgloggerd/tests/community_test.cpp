// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * community_test -- what the community tables keep of TDLib's
 * community, communityFullInfo and the service lines naming one, over
 * hand-built TDLib objects: no TDLib client, no database.
 */
#include <tgloggerd/CommunityExtract.hpp>

#include <td/telegram/td_api.hpp>

#include <cstdio>
#include <string>
#include <utility>
#include <vector>

using namespace tgloggerd;
namespace td_api = td::td_api;

static int g_failures;

#define CHECK(cond, what)                                                     \
	do {                                                                  \
		if (!(cond)) {                                                \
			printf("  FAIL: %s\n", (what));                       \
			g_failures++;                                         \
		} else {                                                      \
			printf("  ok:   %s\n", (what));                       \
		}                                                             \
	} while (0)

static td_api::object_ptr<td_api::file> file(int32_t id, int64_t size = 0)
{
	auto f = td_api::make_object<td_api::file>();
	f->id_ = id;
	f->size_ = size;
	f->remote_ = td_api::make_object<td_api::remoteFile>();
	f->remote_->id_ = "remote-" + std::to_string(id);
	f->remote_->unique_id_ = "unique-" + std::to_string(id);
	f->local_ = td_api::make_object<td_api::localFile>();
	return f;
}

static td_api::object_ptr<td_api::chatPhotoInfo> photo_info(int32_t small,
							     int32_t big)
{
	return td_api::make_object<td_api::chatPhotoInfo>(
		small ? file(small) : nullptr, big ? file(big, 4096) : nullptr,
		nullptr, false, false);
}

static td_api::object_ptr<td_api::community>
community(int64_t id, td_api::object_ptr<td_api::chatPhotoInfo> photo)
{
	return td_api::make_object<td_api::community>(
		id, true, "Some community", std::move(photo), 1700000000,
		td_api::make_object<td_api::communityMemberStatusMember>(),
		td_api::make_object<td_api::communityPermissions>(false),
		false);
}

static td_api::object_ptr<td_api::photoSize> size(const std::string &type,
						   int32_t id, int32_t w)
{
	return td_api::make_object<td_api::photoSize>(type, file(id), w, w,
						      std::vector<int32_t>());
}

static void test_community(void)
{
	printf("community:\n");
	{
		auto c = community(77, photo_info(1, 2));
		auto m = community_of(*c);
		CHECK(m.id == 77 && m.name == "Some community" &&
		      m.have_access && m.date == 1700000000,
		      "id, name, access and date");
		CHECK(m.photo && m.photo->tg_file_id == "remote-2" &&
		      m.photo->tg_unique_id == "unique-2",
		      "the big photo file is taken");
		CHECK(m.photo && m.photo->file_type == "photo" &&
		      m.photo->file_ext == std::string("jpg") &&
		      !m.photo->on_disk && m.photo->file_size == 4096,
		      "photo recorded metadata-only as a jpg photo");
	}
	{
		auto c = community(78, photo_info(1, 0));
		auto m = community_of(*c);
		CHECK(m.photo && m.photo->tg_file_id == "remote-1",
		      "without a big file, the small one");
	}
	{
		auto c = community(79, nullptr);
		c->have_access_ = false;
		c->date_ = 0;
		auto m = community_of(*c);
		CHECK(!m.photo && !m.have_access && m.date == 0,
		      "no photo, no access, no date");
	}
	{
		auto c = community(80, photo_info(0, 2));
		c->photo_->big_->remote_->id_.clear();
		auto m = community_of(*c);
		CHECK(!m.photo, "a file without a remote id is not recorded");
	}
}

static td_api::object_ptr<td_api::chatPhoto> chat_photo(bool with_c)
{
	std::vector<td_api::object_ptr<td_api::photoSize>> sizes;
	sizes.push_back(size("a", 10, 160));
	if (with_c)
		sizes.push_back(size("c", 11, 640));
	sizes.push_back(size("x", 12, 320));
	auto p = td_api::make_object<td_api::chatPhoto>();
	p->id_ = 5;
	p->sizes_ = std::move(sizes);
	p->animation_ = td_api::make_object<td_api::animatedChatPhoto>(
		640, file(13), 0.0);
	return p;
}

static void test_full_info(void)
{
	printf("full info:\n");
	{
		auto p = chat_photo(true);
		const td_api::file *f = chat_photo_big_file(*p);
		CHECK(f && f->id_ == 11, "the c size is the big file");
		CHECK(chat_photo_files(*p).size() == 4,
		      "every size and the animation are listed");
	}
	{
		auto p = chat_photo(false);
		const td_api::file *f = chat_photo_big_file(*p);
		CHECK(f && f->id_ == 12, "without a c size, the largest");
	}
	{
		std::vector<td_api::object_ptr<td_api::communityChat>> chats;
		chats.push_back(td_api::make_object<td_api::communityChat>(
			-1001, true, false));
		chats.push_back(td_api::make_object<td_api::communityChat>(
			-1002, false, true));
		chats.push_back(nullptr);
		auto fi = td_api::make_object<td_api::communityFullInfo>(
			chat_photo(true), std::move(chats), 3, 0, 0, nullptr);
		auto m = community_full_info_of(77, *fi);
		CHECK(m.community_id == 77, "community id");
		CHECK(m.photo && m.photo->tg_unique_id == "unique-11",
		      "the big photo of the full info");
		CHECK(m.administrator_count && *m.administrator_count == 3,
		      "administrator count");
		CHECK(m.chats.size() == 2, "null chats are skipped");
		CHECK(m.chats.size() == 2 && m.chats[0].chat_id == -1001 &&
		      m.chats[0].can_view_history && !m.chats[0].is_hidden,
		      "first chat: can view history, not hidden");
		CHECK(m.chats.size() == 2 && m.chats[1].chat_id == -1002 &&
		      !m.chats[1].can_view_history && m.chats[1].is_hidden,
		      "second chat: hidden, no history");
	}
	{
		auto fi = td_api::make_object<td_api::communityFullInfo>(
			nullptr,
			std::vector<td_api::object_ptr<td_api::communityChat>>(),
			0, 0, 0, nullptr);
		auto m = community_full_info_of(78, *fi);
		CHECK(!m.photo && !m.administrator_count && m.chats.empty(),
		      "no photo, no administrator count (0), no chats");
	}
}

static void test_messages(void)
{
	printf("messages:\n");
	auto join = td_api::make_object<td_api::messageChatJoinFromCommunity>(41);
	auto added = td_api::make_object<td_api::messageChatAddedToCommunity>(42);
	auto removed =
		td_api::make_object<td_api::messageChatRemovedFromCommunity>();
	auto text = td_api::make_object<td_api::messageText>();
	CHECK(message_community_id(join.get()) == 41, "join from community");
	CHECK(message_community_id(added.get()) == 42, "added to community");
	CHECK(message_community_id(removed.get()) == 0,
	      "removed from community carries no id");
	CHECK(message_community_id(text.get()) == 0, "other content");
	CHECK(message_community_id(nullptr) == 0, "no content");
}

int main()
{
	test_community();
	test_full_info();
	test_messages();
	if (g_failures) {
		printf("\n%d check(s) FAILED\n", g_failures);
		return 1;
	}
	printf("\nall checks passed\n");
	return 0;
}
