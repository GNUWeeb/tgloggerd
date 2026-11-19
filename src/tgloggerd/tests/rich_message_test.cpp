// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * rich_message_test -- how a messageRichMessage is stored, over
 * hand-built TDLib objects: no TDLib client. It checks the plain text of
 * the page blocks and the curated content_extra beside TDLib's JSON.
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
rich(Blocks blocks, bool is_rtl = false)
{
	auto c = td_api::make_object<td_api::messageRichMessage>();
	c->message_ = td_api::make_object<td_api::richMessage>(
		std::move(blocks), is_rtl, true);
	return c;
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

int main(void)
{
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

	if (g_failures) {
		printf("\n%d check(s) FAILED\n", g_failures);
		return 1;
	}
	printf("\nall checks passed\n");
	return 0;
}
