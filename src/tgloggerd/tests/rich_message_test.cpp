// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * rich_message_test -- how a messageRichMessage is stored, over
 * hand-built TDLib objects: no TDLib client. It checks the plain text of
 * the page blocks, the curated content_extra beside TDLib's JSON, and the
 * media of the blocks as message files (role "rich").
 */
#include <tgloggerd/MessageExtract.hpp>

#include <td/telegram/td_api.hpp>

#include <nlohmann/json.hpp>

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

using Block = td_api::object_ptr<td_api::PageBlock>;
using Blocks = std::vector<Block>;

static td_api::object_ptr<td_api::RichText> plain(const std::string &s)
{
	return td_api::make_object<td_api::richTextPlain>(s);
}

static td_api::object_ptr<td_api::RichText> bold(const std::string &s)
{
	return td_api::make_object<td_api::richTextBold>(plain(s));
}

static Block paragraph(td_api::object_ptr<td_api::RichText> t)
{
	return td_api::make_object<td_api::pageBlockParagraph>(std::move(t));
}

static td_api::object_ptr<td_api::pageBlockCaption>
caption(const std::string &s)
{
	return td_api::make_object<td_api::pageBlockCaption>(plain(s), nullptr);
}

static td_api::object_ptr<td_api::messageRichMessage>
rich(Blocks blocks, bool is_rtl = false, bool is_full = true)
{
	auto c = td_api::make_object<td_api::messageRichMessage>();
	c->message_ = td_api::make_object<td_api::richMessage>(
		std::move(blocks), is_rtl, is_full);
	return c;
}

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

/* A photo of two sizes, @small_id the smaller, @big_id the larger. */
static td_api::object_ptr<td_api::photo> photo(int32_t small_id, int32_t big_id)
{
	auto p = td_api::make_object<td_api::photo>();
	p->sizes_.push_back(td_api::make_object<td_api::photoSize>(
		"m", file(small_id), 320, 240, td_api::array<td_api::int32>()));
	p->sizes_.push_back(td_api::make_object<td_api::photoSize>(
		"y", file(big_id), 1280, 960, td_api::array<td_api::int32>()));
	return p;
}

static Block photo_block(int32_t small_id, int32_t big_id)
{
	return td_api::make_object<td_api::pageBlockPhoto>(
		photo(small_id, big_id), nullptr, "", false);
}

static Block video_block(int32_t id)
{
	auto v = td_api::make_object<td_api::video>();
	v->file_name_ = "clip.mp4";
	v->video_ = file(id);
	return td_api::make_object<td_api::pageBlockVideo>(
		std::move(v), nullptr, false, false, false);
}

/* A page with media at the top level and nested: photo, collage (video,
 * photo), list item (document), voice note, divider, animation, audio. */
static td_api::object_ptr<td_api::messageRichMessage> media_sample(void)
{
	Blocks b;
	b.push_back(photo_block(1, 2));

	Blocks collage;
	collage.push_back(video_block(3));
	collage.push_back(photo_block(4, 5));
	b.push_back(td_api::make_object<td_api::pageBlockCollage>(
		std::move(collage), caption("two")));

	auto doc = td_api::make_object<td_api::document>();
	doc->file_name_ = "notes.pdf";
	doc->document_ = file(6);
	Blocks ib;
	ib.push_back(td_api::make_object<td_api::pageBlockDocument>(
		std::move(doc), nullptr));
	td_api::array<td_api::object_ptr<td_api::pageBlockListItem>> items;
	items.push_back(td_api::make_object<td_api::pageBlockListItem>(
		"1.", std::move(ib), false, false, 1, "1"));
	b.push_back(td_api::make_object<td_api::pageBlockList>(std::move(items)));

	auto voice = td_api::make_object<td_api::voiceNote>();
	voice->voice_ = file(7);
	b.push_back(td_api::make_object<td_api::pageBlockVoiceNote>(
		std::move(voice), nullptr));
	b.push_back(td_api::make_object<td_api::pageBlockDivider>());

	auto anim = td_api::make_object<td_api::animation>();
	anim->file_name_ = "loop.gif.mp4";
	anim->animation_ = file(8);
	b.push_back(td_api::make_object<td_api::pageBlockAnimation>(
		std::move(anim), nullptr, true, false));

	auto audio = td_api::make_object<td_api::audio>();
	audio->file_name_ = "song.mp3";
	audio->audio_ = file(9);
	b.push_back(td_api::make_object<td_api::pageBlockAudio>(
		std::move(audio), nullptr));

	/* No file: a photo block whose photo is missing. */
	b.push_back(td_api::make_object<td_api::pageBlockPhoto>(
		nullptr, nullptr, "", false));
	return rich(std::move(b));
}

/* Whether @x is file @id of @category at @position, role "rich". */
static bool is_rich(const ExtraFile &x, int32_t id, const char *category,
		    int position)
{
	return x.file && x.file->id_ == id && !strcmp(x.role, "rich") &&
	       !strcmp(x.category, category) && x.position == position;
}

