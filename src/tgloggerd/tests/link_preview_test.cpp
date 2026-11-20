// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * link_preview_test -- how a link preview's type is stored, over
 * hand-built TDLib objects: no TDLib client. It checks the type name and
 * image of each preview type (link_preview_image), the curated
 * telegram_link_previews.extra (link_preview_extra_json), the target of
 * a Telegram message link, and the preview's own media among the
 * message's further files (role "preview_media").
 */
#include <tgloggerd/MessageExtract.hpp>

#include <td/telegram/td_api.hpp>

#include <nlohmann/json.hpp>

#include <cstdio>
#include <cstring>
#include <initializer_list>
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

static td_api::object_ptr<td_api::sticker> sticker(int32_t id)
{
	auto s = td_api::make_object<td_api::sticker>();
	s->id_ = 1000 + id;
	s->set_id_ = 77;
	s->width_ = 512;
	s->height_ = 512;
	s->emoji_ = "\xF0\x9F\x98\x80";
	s->format_ = td_api::make_object<td_api::stickerFormatWebp>();
	s->sticker_ = file(id);
	return s;
}

/* A photo of two sizes; the larger is file @id. */
static td_api::object_ptr<td_api::photo> photo(int32_t id)
{
	auto p = td_api::make_object<td_api::photo>();
	auto small = td_api::make_object<td_api::photoSize>();
	small->type_ = "s";
	small->width_ = 90;
	small->height_ = 60;
	small->photo_ = file(id + 500);
	auto big = td_api::make_object<td_api::photoSize>();
	big->type_ = "y";
	big->width_ = 1200;
	big->height_ = 800;
	big->photo_ = file(id);
	p->sizes_.push_back(std::move(small));
	p->sizes_.push_back(std::move(big));
	return p;
}

