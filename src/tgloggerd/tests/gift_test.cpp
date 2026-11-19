// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * gift_test -- how gift service messages are stored, over hand-built
 * TDLib objects: no TDLib client. It checks the curated content_extra of
 * an upgraded gift (model, symbol, backdrop, original details) and of a
 * chat theme set from a gift, and the stickers kept as message files
 * (role "gift").
 */
#include <tgloggerd/MessageExtract.hpp>

#include <td/telegram/td_api.hpp>

#include <nlohmann/json.hpp>

#include <cstdio>
#include <cstring>
#include <initializer_list>
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

static td_api::object_ptr<td_api::sticker> sticker(int32_t id)
{
	auto s = td_api::make_object<td_api::sticker>();
	s->id_ = 1000 + id;
	s->emoji_ = "\xf0\x9f\x8e\x81";
	s->format_ = td_api::make_object<td_api::stickerFormatTgs>();
	s->sticker_ = file(id);
	return s;
}

static td_api::object_ptr<td_api::MessageSender> user(int64_t id)
{
	return td_api::make_object<td_api::messageSenderUser>(id);
}

static td_api::object_ptr<td_api::formattedText> text(const std::string &s)
{
	return td_api::make_object<td_api::formattedText>(
		s, td_api::array<td_api::object_ptr<td_api::textEntity>>());
}

/* An upgraded gift with a model (file 11), a symbol (file 12), a
 * backdrop and its original details. */
static td_api::object_ptr<td_api::upgradedGift> upgraded()
{
	auto g = td_api::make_object<td_api::upgradedGift>();
	g->id_ = 5000000000000000001LL;
	g->regular_gift_id_ = 5000000000000000002LL;
	g->title_ = "Plush Pepe";
	g->name_ = "PlushPepe-42";
	g->number_ = 42;
	g->owner_id_ = user(7);
	g->model_ = td_api::make_object<td_api::upgradedGiftModel>(
		"Aqua", sticker(11),
		td_api::make_object<td_api::upgradedGiftAttributeRarityPerMille>(15),
		true);
	g->symbol_ = td_api::make_object<td_api::upgradedGiftSymbol>(
		"Star", sticker(12),
		td_api::make_object<td_api::upgradedGiftAttributeRarityEpic>());
	g->backdrop_ = td_api::make_object<td_api::upgradedGiftBackdrop>(
		3, "Midnight",
		td_api::make_object<td_api::upgradedGiftBackdropColors>(
			0x112233, 0x000000, 0xabcdef, 0xffffff),
		td_api::make_object<td_api::upgradedGiftAttributeRarityPerMille>(0));
	g->original_details_ =
		td_api::make_object<td_api::upgradedGiftOriginalDetails>(
			user(8), user(9), text("happy birthday"), 1700000000);
	return g;
}

static nlohmann::json extra_of(const td_api::MessageContent &c)
{
	models::MessageContent out;
	extract_message_content(c, out);
	return nlohmann::json::parse(out.content_extra.value_or("null"),
				     nullptr, false);
}