/* A page with one block of each text-bearing kind that matters. */
static td_api::object_ptr<td_api::messageRichMessage> sample(void)
{
	Blocks b;
	b.push_back(td_api::make_object<td_api::pageBlockTitle>(
		plain("Release notes")));

	td_api::array<td_api::object_ptr<td_api::RichText>> parts;
	parts.push_back(plain("Now with "));
	parts.push_back(bold("tables"));
	parts.push_back(td_api::make_object<td_api::richTextCustomEmoji>(
		42, "!"));
	b.push_back(paragraph(
		td_api::make_object<td_api::richTexts>(std::move(parts))));

	td_api::array<td_api::object_ptr<td_api::pageBlockListItem>> items;
	for (const char *s : { "first", "second" }) {
		Blocks ib;
		ib.push_back(paragraph(plain(s)));
		items.push_back(td_api::make_object<td_api::pageBlockListItem>(
			"-", std::move(ib), false, false, 0, ""));
	}
	b.push_back(td_api::make_object<td_api::pageBlockList>(std::move(items)));

	td_api::array<td_api::array<td_api::object_ptr<td_api::pageBlockTableCell>>>
		cells(1);
	for (const char *s : { "a", "", "b" })
		cells[0].push_back(td_api::make_object<td_api::pageBlockTableCell>(
			plain(s), false, 1, 1, nullptr, nullptr));
	b.push_back(td_api::make_object<td_api::pageBlockTable>(
		plain("Table"), std::move(cells), false, false, false));

	b.push_back(td_api::make_object<td_api::pageBlockDivider>());
	b.push_back(td_api::make_object<td_api::pageBlockPhoto>(
		nullptr, caption("A photo"), "", false));
	return rich(std::move(b), true);
}

/* A widget as TDLib gives it. */
static td_api::object_ptr<td_api::messageWidget>
widget(int32_t w, int32_t h, int64_t id, const std::string &caption)
{
	auto c = td_api::make_object<td_api::messageWidget>();
	c->width_ = w;
	c->height_ = h;
	c->html_ = "<script>alert(1)</script><b>hi</b>";
	c->widget_id_ = id;
	c->caption_ = td_api::make_object<td_api::formattedText>(
		caption, std::vector<td_api::object_ptr<td_api::textEntity>>());
	return c;
}

static void test_widget(void)
{
	printf("test: a widget's content_extra\n");
	{
		auto c = widget(320, 200, 1234567890123456789LL, "Live score");
		models::MessageContent out;
		extract_message_content(*c, out);
		CHECK(out.content_type == models::MessageContentType::Widget,
		      "stored as widget");
		CHECK(out.text.value_or("") == "Live score",
		      "the caption is the text");
		nlohmann::json j = nlohmann::json::parse(
			out.content_extra.value_or("null"), nullptr, false);
		CHECK(j.is_object() && j.value("width", -1) == 320 &&
			      j.value("height", -1) == 200,
		      "width and height");
		CHECK(j.is_object() &&
			      j.value("widget_id", "") == "1234567890123456789",
		      "widget_id as a string");
		CHECK(j.is_object() && j.value("has_caption", false),
		      "has_caption");
		CHECK(j.is_object() && j.contains("tdlib") &&
			      j["tdlib"].is_object() &&
			      j["tdlib"].value("html", "").find("<script>") !=
				      std::string::npos,
		      "TDLib's JSON beside it holds the HTML as given");
	}
	{
		auto c = widget(0, 0, 0, "");
		models::MessageContent out;
		extract_message_content(*c, out);
		nlohmann::json j = nlohmann::json::parse(
			out.content_extra.value_or("null"), nullptr, false);
		CHECK(j.is_object() && j.value("width", -1) == 0 &&
			      j.value("height", -1) == 0,
		      "a full-width, unfixed widget keeps 0 and 0");
		CHECK(j.is_object() && !j.contains("widget_id"),
		      "no widget_id when it is not shared");
		CHECK(j.is_object() && !j.value("has_caption", true),
		      "has_caption false without a caption");
		CHECK(!out.text.has_value(), "no caption, no text");
	}
}

