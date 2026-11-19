// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * extra_files_test -- message_extra_files() over hand-built TDLib
 * messages: no TDLib client. It checks which thumbnails of a message's
 * videos are kept as role "thumbnail" (still images only, and for
 * stickers only the video ones).
 */
#include <tgloggerd/MessageExtract.hpp>

#include <td/telegram/td_api.hpp>

#include <cstdio>
#include <cstring>
#include <string>

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

static td_api::object_ptr<td_api::thumbnail>
thumb(td_api::object_ptr<td_api::ThumbnailFormat> format, int32_t id)
{
	return td_api::make_object<td_api::thumbnail>(std::move(format), 320,
						      180, file(id));
}

static td_api::object_ptr<td_api::thumbnail> jpeg(int32_t id)
{
	return thumb(td_api::make_object<td_api::thumbnailFormatJpeg>(), id);
}

static td_api::object_ptr<td_api::MessageContent>
video(td_api::object_ptr<td_api::thumbnail> t)
{
	auto c = td_api::make_object<td_api::messageVideo>();
	c->video_ = td_api::make_object<td_api::video>();
	c->video_->thumbnail_ = std::move(t);
	c->video_->video_ = file(1);
	return c;
}

static td_api::object_ptr<td_api::MessageContent>
sticker(td_api::object_ptr<td_api::StickerFormat> format,
	td_api::object_ptr<td_api::thumbnail> t)
{
	auto c = td_api::make_object<td_api::messageSticker>();
	c->sticker_ = td_api::make_object<td_api::sticker>();
	c->sticker_->format_ = std::move(format);
	c->sticker_->thumbnail_ = std::move(t);
	c->sticker_->sticker_ = file(1);
	return c;
}

/* Whether @c's extra files are exactly one thumbnail, file @id. */
static bool one_thumbnail(const td_api::MessageContent &c, int32_t id)
{
	auto x = message_extra_files(c);
	return x.size() == 1 && x[0].file->id_ == id &&
	       !strcmp(x[0].role, "thumbnail") && x[0].position == 0 &&
	       !strcmp(x[0].category, "thumbnail");
}

int main(void)
{
	printf("test: still thumbnails of videos\n");
	CHECK(one_thumbnail(*video(jpeg(2)), 2), "a video's JPEG thumbnail");
	CHECK(one_thumbnail(*video(thumb(
		      td_api::make_object<td_api::thumbnailFormatPng>(), 3)), 3),
	      "a PNG one");
	CHECK(one_thumbnail(*video(thumb(
		      td_api::make_object<td_api::thumbnailFormatWebp>(), 4)), 4),
	      "a WebP one");
	CHECK(message_extra_files(*video(thumb(
		      td_api::make_object<td_api::thumbnailFormatMpeg4>(), 5)))
		      .empty(),
	      "an MPEG-4 thumbnail is not kept");
	CHECK(message_extra_files(*video(thumb(
		      td_api::make_object<td_api::thumbnailFormatGif>(), 6)))
		      .empty(),
	      "a GIF thumbnail is not kept");
	CHECK(message_extra_files(*video(nullptr)).empty(),
	      "a video without a thumbnail has no extra file");

	{
		auto c = video(jpeg(7));
		auto &v = static_cast<td_api::messageVideo &>(*c);
		v.cover_ = td_api::make_object<td_api::photo>();
		v.cover_->sizes_.push_back(td_api::make_object<td_api::photoSize>(
			"x", file(8), 1280, 720,
			td_api::array<td_api::int32>()));
		auto x = message_extra_files(*c);
		CHECK(x.size() == 2 && !strcmp(x[0].role, "cover") &&
			      x[0].file->id_ == 8 &&
			      !strcmp(x[1].role, "thumbnail") &&
			      x[1].file->id_ == 7,
		      "a video with a cover keeps both, the cover first");
	}

	{
		auto c = td_api::make_object<td_api::messageAnimation>();
		c->animation_ = td_api::make_object<td_api::animation>();
		c->animation_->thumbnail_ = jpeg(9);
		c->animation_->animation_ = file(1);
		CHECK(one_thumbnail(*c, 9), "an animation's thumbnail");
	}
	{
		auto c = td_api::make_object<td_api::messageVideoNote>();
		c->video_note_ = td_api::make_object<td_api::videoNote>();
		c->video_note_->thumbnail_ = jpeg(10);
		c->video_note_->video_ = file(1);
		CHECK(one_thumbnail(*c, 10), "a video note's thumbnail");
	}

	CHECK(one_thumbnail(*sticker(
		      td_api::make_object<td_api::stickerFormatWebm>(),
		      thumb(td_api::make_object<td_api::thumbnailFormatWebp>(),
			    11)), 11),
	      "a video sticker's WebP thumbnail");
	CHECK(message_extra_files(*sticker(
		      td_api::make_object<td_api::stickerFormatWebm>(),
		      thumb(td_api::make_object<td_api::thumbnailFormatWebm>(),
			    12)))
		      .empty(),
	      "a video sticker's WebM thumbnail is not kept");
	CHECK(message_extra_files(*sticker(
		      td_api::make_object<td_api::stickerFormatWebp>(),
		      thumb(td_api::make_object<td_api::thumbnailFormatWebp>(),
			    13)))
		      .empty(),
	      "a .webp sticker keeps no thumbnail: it is an image");
	CHECK(message_extra_files(*sticker(
		      td_api::make_object<td_api::stickerFormatTgs>(),
		      thumb(td_api::make_object<td_api::thumbnailFormatTgs>(),
			    14)))
		      .empty(),
	      "a .tgs sticker keeps no thumbnail");

	{
		auto c = td_api::make_object<td_api::messageDocument>();
		c->document_ = td_api::make_object<td_api::document>();
		c->document_->thumbnail_ = jpeg(15);
		c->document_->document_ = file(1);
		CHECK(message_extra_files(*c).empty(),
		      "a document's thumbnail is not kept");
	}

	if (g_failures) {
		printf("\n%d check(s) FAILED\n", g_failures);
		return 1;
	}
	printf("\nall checks passed\n");
	return 0;
}