/* The extra of @lp parsed, or an empty object when there is none. */
static json extra(const td_api::linkPreview &lp)
{
	auto s = link_preview_extra_json(lp);
	return s ? json::parse(*s) : json::object();
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
		t->document_->file_name_ = "paper.pdf";
		t->document_->document_ = file(27);
		CHECK(one_medium(*preview(std::move(t)), 27, "document",
				 "paper.pdf"),
		      "a document preview's document");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeSticker>();
		t->sticker_ = sticker(29);
		CHECK(one_medium(*preview(std::move(t)), 29, "sticker", ""),
		      "a sticker preview's sticker");
	}
	{
		/* The video at 0 and its cover after it. */
		auto t = td_api::make_object<td_api::linkPreviewTypeVideo>();
		t->video_ = video(30);
		t->cover_ = photo(31);
		auto lp = preview(std::move(t));
		auto x = link_preview_media_files(*lp);
		CHECK(x.size() == 2 && x[0].file->id_ == 30 &&
			      x[0].position == 0 && x[1].file->id_ == 31 &&
			      !strcmp(x[1].category, "photo") &&
			      !strcmp(x[1].role, "preview_media") &&
			      x[1].position == 1,
		      "a video preview's cover at position 1");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeVideo>();
		t->cover_ = photo(32);
		auto lp = preview(std::move(t));
		auto x = link_preview_media_files(*lp);
		CHECK(x.size() == 1 && x[0].file->id_ == 32 &&
			      x[0].position == 1,
		      "a cover keeps position 1 without its video");
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

static td_api::object_ptr<td_api::thumbnail> thumbnail(int32_t id)
{
	auto t = td_api::make_object<td_api::thumbnail>();
	t->format_ = td_api::make_object<td_api::thumbnailFormatJpeg>();
	t->width_ = 320;
	t->height_ = 320;
	t->file_ = file(id);
	return t;
}

static td_api::object_ptr<td_api::chatPhoto> chat_photo(int32_t id)
{
	auto p = td_api::make_object<td_api::chatPhoto>();
	auto sz = td_api::make_object<td_api::photoSize>();
	sz->type_ = "c";
	sz->width_ = 640;
	sz->height_ = 640;
	sz->photo_ = file(id);
	p->sizes_.push_back(std::move(sz));
	return p;
}

/* The image and type name of @t. */
static const td_api::file *image(const td_api::LinkPreviewType *t,
				 std::string *name = nullptr)
{
	const char *n = "";
	const td_api::file *f = link_preview_image(t, &n);
	if (name)
		*name = n;
	return f;
}

/* Whether the media of @lp are exactly @want: (file id, category,
 * position), all role preview_media. */
struct Medium {
	int32_t id;
	const char *category;
	int position;
};
static bool media_are(const td_api::linkPreview &lp,
		      std::initializer_list<Medium> want)
{
	auto x = link_preview_media_files(lp);
	if (x.size() != want.size())
		return false;
	size_t i = 0;
	for (const auto &w : want) {
		const auto &e = x[i++];
		if (!e.file || e.file->id_ != w.id ||
		    strcmp(e.category, w.category) ||
		    strcmp(e.role, "preview_media") || e.position != w.position)
			return false;
	}
	return true;
}

static void test_type_names()
{
	printf("type names\n");
	struct Case {
		td_api::object_ptr<td_api::LinkPreviewType> t;
		const char *name;
	};
	Case cases[] = {
		{ td_api::make_object<td_api::linkPreviewTypeAlbum>(), "album" },
		{ td_api::make_object<td_api::linkPreviewTypeBackground>(), "background" },
		{ td_api::make_object<td_api::linkPreviewTypeDirectMessagesChat>(), "direct_messages_chat" },
		{ td_api::make_object<td_api::linkPreviewTypeGiftAuction>(), "gift_auction" },
		{ td_api::make_object<td_api::linkPreviewTypeGiftCollection>(), "gift_collection" },
		{ td_api::make_object<td_api::linkPreviewTypeLiveStory>(), "live_story" },
		{ td_api::make_object<td_api::linkPreviewTypePremiumGiftCode>(), "premium_gift_code" },
		{ td_api::make_object<td_api::linkPreviewTypeRequestManagedBot>(), "request_managed_bot" },
		{ td_api::make_object<td_api::linkPreviewTypeShareableChatFolder>(), "shareable_chat_folder" },
		{ td_api::make_object<td_api::linkPreviewTypeStickerSet>(), "sticker_set" },
		{ td_api::make_object<td_api::linkPreviewTypeStoryAlbum>(), "story_album" },
		{ td_api::make_object<td_api::linkPreviewTypeTextCompositionStyle>(), "text_composition_style" },
		{ td_api::make_object<td_api::linkPreviewTypeTheme>(), "theme" },
		{ td_api::make_object<td_api::linkPreviewTypeUpgradedGift>(), "upgraded_gift" },
		{ td_api::make_object<td_api::linkPreviewTypeVideoNote>(), "video_note" },
		{ td_api::make_object<td_api::linkPreviewTypeVoiceNote>(), "voice_note" },
		/* The names stored before keep their meaning. */
		{ td_api::make_object<td_api::linkPreviewTypeUnsupported>(), "unsupported" },
		{ td_api::make_object<td_api::linkPreviewTypeArticle>(), "article" },
		{ td_api::make_object<td_api::linkPreviewTypeEmbeddedVideoPlayer>(), "embedded_video_player" },
		{ td_api::make_object<td_api::linkPreviewTypeVideo>(), "video" },
		{ td_api::make_object<td_api::linkPreviewTypeSticker>(), "sticker" },
		{ td_api::make_object<td_api::linkPreviewTypeStory>(), "story" },
		{ td_api::make_object<td_api::linkPreviewTypeExternalAudio>(), "external_audio" },
	};
	for (const auto &c : cases) {
		std::string name;
		image(c.t.get(), &name);
		CHECK(name == c.name, c.name);
	}
	std::string name;
	CHECK(!image(nullptr, &name) && name == "unsupported",
	      "no type at all is unsupported");
}

static void test_new_types()
{
	printf("the types stored as unsupported before\n");
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeAlbum>();
		t->caption_ = "Trip";
		t->media_.push_back(
			td_api::make_object<td_api::linkPreviewAlbumMediaPhoto>(photo(41)));
		t->media_.push_back(nullptr);
		t->media_.push_back(
			td_api::make_object<td_api::linkPreviewAlbumMediaVideo>(video(42)));
		auto lp = preview(std::move(t));
		auto j = extra(*lp);
		CHECK(j.value("caption", "") == "Trip" && j.contains("items") &&
			      j["items"].size() == 3 &&
			      j["items"][0].value("kind", "") == "photo" &&
			      j["items"][0].value("width", 0) == 1200 &&
			      j["items"][1].is_null() &&
			      j["items"][2].value("kind", "") == "video" &&
			      j["items"][2].value("duration", 0) == 42 &&
			      j["items"][2].value("file_name", "") == "clip.mp4",
		      "an album: its caption and items, a missing one null");
		CHECK(media_are(*lp, { { 41, "photo", 0 }, { 42, "video", 2 } }),
		      "an album: each item by position");
		CHECK(!image(lp->type_.get()), "an album: no card image");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeBackground>();
		t->document_ = td_api::make_object<td_api::document>();
		t->document_->mime_type_ = "image/jpeg";
		t->document_->file_name_ = "bg.jpg";
		t->document_->thumbnail_ = thumbnail(44);
		t->document_->document_ = file(43);
		t->background_type_ =
			td_api::make_object<td_api::backgroundTypeWallpaper>(true, false);
		auto lp = preview(std::move(t));
		auto j = extra(*lp);
		CHECK(j.value("background_type", "") == "wallpaper" &&
			      j.value("is_blurred", false) &&
			      !j.contains("is_moving") &&
			      j.value("mime", "") == "image/jpeg" &&
			      j.value("file_name", "") == "bg.jpg",
		      "a wallpaper: its type, flags and file");
		CHECK(media_are(*lp, { { 43, "document", 0 } }),
		      "a wallpaper: its document at 0");
		auto *img = image(lp->type_.get());
		CHECK(img && img->id_ == 44, "a wallpaper: its thumbnail as the image");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeBackground>();
		t->background_type_ = td_api::make_object<td_api::backgroundTypeFill>(
			td_api::make_object<td_api::backgroundFillSolid>(0x336699));
		auto lp = preview(std::move(t));
		auto j = extra(*lp);
		CHECK(j.value("background_type", "") == "fill" &&
			      j.contains("fill") &&
			      j["fill"].value("type", "") == "solid" &&
			      j["fill"]["colors"][0] == 0x336699,
		      "a fill: its colour");
		CHECK(link_preview_media_files(*lp).empty(), "a fill: no file");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeDirectMessagesChat>();
		t->photo_ = chat_photo(45);
		auto lp = preview(std::move(t));
		auto *img = image(lp->type_.get());
		CHECK(img && img->id_ == 45 && !link_preview_extra_json(*lp) &&
			      link_preview_media_files(*lp).empty(),
		      "a direct messages chat: its photo only");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeGiftAuction>();
		t->gift_ = td_api::make_object<td_api::gift>();
		t->gift_->id_ = 9;
		t->gift_->star_count_ = 100;
		t->gift_->sticker_ = sticker(46);
		t->gift_->auction_info_ =
			td_api::make_object<td_api::giftAuction>("auc", 50, 1690000000);
		t->auction_end_date_ = 1700000000;
		auto lp = preview(std::move(t));
		auto j = extra(*lp);
		CHECK(j.contains("gift") && j["gift"].value("id", "") == "9" &&
			      j["gift"].value("star_count", 0) == 100 &&
			      j.value("auction_end_date", 0) == 1700000000 &&
			      j.value("gifts_per_round", 0) == 50 &&
			      j.value("auction_start_date", 0) == 1690000000,
		      "a gift auction: the gift and the auction's dates");
		CHECK(media_are(*lp, { { 46, "sticker", 0 } }),
		      "a gift auction: the gift's sticker");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeGiftCollection>();
		for (int i = 0; i < 6; i++)
			t->icons_.push_back(sticker(60 + i));
		auto lp = preview(std::move(t));
		auto j = extra(*lp);
		CHECK(j.contains("icons") && j["icons"].size() == 5 &&
			      j["icons"][4].value("sticker_id", "") == "1064",
		      "a gift collection: its first five icons");
		CHECK(media_are(*lp, { { 60, "sticker", 0 }, { 61, "sticker", 1 },
				       { 62, "sticker", 2 }, { 63, "sticker", 3 },
				       { 64, "sticker", 4 } }),
		      "a gift collection: the icons by position");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeLiveStory>();
		t->story_poster_chat_id_ = -1001234567890LL;
		t->story_id_ = 3;
		auto j = extra(*preview(std::move(t)));
		CHECK(j.value("poster_chat_id", (int64_t)0) == -1001234567890LL &&
			      j.value("story_id", 0) == 3,
		      "a live story: its poster chat and id");
	}
	CHECK(!link_preview_extra_json(*preview(
		      td_api::make_object<td_api::linkPreviewTypePremiumGiftCode>(),
		      "https://t.me/giftcode/SECRETCODE")),
	      "a premium gift code has no extra (its code is not kept)");
	CHECK(!link_preview_extra_json(*preview(
		      td_api::make_object<td_api::linkPreviewTypeRequestManagedBot>())),
	      "a managed bot request has no extra");
	CHECK(!link_preview_extra_json(*preview(
		      td_api::make_object<td_api::linkPreviewTypeShareableChatFolder>())),
	      "a shareable chat folder has no extra");
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeStickerSet>();
		for (int i = 0; i < 3; i++)
			t->stickers_.push_back(sticker(70 + i));
		auto lp = preview(std::move(t));
		auto j = extra(*lp);
		CHECK(j.value("set_id", "") == "77" && j.contains("stickers") &&
			      j["stickers"].size() == 3 &&
			      j["stickers"][2].value("sticker_id", "") == "1072" &&
			      j["stickers"][0].value("format", "") == "webp" &&
			      !j["stickers"][0].contains("set_id"),
		      "a sticker set: its id and stickers");
		CHECK(media_are(*lp, { { 70, "sticker", 0 }, { 71, "sticker", 1 },
				       { 72, "sticker", 2 } }),
		      "a sticker set: the stickers by position");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeStoryAlbum>();
		t->photo_icon_ = photo(80);
		t->video_icon_ = video(81);
		auto lp = preview(std::move(t));
		auto j = extra(*lp);
		CHECK(j.value("width", 0) == 1200 && j.contains("video") &&
			      j["video"].value("duration", 0) == 42,
		      "a story album: its photo and video icons");
		auto *img = image(lp->type_.get());
		CHECK(img && img->id_ == 80 &&
			      media_are(*lp, { { 81, "video", 0 } }),
		      "a story album: the photo icon as the image, the video at 0");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeTextCompositionStyle>();
		t->custom_emoji_id_ = 5368324170671202286LL;
		CHECK(extra(*preview(std::move(t))).value("custom_emoji_id", "") ==
			      "5368324170671202286",
		      "a text composition style: its custom emoji");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeTheme>();
		auto d = td_api::make_object<td_api::document>();
		d->document_ = file(82);
		t->documents_.push_back(std::move(d));
		t->settings_ = td_api::make_object<td_api::themeSettings>();
		t->settings_->base_theme_ =
			td_api::make_object<td_api::builtInThemeNight>();
		t->settings_->accent_color_ = 0;
		t->settings_->outgoing_message_fill_ =
			td_api::make_object<td_api::backgroundFillSolid>(0x112233);
		t->settings_->has_outgoing_message_accent_color_ = true;
		t->settings_->outgoing_message_accent_color_ = 0x445566;
		auto lp = preview(std::move(t));
		auto j = extra(*lp);
		CHECK(j.value("base_theme", "") == "night" &&
			      j.contains("accent_color") && j["accent_color"] == 0 &&
			      j.contains("outgoing_message_fill") &&
			      j.value("outgoing_message_accent_color", 0) == 0x445566,
		      "a theme: its settings");
		CHECK(link_preview_media_files(*lp).empty(),
		      "a theme: its files are not kept");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeUpgradedGift>();
		t->gift_ = td_api::make_object<td_api::upgradedGift>();
		t->gift_->id_ = 11;
		t->gift_->title_ = "Plush Pepe";
		t->gift_->name_ = "PlushPepe-12";
		t->gift_->number_ = 12;
		t->gift_->model_ = td_api::make_object<td_api::upgradedGiftModel>();
		t->gift_->model_->name_ = "Gold";
		t->gift_->model_->sticker_ = sticker(83);
		t->gift_->symbol_ = td_api::make_object<td_api::upgradedGiftSymbol>();
		t->gift_->symbol_->name_ = "Star";
		t->gift_->symbol_->sticker_ = sticker(84);
		auto lp = preview(std::move(t));
		auto j = extra(*lp);
		CHECK(j.contains("gift") &&
			      j["gift"].value("name", "") == "PlushPepe-12" &&
			      j["gift"].value("number", 0) == 12 &&
			      j["gift"]["model"].value("name", "") == "Gold",
		      "an upgraded gift: the gift's keys");
		CHECK(media_are(*lp, { { 83, "sticker", 0 }, { 84, "sticker", 1 } }),
		      "an upgraded gift: model sticker at 0, symbol at 1");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeVideoNote>();
		t->video_note_ = td_api::make_object<td_api::videoNote>();
		t->video_note_->duration_ = 10;
		t->video_note_->length_ = 240;
		t->video_note_->thumbnail_ = thumbnail(86);
		t->video_note_->video_ = file(85);
		auto lp = preview(std::move(t));
		auto j = extra(*lp);
		CHECK(j.value("duration", 0) == 10 && j.value("length", 0) == 240,
		      "a video note: duration and size");
		auto *img = image(lp->type_.get());
		CHECK(img && img->id_ == 86 &&
			      media_are(*lp, { { 85, "video_note", 0 } }),
		      "a video note: its thumbnail as the image, its video at 0");
	}
	{
		auto t = td_api::make_object<td_api::linkPreviewTypeVoiceNote>();
		t->voice_note_ = td_api::make_object<td_api::voiceNote>();
		t->voice_note_->duration_ = 5;
		t->voice_note_->mime_type_ = "audio/ogg";
		t->voice_note_->waveform_ = "abc";
		t->voice_note_->voice_ = file(87);
		auto lp = preview(std::move(t));
		auto j = extra(*lp);
		CHECK(j.value("duration", 0) == 5 &&
			      j.value("mime", "") == "audio/ogg" &&
			      j.value("waveform", "") == "YWJj",
		      "a voice note: duration, MIME type, waveform");
		CHECK(media_are(*lp, { { 87, "voice", 0 } }),
		      "a voice note: its voice at 0");
	}
	{
		/* Through the message, as the fetcher's MessageExtra source
		 * finds an album item. */
		auto t = td_api::make_object<td_api::linkPreviewTypeAlbum>();
		t->media_.push_back(
			td_api::make_object<td_api::linkPreviewAlbumMediaVideo>(video(88)));
		t->media_.push_back(
			td_api::make_object<td_api::linkPreviewAlbumMediaPhoto>(photo(89)));
		auto c = td_api::make_object<td_api::messageText>();
		c->text_ = td_api::make_object<td_api::formattedText>();
		c->link_preview_ = preview(std::move(t));
		auto x = message_extra_files(*c);
		CHECK(x.size() == 2 && x[1].file->id_ == 89 &&
			      !strcmp(x[1].role, "preview_media") &&
			      x[1].position == 1,
		      "a text message lists its album preview's items");
	}
}

int main()
{
	test_extra();
	test_message_links();
	test_media();
	test_type_names();
	test_new_types();
	if (g_failures) {
		printf("\n%d check(s) FAILED\n", g_failures);
		return 1;
	}
	printf("\nall checks passed\n");
	return 0;
}