int main(void)
{
	test_widget();

	printf("test: which rich messages are asked for in full\n");
	{
		auto whole = rich(Blocks());
		CHECK(!rich_message_is_partial(*whole),
		      "a full rich message is not asked for again");
		auto part = rich(Blocks(), false, false);
		CHECK(rich_message_is_partial(*part),
		      "one delivered without all its blocks is");
		auto empty = td_api::make_object<td_api::messageRichMessage>();
		CHECK(!rich_message_is_partial(*empty),
		      "nor is one TDLib gave no blocks object at all");
		auto text = td_api::make_object<td_api::messageText>();
		CHECK(!rich_message_is_partial(*text), "nor any other content");
	}

	printf("test: the text of a rich message\n");
	{
		auto c = sample();
		models::MessageContent out;
		extract_message_content(*c, out);
		CHECK(out.content_type == models::MessageContentType::Rich,
		      "stored as rich");
		CHECK(out.tdlib_type.value_or("") == "messageRichMessage",
		      "with its class");
		CHECK(out.text.has_value() &&
			      *out.text == "Release notes\n"
					   "Now with tables!\n"
					   "- first\n"
					   "- second\n"
					   "Table\n"
					   "a | b\n"
					   "A photo",
		      "the blocks' plain text, one line each, in reading order");
		CHECK(!out.entities.has_value(), "no entities");
	}
	{
		Blocks b;
		b.push_back(td_api::make_object<td_api::pageBlockDivider>());
		auto c = rich(std::move(b));
		models::MessageContent out;
		extract_message_content(*c, out);
		CHECK(!out.text.has_value(), "blocks without text leave it NULL");
	}

	printf("test: the content_extra of a rich message\n");
	{
		auto c = sample();
		models::MessageContent out;
		extract_message_content(*c, out);
		nlohmann::json j = nlohmann::json::parse(
			out.content_extra.value_or("null"), nullptr, false);
		CHECK(j.is_object() && j.value("blocks", -1) == 6,
		      "\"blocks\" counts the top-level blocks");
		CHECK(j.is_object() && j.value("is_rtl", false),
		      "\"is_rtl\" as TDLib says");
		CHECK(j.is_object() && j.contains("tdlib") &&
			      j["tdlib"].value("@type", "") ==
				      "messageRichMessage" &&
			      j["tdlib"].contains("message"),
		      "TDLib's JSON stays beside them");
		CHECK(j.is_object() && !j.contains("tdlib_omitted_bytes"),
		      "a small message is not capped");
	}
	{
		/* Over the 256 KiB JSON cap: the class only, but the curated
		 * keys and the text still come from the live object. */
		std::string big(300 * 1024, 'x');
		Blocks b;
		b.push_back(paragraph(plain("word " + big)));
		auto c = rich(std::move(b));
		models::MessageContent out;
		extract_message_content(*c, out);
		nlohmann::json j = nlohmann::json::parse(
			out.content_extra.value_or("null"), nullptr, false);
		CHECK(j.is_object() && j.value("blocks", -1) == 1 &&
			      j.contains("is_rtl") && !j.value("is_rtl", true),
		      "a capped message keeps blocks and is_rtl");
		CHECK(j.is_object() && j.contains("tdlib_omitted_bytes") &&
			      j["tdlib"].size() == 1,
		      "and only the class of TDLib's JSON");
		CHECK(out.text.has_value() && out.text->rfind("word ", 0) == 0,
		      "and its text");
	}

	printf("test: the media of a rich message\n");
	{
		auto c = media_sample();
		auto x = message_extra_files(*c);
		CHECK(x.size() == 7, "one file per media item");
		CHECK(x.size() == 7 && is_rich(x[0], 2, "photo", 0) &&
			      is_rich(x[1], 3, "video", 1) &&
			      is_rich(x[2], 5, "photo", 2) &&
			      is_rich(x[3], 6, "document", 3) &&
			      is_rich(x[4], 7, "voice", 4) &&
			      is_rich(x[5], 8, "animation", 5) &&
			      is_rich(x[6], 9, "audio", 6),
		      "in order of appearance, nested ones in place, a photo"
		      " by its largest size");
		CHECK(x.size() == 7 && x[1].file_name == "clip.mp4" &&
			      x[3].file_name == "notes.pdf",
		      "with their file names");

		const ExtraFile *f = rich_file_with_unique_id(x, "unique-6");
		CHECK(f && f->file->id_ == 6, "found again by unique id");
		CHECK(!rich_file_with_unique_id(x, "unique-1"),
		      "a smaller photo size is not one of them");
		CHECK(!rich_file_with_unique_id(x, ""),
		      "an empty unique id finds nothing");
	}
	{
		auto c = sample();
		CHECK(message_extra_files(*c).empty(),
		      "a photo block without a photo has no file");
	}
	{
		Blocks b;
		b.push_back(td_api::make_object<td_api::pageBlockCover>(
			photo_block(10, 11)));
		Blocks inner;
		inner.push_back(video_block(12));
		b.push_back(td_api::make_object<td_api::pageBlockDetails>(
			plain("More"), std::move(inner), false));
		auto c = rich(std::move(b));
		auto x = message_extra_files(*c);
		CHECK(x.size() == 2 && is_rich(x[0], 11, "photo", 0) &&
			      is_rich(x[1], 12, "video", 1),
		      "a cover's photo and a collapsible block's video");
	}

	if (g_failures) {
		printf("\n%d check(s) FAILED\n", g_failures);
		return 1;
	}
	printf("\nall checks passed\n");
	return 0;
}
