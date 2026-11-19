// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * link_preview_test -- how a link preview's type is stored, over
 * hand-built TDLib objects: no TDLib client. It checks the curated
 * telegram_link_previews.extra of each preview type
 * (link_preview_extra_json), the target of a Telegram message link, and
 * the preview's own medium among the message's further files (role
 * "preview_media").
 */
#include <tgloggerd/MessageExtract.hpp>

#include <td/telegram/td_api.hpp>

#include <nlohmann/json.hpp>

#include <cstdio>
#include <cstring>
#include <string>

using namespace tgloggerd;
namespace td_api = td::td_api;
using nlohmann::json;

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

static td_api::object_ptr<td_api::linkPreview>
preview(td_api::object_ptr<td_api::LinkPreviewType> t,
	const std::string &url = "https://example.org/")
{
	auto lp = td_api::make_object<td_api::linkPreview>();
	lp->url_ = url;
	lp->type_ = std::move(t);
	return lp;
}

static td_api::object_ptr<td_api::video> video(int32_t id)
{
	auto v = td_api::make_object<td_api::video>();
	v->width_ = 1280;
	v->height_ = 720;
	v->duration_ = 42;
	v->mime_type_ = "video/mp4";
	v->file_name_ = "clip.mp4";
	v->supports_streaming_ = true;
	v->video_ = file(id);
	return v;
}

static td_api::object_ptr<td_api::animation> animation(int32_t id)
{
	auto a = td_api::make_object<td_api::animation>();
	a->width_ = 320;
	a->height_ = 240;
	a->duration_ = 3;
	a->mime_type_ = "video/mp4";
	a->file_name_ = "loop.mp4";
	a->animation_ = file(id);
	return a;
}

static td_api::object_ptr<td_api::audio> audio(int32_t id)
{
	auto a = td_api::make_object<td_api::audio>();
	a->duration_ = 200;
	a->title_ = "Song";
	a->performer_ = "Band";
	a->file_name_ = "song.mp3";
	a->mime_type_ = "audio/mpeg";
	a->audio_ = file(id);
	return a;
}

/* The extra of @lp parsed, or null when there is none. */
static json extra(const td_api::linkPreview &lp)
{
	auto s = link_preview_extra_json(lp);
	return s ? json::parse(*s) : json();
}

/* Whether @lp's media is exactly file @id as @category. */
static bool one_medium(const td_api::linkPreview &lp, int32_t id,
		       const char *category, const char *name)
{
	auto x = link_preview_media_files(lp);
	return x.size() == 1 && x[0].file && x[0].file->id_ == id &&
	       !strcmp(x[0].category, category) && x[0].file_name == name &&
	       !strcmp(x[0].role, "preview_media") && x[0].position == 0;
}