static void test_upgraded_gift_keys(void)
{
	printf("test: the curated keys of an upgraded gift\n");
	auto c = td_api::make_object<td_api::messageUpgradedGift>();
	c->gift_ = upgraded();
	c->sender_id_ = user(8);
	c->receiver_id_ = user(7);
	c->origin_ = td_api::make_object<td_api::upgradedGiftOriginResale>();
	nlohmann::json j = extra_of(*c);
	CHECK(j.is_object() && j.contains("gift") && j["gift"].is_object(),
	      "the gift is an object");
	if (!j.is_object() || !j.contains("gift") || !j["gift"].is_object())
		return;
	const nlohmann::json &g = j["gift"];
	CHECK(g.value("name", "") == "PlushPepe-42" && g.value("number", 0) == 42,
	      "the existing keys stay");
	CHECK(g.contains("model") &&
		      g["model"] == nlohmann::json({ { "name", "Aqua" },
						     { "rarity_per_mille", 15 },
						     { "crafted", true } }),
	      "model {name, rarity_per_mille, crafted}");
	CHECK(g.contains("symbol") &&
		      g["symbol"] == nlohmann::json({ { "name", "Star" },
						      { "rarity", "epic" } }),
	      "symbol {name, rarity} for a named rarity");
	CHECK(g.contains("backdrop") &&
		      g["backdrop"] == nlohmann::json({
					       { "name", "Midnight" },
					       { "center_color", 0x112233 },
					       { "edge_color", 0 },
					       { "symbol_color", 0xabcdef },
					       { "text_color", 0xffffff },
					       { "rarity_per_mille", 0 } }),
	      "backdrop colours as RGB integers, black and a 0 per mille kept");
	CHECK(g.contains("original_details") &&
		      g["original_details"] ==
			      nlohmann::json({ { "sender", { { "user_id", 8 } } },
					       { "receiver", { { "user_id", 9 } } },
					       { "text", { { "text", "happy birthday" } } },
					       { "date", 1700000000 } }),
	      "original_details {sender, receiver, text, date}");
	CHECK(!g.dump().empty() && g.dump().find("remote-") == std::string::npos,
	      "no file reference in the JSON");

	/* The same gift in a purchase offer carries the same keys. */
	auto o = td_api::make_object<td_api::messageUpgradedGiftPurchaseOffer>();
	o->gift_ = upgraded();
	nlohmann::json oj = extra_of(*o);
	CHECK(oj.is_object() && oj.contains("gift") &&
		      oj["gift"].contains("model") &&
		      oj["gift"].contains("backdrop"),
	      "a purchase offer's gift has them too");

	/* An upgraded gift whose attributes TDLib leaves out. */
	auto bare = td_api::make_object<td_api::messageUpgradedGift>();
	bare->gift_ = td_api::make_object<td_api::upgradedGift>();
	bare->gift_->id_ = 77;
	nlohmann::json bj = extra_of(*bare);
	CHECK(bj.is_object() && bj.contains("gift") &&
		      !bj["gift"].contains("model") &&
		      !bj["gift"].contains("symbol") &&
		      !bj["gift"].contains("backdrop") &&
		      !bj["gift"].contains("original_details"),
	      "missing attributes leave their keys out");
}

static void test_gift_theme_keys(void)
{
	printf("test: the curated keys of a chat theme from a gift\n");
	auto c = td_api::make_object<td_api::messageChatSetTheme>();
	auto gt = td_api::make_object<td_api::giftChatTheme>();
	gt->gift_ = upgraded();
	c->theme_ = td_api::make_object<td_api::chatThemeGift>(std::move(gt));
	nlohmann::json j = extra_of(*c);
	CHECK(j.is_object() && j.value("theme_kind", "") == "gift",
	      "theme_kind stays \"gift\"");
	CHECK(j.is_object() && j.value("gift_id", "") == "5000000000000000001",
	      "gift_id as a string");
	CHECK(j.is_object() && j.value("gift_name", "") == "PlushPepe-42",
	      "gift_name");

	auto e = td_api::make_object<td_api::messageChatSetTheme>();
	e->theme_ = td_api::make_object<td_api::chatThemeEmoji>("Ocean");
	nlohmann::json ej = extra_of(*e);
	CHECK(ej.is_object() && ej.value("theme_name", "") == "Ocean" &&
		      !ej.contains("gift_id"),
	      "an emoji theme is unchanged");
}

/* Whether @x is exactly the role "gift" stickers @ids, at positions 0, 1... */
static bool gift_files(const std::vector<ExtraFile> &x,
		       std::initializer_list<int32_t> ids)
{
	if (x.size() != ids.size())
		return false;
	int pos = 0;
	for (int32_t id : ids) {
		const ExtraFile &f = x[pos];
		if (f.file->id_ != id || strcmp(f.role, "gift") ||
		    strcmp(f.category, "sticker") || f.position != pos)
			return false;
		pos++;
	}
	return true;
}

static td_api::object_ptr<td_api::gift> regular_gift(int32_t sticker_id)
{
	auto g = td_api::make_object<td_api::gift>();
	g->id_ = 6000000000000000001LL;
	g->star_count_ = 50;
	g->sticker_ = sticker(sticker_id);
	return g;
}

