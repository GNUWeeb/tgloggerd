// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * chat_background_test -- how a chat background change and an animated
 * chat photo are stored, over hand-built TDLib messages: no TDLib
 * client. It checks the curated content_extra keys of the background's
 * fill and pattern, that the background's document is listed as role
 * "background", and that an animated chat photo's video is listed as
 * role "chat_photo" at position 1.
 */
#include <tgloggerd/MessageExtract.hpp>

#include <td/telegram/td_api.hpp>
#include <nlohmann/json.hpp>

#include <cstdio>
#include <string>
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

static td_api::object_ptr<td_api::file> file(int32_t id)
{
	auto f = td_api::make_object<td_api::file>();
	f->id_ = id;
	f->remote_ = td_api::make_object<td_api::remoteFile>();
	f->remote_->id_ = "remote-" + std::to_string(id);
	f->remote_->unique_id_ = "unique-" + std::to_string(id);
	f->local_ = td_api::make_object<td_api::localFile>();
	return f;
}

static td_api::object_ptr<td_api::document> document(int32_t id,
						     const std::string &name,
						     const std::string &mime)
{
	return td_api::make_object<td_api::document>(name, mime, nullptr,
						     nullptr, file(id));
}

static td_api::object_ptr<td_api::MessageContent>
set_background(td_api::object_ptr<td_api::document> doc,
	       td_api::object_ptr<td_api::BackgroundType> type)
{
	auto bg = td_api::make_object<td_api::background>(
		42, false, true, "bgname", std::move(doc), std::move(type));
	auto cb = td_api::make_object<td_api::chatBackground>(std::move(bg), 30);
	return td_api::make_object<td_api::messageChatSetBackground>(
		0, std::move(cb), false);
}

/* The "background" object of @c's content_extra, or null. */
static nlohmann::json background_of(const td_api::MessageContent &c)
{
	models::MessageContent out;
	extract_message_content(c, out);
	nlohmann::json j = nlohmann::json::parse(
		out.content_extra.value_or("null"), nullptr, false);
	if (!j.is_object() || !j.contains("background"))
		return nullptr;
	return j["background"];
}

/* Whether @c's extra files are exactly the background's file @id. */
static bool one_background(const td_api::MessageContent &c, int32_t id,
			   const char *name)
{
	auto files = message_extra_files(c);
	return files.size() == 1 && files[0].file &&
	       files[0].file->id_ == id &&
	       std::string(files[0].role) == "background" &&
	       files[0].position == 0 &&
	       std::string(files[0].category) == "document" &&
	       files[0].file_name == name;
}

static void test_wallpaper()
{
	printf("test: a wallpaper\n");
	auto c = set_background(
		document(7, "wall.jpg", "image/jpeg"),
		td_api::make_object<td_api::backgroundTypeWallpaper>(true,
								     false));
	nlohmann::json bg = background_of(*c);
	CHECK(bg.is_object() && bg.value("id", "") == "42" &&
		      bg.value("name", "") == "bgname" &&
		      bg.value("dark", false) &&
		      bg.value("type", "") == "wallpaper" &&
		      bg.value("dark_theme_dimming", 0) == 30,
	      "the keys stored before stay");
	CHECK(bg.is_object() && bg.value("is_blurred", false) &&
		      !bg.contains("is_moving") && !bg.contains("fill"),
	      "is_blurred kept, an unset is_moving left out, no fill");
	CHECK(one_background(*c, 7, "wall.jpg"),
	      "the wallpaper as role background, position 0");
}

static void test_pattern()
{
	printf("test: a pattern over a gradient\n");
	auto c = set_background(
		document(8, "pattern.tgv", "application/x-tgwallpattern"),
		td_api::make_object<td_api::backgroundTypePattern>(
			td_api::make_object<td_api::backgroundFillGradient>(
				0x112233, 0x445566, 45),
			60, true, true));
	nlohmann::json bg = background_of(*c);
	CHECK(bg.is_object() && bg.value("type", "") == "pattern",
	      "type pattern");
	CHECK(bg.is_object() && bg.value("intensity", 0) == 60 &&
		      bg.value("is_inverted", false) &&
		      bg.value("is_moving", false),
	      "intensity, is_inverted, is_moving");
	CHECK(bg.is_object() && bg.contains("fill") &&
		      bg["fill"].value("type", "") == "gradient" &&
		      bg["fill"]["colors"] ==
			      nlohmann::json::array({ 0x112233, 0x445566 }) &&
		      bg["fill"].value("rotation_angle", 0) == 45,
	      "fill: gradient, top then bottom colour, rotation");
	CHECK(one_background(*c, 8, "pattern.tgv"),
	      "the pattern as role background, position 0");
}