static void test_extra()
{
	printf("extra keys per preview type\n");
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeEmbeddedVideoPlayer>();
		t->url_ = "https://www.youtube.com/embed/abc";
		t->width_ = 1280;
		t->height_ = 720;
		t->duration_ = 213;
		t->video_ = video(5);
		auto j = extra(*preview(std::move(t)));
		CHECK(j.value("embed_url", "") ==
			      "https://www.youtube.com/embed/abc" &&
		      j.value("embed_width", 0) == 1280 &&
		      j.value("embed_height", 0) == 720 &&
		      j.value("duration", 0) == 213,
		      "an embedded video player: URL, size and duration");
		CHECK(j.contains("media") &&
			      j["media"].value("duration", 0) == 42 &&
			      j["media"].value("mime", "") == "video/mp4",
		      "an embedded video player: its cached video under media");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeEmbeddedAudioPlayer>();
		t->url_ = "https://w.soundcloud.com/player/x";
		t->height_ = 166;
		auto j = extra(*preview(std::move(t)));
		CHECK(j.value("embed_url", "") ==
			      "https://w.soundcloud.com/player/x" &&
			      j.value("embed_height", 0) == 166 &&
			      !j.contains("embed_width") && !j.contains("media"),
		      "an embedded audio player without a cached audio");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeVideo>();
		t->video_ = video(6);
		t->start_timestamp_ = 30;
		auto j = extra(*preview(std::move(t)));
		CHECK(j.value("duration", 0) == 42 &&
			      j.value("width", 0) == 1280 &&
			      j.value("height", 0) == 720 &&
			      j.value("mime", "") == "video/mp4" &&
			      j.value("file_name", "") == "clip.mp4" &&
			      j.value("start_timestamp", 0) == 30 &&
			      !j.contains("cover"),
		      "a video: duration, size, MIME type, start");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeAnimation>();
		t->animation_ = animation(7);
		auto j = extra(*preview(std::move(t)));
		CHECK(j.value("duration", 0) == 3 && j.value("width", 0) == 320,
		      "an animation: duration and size");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeAudio>();
		t->audio_ = audio(8);
		auto j = extra(*preview(std::move(t)));
		CHECK(j.value("duration", 0) == 200 &&
			      j.value("audio_title", "") == "Song" &&
			      j.value("performer", "") == "Band",
		      "an audio: duration, title, performer");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeStory>();
		t->story_poster_chat_id_ = -1001234567890LL;
		t->story_id_ = 17;
		auto j = extra(*preview(std::move(t)));
		CHECK(j.value("poster_chat_id", (int64_t)0) == -1001234567890LL &&
			      j.value("story_id", 0) == 17,
		      "a story: its poster chat and id");
	}
	{
		auto j = extra(*preview(
			td_api::make_object<td_api::linkPreviewTypeMessage>(),
			"https://t.me/gnuweeb/12345"));
		CHECK(j.value("username", "") == "gnuweeb" &&
			      j.value("message_id", (int64_t)0) == 12345 &&
			      !j.contains("chat_id"),
		      "a message preview of a public link: username, message");
	}
	{
		auto j = extra(*preview(
			td_api::make_object<td_api::linkPreviewTypeMessage>(),
			"https://t.me/c/1234567890/77"));
		CHECK(j.value("chat_id", (int64_t)0) == -1001234567890LL &&
			      j.value("message_id", (int64_t)0) == 77 &&
			      !j.contains("username"),
		      "a message preview of a private link: chat id, message");
	}
	CHECK(!link_preview_extra_json(*preview(
		      td_api::make_object<td_api::linkPreviewTypeMessage>(),
		      "https://example.org/a/1")),
	      "a message preview whose URL names no message has no extra");
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeExternalVideo>();
		t->url_ = "https://cdn.example.org/v.mp4";
		t->mime_type_ = "video/mp4";
		t->width_ = 640;
		t->height_ = 360;
		t->duration_ = 9;
		auto j = extra(*preview(std::move(t)));
		CHECK(j.value("media_url", "") == "https://cdn.example.org/v.mp4" &&
			      j.value("width", 0) == 640 &&
			      j.value("duration", 0) == 9,
		      "an external video: URL, MIME type, size, duration");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeChat>();
		t->type_ = td_api::make_object<td_api::inviteLinkChatTypeChannel>();
		t->creates_join_request_ = true;
		auto j = extra(*preview(std::move(t)));
		CHECK(j.value("chat_type", "") == "channel" &&
			      j.value("creates_join_request", false),
		      "a chat: its type and join request");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeUser>();
		t->is_bot_ = true;
		CHECK(extra(*preview(std::move(t))).value("is_bot", false),
		      "a user: whether it is a bot");
	}
	CHECK(!link_preview_extra_json(*preview(
		      td_api::make_object<td_api::linkPreviewTypeArticle>())),
	      "an article without a photo has no extra");
	CHECK(!link_preview_extra_json(*preview(
		      td_api::make_object<td_api::linkPreviewTypeInvoice>())),
	      "an invoice has no extra");
	CHECK(!link_preview_extra_json(*preview(nullptr)),
	      "a preview without a type has no extra");
}