static void test_gift_files(void)
{
	printf("test: gift stickers as message files\n");
	{
		auto c = td_api::make_object<td_api::messageGift>();
		c->gift_ = regular_gift(21);
		CHECK(gift_files(message_extra_files(*c), { 21 }),
		      "a gift's sticker at position 0");
	}
	{
		auto c = td_api::make_object<td_api::messageRefundedUpgradedGift>();
		c->gift_ = regular_gift(22);
		CHECK(gift_files(message_extra_files(*c), { 22 }),
		      "a refunded upgraded gift's regular sticker");
	}
	{
		auto c = td_api::make_object<td_api::messageUpgradedGift>();
		c->gift_ = upgraded();
		CHECK(gift_files(message_extra_files(*c), { 11, 12 }),
		      "an upgraded gift's model at 0 and symbol at 1");
	}
	{
		auto c = td_api::make_object<td_api::messageUpgradedGift>();
		c->gift_ = upgraded();
		c->gift_->model_ = nullptr;
		auto x = message_extra_files(*c);
		CHECK(x.size() == 1 && x[0].file->id_ == 12 && x[0].position == 1,
		      "the symbol keeps position 1 without a model");
	}
	{
		auto c = td_api::make_object<td_api::messageUpgradedGiftPurchaseOffer>();
		c->gift_ = upgraded();
		CHECK(gift_files(message_extra_files(*c), { 11, 12 }),
		      "a purchase offer's gift stickers");
	}
	{
		auto c = td_api::make_object<td_api::messageUpgradedGiftPurchaseOfferRejected>();
		c->gift_ = upgraded();
		CHECK(gift_files(message_extra_files(*c), { 11, 12 }),
		      "a rejected offer's gift stickers");
	}
	{
		auto c = td_api::make_object<td_api::messageGiftedPremium>();
		c->sticker_ = sticker(31);
		CHECK(gift_files(message_extra_files(*c), { 31 }),
		      "a gifted Premium's animation sticker");
	}
	{
		auto c = td_api::make_object<td_api::messageGiftedStars>();
		c->sticker_ = sticker(32);
		CHECK(gift_files(message_extra_files(*c), { 32 }),
		      "gifted Stars' animation sticker");
	}
	{
		auto c = td_api::make_object<td_api::messageGiftedGrams>();
		c->sticker_ = sticker(33);
		CHECK(gift_files(message_extra_files(*c), { 33 }),
		      "gifted grams' animation sticker");
	}
	{
		auto c = td_api::make_object<td_api::messageGiveawayPrizeStars>();
		c->sticker_ = sticker(34);
		CHECK(gift_files(message_extra_files(*c), { 34 }),
		      "a giveaway prize's animation sticker");
	}
	{
		auto c = td_api::make_object<td_api::messageGiveaway>();
		c->sticker_ = sticker(35);
		CHECK(gift_files(message_extra_files(*c), { 35 }),
		      "a giveaway's prize sticker");
	}
	{
		auto c = td_api::make_object<td_api::messageGiftedPremium>();
		CHECK(message_extra_files(*c).empty(),
		      "no sticker, no file");
		auto g = td_api::make_object<td_api::messageGift>();
		CHECK(message_extra_files(*g).empty(), "no gift, no file");
	}
	{
		/* The code of a Premium gift code is never stored, nor is
		 * its sticker a gift file. */
		auto c = td_api::make_object<td_api::messagePremiumGiftCode>();
		c->code_ = "SECRETCODE";
		c->sticker_ = sticker(36);
		CHECK(message_extra_files(*c).empty(),
		      "a Premium gift code has no gift file");
		models::MessageContent out;
		extract_message_content(*c, out);
		CHECK(out.content_extra.value_or("").find("SECRETCODE") ==
				      std::string::npos &&
			      (!out.text.has_value() ||
			       out.text->find("SECRETCODE") == std::string::npos),
		      "and its code is not stored");
	}
}

int main(void)
{
	test_upgraded_gift_keys();
	test_gift_theme_keys();
	test_gift_files();

	if (g_failures) {
		printf("%d check(s) failed\n", g_failures);
		return 1;
	}
	printf("all checks passed\n");
	return 0;
}