static void test_fill()
{
	printf("test: a fill\n");
	{
		auto c = set_background(
			nullptr,
			td_api::make_object<td_api::backgroundTypeFill>(
				td_api::make_object<td_api::backgroundFillSolid>(
					0)));
		nlohmann::json bg = background_of(*c);
		CHECK(bg.is_object() && bg.value("type", "") == "fill" &&
			      bg.contains("fill") &&
			      bg["fill"].value("type", "") == "solid" &&
			      bg["fill"]["colors"] ==
				      nlohmann::json::array({ 0 }) &&
			      !bg["fill"].contains("rotation_angle"),
		      "solid: one colour, black kept, no rotation");
		CHECK(message_extra_files(*c).empty(), "no document, no file");
	}
	{
		auto c = set_background(
			nullptr,
			td_api::make_object<td_api::backgroundTypeFill>(
				td_api::make_object<
					td_api::backgroundFillFreeformGradient>(
					std::vector<int32_t>{ 1, 2, 3, 4 })));
		nlohmann::json bg = background_of(*c);
		CHECK(bg.is_object() && bg.contains("fill") &&
			      bg["fill"].value("type", "") ==
				      "freeform_gradient" &&
			      bg["fill"]["colors"] ==
				      nlohmann::json::array({ 1, 2, 3, 4 }),
		      "freeform gradient: its colours in order");
	}
	{
		auto c = set_background(
			nullptr,
			td_api::make_object<td_api::backgroundTypeFill>(
				td_api::make_object<
					td_api::backgroundFillGradient>(
					5, 6, 0)));
		nlohmann::json bg = background_of(*c);
		CHECK(bg.is_object() && bg.contains("fill") &&
			      !bg["fill"].contains("rotation_angle"),
		      "gradient: a rotation of 0 left out");
	}
}

static void test_chat_theme()
{
	printf("test: a chat theme background\n");
	auto c = set_background(
		nullptr,
		td_api::make_object<td_api::backgroundTypeChatTheme>("Duck"));
	nlohmann::json bg = background_of(*c);
	CHECK(bg.is_object() && bg.value("type", "") == "chat_theme" &&
		      bg.value("theme_name", "") == "Duck" &&
		      !bg.contains("fill"),
	      "type chat_theme with its theme_name");
	CHECK(message_extra_files(*c).empty(), "no file");
}

static td_api::object_ptr<td_api::chatPhoto> chat_photo(bool big, bool small)
{
	auto p = td_api::make_object<td_api::chatPhoto>();
	p->id_ = 99;
	p->sizes_.push_back(td_api::make_object<td_api::photoSize>(
		"a", file(20), 160, 160, std::vector<int32_t>{}));
	p->sizes_.push_back(td_api::make_object<td_api::photoSize>(
		"c", file(21), 640, 640, std::vector<int32_t>{}));
	if (big)
		p->animation_ = td_api::make_object<td_api::animatedChatPhoto>(
			800, file(22), 0.5);
	if (small)
		p->small_animation_ =
			td_api::make_object<td_api::animatedChatPhoto>(
				160, file(23), 0.5);
	return p;
}

/* Whether @files are the still photo (21) and, if @video, file @video
 * as the animation at position 1. */
static bool photo_files(const std::vector<ExtraFile> &files, int32_t video)
{
	if (files.empty() || !files[0].file || files[0].file->id_ != 21 ||
	    std::string(files[0].role) != "chat_photo" ||
	    files[0].position != 0 ||
	    std::string(files[0].category) != "photo")
		return false;
	if (!video)
		return files.size() == 1;
	return files.size() == 2 && files[1].file &&
	       files[1].file->id_ == video &&
	       std::string(files[1].role) == "chat_photo" &&
	       files[1].position == 1 &&
	       std::string(files[1].category) == "animation";
}

static void test_animated_photo()
{
	printf("test: an animated chat photo\n");
	{
		auto c = td_api::make_object<td_api::messageChatChangePhoto>(
			chat_photo(true, true));
		CHECK(photo_files(message_extra_files(*c), 22),
		      "change: the still photo, then the big animation");
		models::MessageContent out;
		extract_message_content(*c, out);
		nlohmann::json j = nlohmann::json::parse(
			out.content_extra.value_or("null"), nullptr, false);
		CHECK(j.is_object() && j.contains("photo") &&
			      j["photo"].value("animated", false),
		      "animated: true stays");
	}
	{
		auto c = td_api::make_object<td_api::messageChatChangePhoto>(
			chat_photo(false, true));
		CHECK(photo_files(message_extra_files(*c), 23),
		      "change: only a small animation is taken");
	}
	{
		auto c = td_api::make_object<td_api::messageChatChangePhoto>(
			chat_photo(false, false));
		CHECK(photo_files(message_extra_files(*c), 0),
		      "change: a still photo lists no video");
	}
	{
		auto c = td_api::make_object<td_api::messageSuggestProfilePhoto>(
			chat_photo(true, false));
		CHECK(photo_files(message_extra_files(*c), 22),
		      "suggestion: the still photo, then the animation");
	}
}

int main()
{
	test_wallpaper();
	test_pattern();
	test_fill();
	test_chat_theme();
	test_animated_photo();
	if (g_failures) {
		printf("\n%d check(s) FAILED\n", g_failures);
		return 1;
	}
	printf("\nall checks passed\n");
	return 0;
}