static void test_message_links()
{
	printf("Telegram message links\n");
	struct Case {
		const char *url;
		bool ok;
		int64_t chat_id;
		const char *username;
		int64_t message_id;
	} cases[] = {
		{ "https://t.me/gnuweeb/5", true, 0, "gnuweeb", 5 },
		{ "http://telegram.me/GnuWeeb/6?single", true, 0, "GnuWeeb", 6 },
		{ "https://www.t.me/gnuweeb/7#x", true, 0, "gnuweeb", 7 },
		{ "https://telegram.dog/gnuweeb/8", true, 0, "gnuweeb", 8 },
		{ "https://t.me/gnuweeb/100/9", true, 0, "gnuweeb", 9 },
		{ "t.me/gnuweeb/10", true, 0, "gnuweeb", 10 },
		{ "https://t.me/c/1234567890/11", true, -1001234567890LL, "", 11 },
		{ "https://t.me/c/1234567890/3/12", true, -1001234567890LL, "", 12 },
		{ "tg://resolve?domain=gnuweeb&post=13", true, 0, "gnuweeb", 13 },
		{ "tg://privatepost?channel=1234567890&post=14", true,
		  -1001234567890LL, "", 14 },
		{ "https://t.me/gnuweeb", false, 0, "", 0 },
		{ "https://t.me/+AbCdEf/1", false, 0, "", 0 },
		{ "https://t.me/gnuweeb/s/3", false, 0, "", 0 },
		{ "https://t.me/c/abc/1", false, 0, "", 0 },
		{ "https://example.org/gnuweeb/1", false, 0, "", 0 },
		{ "https://t.me.example.org/gnuweeb/1", false, 0, "", 0 },
		{ "tg://resolve?domain=gnuweeb", false, 0, "", 0 },
	};
	for (const auto &c : cases) {
		MessageLinkTarget t;
		const bool ok = telegram_message_link_target(c.url, t);
		const bool good = ok == c.ok &&
			(!ok || (t.chat_id == c.chat_id &&
				 t.username == c.username &&
				 t.message_id == c.message_id));
		CHECK(good, c.url);
	}
}

static void test_media()
{
	printf("the preview's own medium\n");
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeVideo>();
		t->video_ = video(21);
		CHECK(one_medium(*preview(std::move(t)), 21, "video", "clip.mp4"),
		      "a video preview's video");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeEmbeddedVideoPlayer>();
		t->video_ = video(22);
		CHECK(one_medium(*preview(std::move(t)), 22, "video", "clip.mp4"),
		      "an embedded video player's cached video");
	}
	CHECK(link_preview_media_files(*preview(
		      td_api::make_object<td_api::linkPreviewTypeEmbeddedVideoPlayer>()))
		      .empty(),
	      "an embedded video player without a cached video has none");
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeAnimation>();
		t->animation_ = animation(23);
		CHECK(one_medium(*preview(std::move(t)), 23, "animation",
				 "loop.mp4"),
		      "an animation preview's animation");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeEmbeddedAnimationPlayer>();
		t->animation_ = animation(24);
		CHECK(one_medium(*preview(std::move(t)), 24, "animation",
				 "loop.mp4"),
		      "an embedded animation player's cached animation");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeAudio>();
		t->audio_ = audio(25);
		CHECK(one_medium(*preview(std::move(t)), 25, "audio", "song.mp3"),
		      "an audio preview's audio");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeEmbeddedAudioPlayer>();
		t->audio_ = audio(26);
		CHECK(one_medium(*preview(std::move(t)), 26, "audio", "song.mp3"),
		      "an embedded audio player's cached audio");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeDocument>();
		t->document_ = td_api::make_object<td_api::document>();
		t->document_->document_ = file(27);
		CHECK(link_preview_media_files(*preview(std::move(t))).empty(),
		      "a document preview's document is not kept");
	}
	{
		/* Through the message: what the ingest and the fetcher's
		 * MessageExtra source both list. */
		auto t = td_api::make_object<td_api::linkPreviewTypeVideo>();
		t->video_ = video(28);
		auto c = td_api::make_object<td_api::messageText>();
		c->text_ = td_api::make_object<td_api::formattedText>();
		c->link_preview_ = preview(std::move(t));
		auto x = message_extra_files(*c);
		CHECK(x.size() == 1 && x[0].file->id_ == 28 &&
			      !strcmp(x[0].role, "preview_media") &&
			      x[0].position == 0,
		      "a text message lists its preview's video");
		c->link_preview_ = nullptr;
		CHECK(message_extra_files(*c).empty(),
		      "a text message without a preview lists nothing");
	}
}

int main()
{
	test_extra();
	test_message_links();
	test_media();
	if (g_failures) {
		printf("\n%d check(s) FAILED\n", g_failures);
		return 1;
	}
	printf("\nall checks passed\n");
	return 0;
}
