// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
/*
 * emoji_api_test -- POST /v1/custom-emoji and POST /v1/message-effects
 * over a real InternalApi on loopback, with stand-ins for the tables, the
 * file rows and TDLib that answer with hand-built td_api objects: no
 * TDLib client, no database.
 */
#include <tgloggerd/EmojiApi.hpp>
#include <tgloggerd/InternalApi.hpp>

#include <td/telegram/td_api.h>

#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include <cstdio>
#include <ctime>
#include <map>
#include <mutex>
#include <random>
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

/* 32 bytes of 0x01, base64. */
static const char kKey[] = "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=";

struct Reply {
	int		status = 0;
	nlohmann::json	body;
};

/* One HTTP/1.0 POST; the server closes the connection after it. */
static Reply post(int port, const std::string &path, const std::string &auth,
		  const std::string &body)
{
	Reply r;
	int fd = socket(AF_INET, SOCK_STREAM, 0);
	if (fd < 0)
		return r;

	sockaddr_in a{};
	a.sin_family = AF_INET;
	a.sin_port = htons(static_cast<uint16_t>(port));
	a.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
	if (connect(fd, reinterpret_cast<sockaddr *>(&a), sizeof(a)) < 0) {
		close(fd);
		return r;
	}

	std::string req = "POST " + path + " HTTP/1.0\r\nHost: 127.0.0.1\r\n"
		"Authorization: Bearer " + auth + "\r\n"
		"Content-Type: application/json\r\n"
		"Content-Length: " + std::to_string(body.size()) + "\r\n\r\n" + body;
	if (write(fd, req.data(), req.size()) != (ssize_t)req.size()) {
		close(fd);
		return r;
	}

	std::string all;
	char buf[4096];
	ssize_t n;
	while ((n = read(fd, buf, sizeof(buf))) > 0)
		all.append(buf, static_cast<size_t>(n));
	close(fd);

	if (sscanf(all.c_str(), "HTTP/1.%*d %d", &r.status) != 1)
		r.status = 0;
	size_t p = all.find("\r\n\r\n");
	if (p != std::string::npos)
		r.body = nlohmann::json::parse(all.substr(p + 4), nullptr, false);
	return r;
}

static td_api::object_ptr<td_api::file> remoteFile(int32_t id, const std::string &remote,
						 const std::string &unique, int64_t size)
{
	return td_api::make_object<td_api::file>(
		id, size, size, td_api::make_object<td_api::localFile>(),
		td_api::make_object<td_api::remoteFile>(remote, unique, false,
							 true, size));
}

static td_api::object_ptr<td_api::sticker>
emojiSticker(int64_t custom_emoji_id, const std::string &emoji, int64_t set_id,
	     td_api::object_ptr<td_api::StickerFormat> fmt, bool repaint,
	     const std::string &remote)
{
	return td_api::make_object<td_api::sticker>(
		custom_emoji_id + 1, set_id, 100, 100, emoji, std::move(fmt),
		td_api::make_object<td_api::stickerFullTypeCustomEmoji>(
			custom_emoji_id, repaint),
		nullptr, remoteFile(7, remote, "U" + remote, 4096));
}

static td_api::object_ptr<td_api::sticker>
plainSticker(td_api::object_ptr<td_api::StickerFormat> fmt, const std::string &remote,
	     td_api::object_ptr<td_api::file> premium = nullptr)
{
	return td_api::make_object<td_api::sticker>(
		1, 0, 512, 512, "x", std::move(fmt),
		td_api::make_object<td_api::stickerFullTypeRegular>(std::move(premium)),
		nullptr, remoteFile(8, remote, "U" + remote, 2048));
}

/* The stand-ins: the tables, the file rows and what TDLib answers. */
struct Fake {
	std::mutex				mtx;
	std::map<int64_t, models::CustomEmoji>	emoji;
	std::map<int64_t, models::MessageEffect> effects;
	std::map<std::string, uint64_t>		files;	/* unique id -> row */
	std::vector<models::File>		recorded;
	std::vector<std::vector<int64_t>>	emoji_asks;
	std::vector<int64_t>			effect_asks;
	/* Answer to getCustomEmojiStickers: an error code, 0 for stickers. */
	int32_t					emoji_error = 0;
	std::string				emoji_error_msg;
	int32_t					effect_error = 0;
};

static TDLib::TdObject answerEmoji(Fake &f, const td_api::getCustomEmojiStickers &q)
{
	std::vector<td_api::object_ptr<td_api::sticker>> out;
	for (int64_t id : q.custom_emoji_ids_) {
		if (id == 111)
			out.push_back(emojiSticker(111, "\xF0\x9F\x98\x80", 9001,
				td_api::make_object<td_api::stickerFormatTgs>(),
				false, "R111"));
		else if (id == 222)
			out.push_back(emojiSticker(222,
				/* 20 code points: kept to 16 */
				"aaaaaaaaaaaaaaa\xC3\xA9xxxx", 0,
				td_api::make_object<td_api::stickerFormatWebm>(),
				true, "R222"));
		else if (id == 444)
			out.push_back(emojiSticker(444, "\xE2\x9D\xA4", 9002,
				td_api::make_object<td_api::stickerFormatWebp>(),
				false, "R444"));
	}
	/* A sticker that was not asked for is ignored. */
	out.push_back(emojiSticker(999, "?", 1,
		td_api::make_object<td_api::stickerFormatTgs>(), false, "R999"));
	(void)f;
	return td_api::make_object<td_api::stickers>(std::move(out));
}

static TDLib::TdObject answerEffect(int64_t id)
{
	if (id == 5001) {
		return td_api::make_object<td_api::messageEffect>(
			5001, plainSticker(td_api::make_object<td_api::stickerFormatWebp>(), "Eicon"),
			"\xF0\x9F\x94\xA5", false,
			td_api::make_object<td_api::messageEffectTypeEmojiReaction>(
				plainSticker(td_api::make_object<td_api::stickerFormatTgs>(), "Esel"),
				plainSticker(td_api::make_object<td_api::stickerFormatTgs>(), "Eeff")));
	}
	if (id == 5002) {
		return td_api::make_object<td_api::messageEffect>(
			5002, nullptr, "\xF0\x9F\x8E\x89", true,
			td_api::make_object<td_api::messageEffectTypePremiumSticker>(
				plainSticker(td_api::make_object<td_api::stickerFormatWebm>(),
					     "Pstk", remoteFile(9, "Pani", "UPani", 999))));
	}
	return td_api::make_object<td_api::error>(404, "Not Found");
}

int main(void)
{
	Fake fake;
	EmojiBackend be;
	be.load_emoji = [&](const std::vector<int64_t> &ids) {
		std::lock_guard<std::mutex> lk(fake.mtx);
		std::vector<models::CustomEmoji> out;
		for (int64_t id : ids)
			if (fake.emoji.count(id))
				out.push_back(fake.emoji[id]);
		return out;
	};
	be.save_emoji = [&](const models::CustomEmoji &e) {
		std::lock_guard<std::mutex> lk(fake.mtx);
		fake.emoji[e.custom_emoji_id] = e;
	};
	be.load_effects = [&](const std::vector<int64_t> &ids) {
		std::lock_guard<std::mutex> lk(fake.mtx);
		std::vector<models::MessageEffect> out;
		for (int64_t id : ids)
			if (fake.effects.count(id))
				out.push_back(fake.effects[id]);
		return out;
	};
	be.save_effect = [&](const models::MessageEffect &e) {
		std::lock_guard<std::mutex> lk(fake.mtx);
		fake.effects[e.effect_id] = e;
	};
	be.record_file = [&](const models::File &f) {
		std::lock_guard<std::mutex> lk(fake.mtx);
		fake.recorded.push_back(f);
		auto it = fake.files.find(f.tg_unique_id);
		if (it != fake.files.end())
			return it->second;
		uint64_t id = 100 + fake.files.size();
		fake.files[f.tg_unique_id] = id;
		return id;
	};
	be.request = [&](TDLib::TdFunction fn, std::chrono::milliseconds) -> TDLib::TdObject {
		std::lock_guard<std::mutex> lk(fake.mtx);
		if (fn->get_id() == td_api::getCustomEmojiStickers::ID) {
			auto &q = static_cast<const td_api::getCustomEmojiStickers &>(*fn);
			fake.emoji_asks.push_back(q.custom_emoji_ids_);
			if (fake.emoji_error)
				return td_api::make_object<td_api::error>(
					fake.emoji_error, fake.emoji_error_msg);
			return answerEmoji(fake, q);
		}
		if (fn->get_id() == td_api::getMessageEffect::ID) {
			auto &q = static_cast<const td_api::getMessageEffect &>(*fn);
			fake.effect_asks.push_back(q.effect_id_);
			if (fake.effect_error)
				return td_api::make_object<td_api::error>(
					fake.effect_error, "Too Many Requests: retry after 30");
			return answerEffect(q.effect_id_);
		}
		return td_api::make_object<td_api::error>(500, "unexpected request");
	};

	InternalApi api(nullptr);
	registerEmojiApi(api, be);

	std::mt19937 rng(static_cast<unsigned>(std::time(nullptr)) ^
			 static_cast<unsigned>(getpid()));
	int port = 0;
	for (int i = 0; i < 20 && !port; i++) {
		int p = 20000 + static_cast<int>(rng() % 40000);
		InternalApi::Config cfg;
		cfg.listen = "127.0.0.1:" + std::to_string(p);
		cfg.threads = 2;
		cfg.web_app_key = kKey;
		if (api.start(cfg) == 0)
			port = p;
	}
	if (!port) {
		printf("FAIL: could not bind a loopback port\n");
		return 1;
	}
	const std::string bearer = InternalApi::deriveBearer(kKey);

	printf("POST /v1/custom-emoji, nothing stored yet\n");
	Reply r = post(port, "/v1/custom-emoji", bearer,
		       R"({"ids": ["111", 222, "333", "111"]})");
	CHECK(r.status == 200, "answers 200");
	const nlohmann::json &j = r.body;
	CHECK(j.is_object() && j.value("outcome", "") == "ok", "outcome ok");
	CHECK(fake.emoji_asks.size() == 1 &&
	      fake.emoji_asks[0] == std::vector<int64_t>({ 111, 222, 333 }),
	      "one getCustomEmojiStickers for the three distinct ids");
	CHECK(j.value("asked", -1) == 3, "asked says 3");
	CHECK(j.contains("emoji") && j["emoji"].size() == 2, "two rows");
	if (j.contains("emoji") && j["emoji"].size() == 2) {
		const auto &a = j["emoji"][0], &b = j["emoji"][1];
		CHECK(a["custom_emoji_id"] == "111" && b["custom_emoji_id"] == "222",
		      "in the order asked, ids as strings");
		CHECK(a["emoji"] == "\xF0\x9F\x98\x80" && a["set_id"] == "9001" &&
		      a["format"] == "tgs" && a["needs_repainting"] == false,
		      "the first sticker's fields");
		CHECK(b["set_id"].is_null() && b["format"] == "webm" &&
		      b["needs_repainting"] == true,
		      "no set is null; needs_repainting kept");
		CHECK(b["emoji"] == "aaaaaaaaaaaaaaa\xC3\xA9",
		      "the emoji cut to 16 code points");
		CHECK(a["file_id"].is_number_unsigned() &&
		      b["file_id"].is_number_unsigned() && a["file_id"] != b["file_id"],
		      "each sticker has a file row");
	}
	CHECK(j["missing"] == nlohmann::json::array({ "333" }),
	      "the unknown id is missing");
	CHECK(fake.emoji.size() == 2 && !fake.emoji.count(999),
	      "two rows stored, not the sticker nobody asked for");
	CHECK(fake.recorded.size() == 2 && fake.recorded[0].file_type == "sticker" &&
	      !fake.recorded[0].on_disk && fake.recorded[0].tg_file_id == "R111" &&
	      fake.recorded[0].tg_unique_id == "UR111" &&
	      fake.recorded[0].file_ext == std::optional<std::string>("tgs") &&
	      fake.recorded[0].file_size == 4096,
	      "the file rows are metadata-only sticker rows");

	printf("POST /v1/custom-emoji again\n");
	r = post(port, "/v1/custom-emoji", bearer, R"({"ids": ["222", "111", "333"]})");
	CHECK(r.status == 200 && r.body.value("outcome", "") == "ok", "answers ok");
	CHECK(fake.emoji_asks.size() == 1, "TDLib is asked for none");
	CHECK(r.body.value("asked", -1) == 0, "asked says 0");
	CHECK(r.body["emoji"].size() == 2 &&
	      r.body["emoji"][0]["custom_emoji_id"] == "222",
	      "both rows, from the table, in the order asked");
	CHECK(r.body["missing"] == nlohmann::json::array({ "333" }),
	      "the unknown id still missing, from memory");

	printf("POST /v1/custom-emoji while Telegram asks to wait\n");
	fake.emoji_error = 429;
	fake.emoji_error_msg = "Too Many Requests: retry after 42";
	r = post(port, "/v1/custom-emoji", bearer, R"({"ids": [111, 444]})");
	CHECK(r.status == 200 && r.body.value("outcome", "") == "flood_wait" &&
	      r.body.value("retry_after", 0) == 42, "flood_wait with retry_after");
	CHECK(fake.emoji_asks.size() == 2 &&
	      fake.emoji_asks[1] == std::vector<int64_t>({ 444 }),
	      "only the id not stored was asked for");
	CHECK(r.body["emoji"].size() == 1 && r.body["missing"].empty(),
	      "the stored row is answered; the other is not missing");

	printf("POST /v1/custom-emoji while TDLib is not ready\n");
	fake.emoji_error = TDLib::kRequestNotReady;
	fake.emoji_error_msg = "not authorized";
	r = post(port, "/v1/custom-emoji", bearer, R"({"ids": [444]})");
	CHECK(r.status == 200 && r.body.value("outcome", "") == "unavailable",
	      "outcome unavailable");
	fake.emoji_error = 0;
	r = post(port, "/v1/custom-emoji", bearer, R"({"ids": [444]})");
	CHECK(r.status == 200 && r.body.value("outcome", "") == "ok" &&
	      r.body["emoji"].size() == 1, "then filled once TDLib answers");

	printf("POST /v1/custom-emoji with bad bodies\n");
	std::string many = "{\"ids\": [";
	for (int i = 1; i <= 201; i++)
		many += (i > 1 ? "," : "") + std::to_string(i);
	many += "]}";
	for (const std::string &bad : { std::string("{}"), std::string("[]"),
		std::string(R"({"ids": "111"})"), std::string(R"({"ids": [0]})"),
		std::string(R"({"ids": ["abc"]})"), std::string(R"({"ids": ["12x"]})"),
		std::string(R"({"ids": [" 1"]})"), std::string(R"({"ids": [1.5]})"),
		std::string(R"({"ids": [18446744073709551615]})"), many }) {
		r = post(port, "/v1/custom-emoji", bearer, bad);
		CHECK(r.status == 400, ("400 for " + bad.substr(0, 40)).c_str());
	}
	r = post(port, "/v1/custom-emoji", bearer, R"({"ids": []})");
	CHECK(r.status == 200 && r.body["emoji"].empty() && r.body.value("asked", -1) == 0,
	      "no ids: an empty answer");
	CHECK(fake.emoji_asks.size() == 4, "and no TDLib call for any of them");

	printf("POST /v1/message-effects\n");
	r = post(port, "/v1/message-effects", bearer,
		 R"({"ids": ["5001", 5002, "5003"]})");
	CHECK(r.status == 200 && r.body.value("outcome", "") == "ok", "answers ok");
	CHECK(fake.effect_asks == std::vector<int64_t>({ 5001, 5002, 5003 }),
	      "one getMessageEffect per effect");
	CHECK(r.body["effects"].size() == 2 &&
	      r.body["missing"] == nlohmann::json::array({ "5003" }),
	      "two effects, the unknown one missing");
	if (r.body["effects"].size() == 2) {
		const auto &a = r.body["effects"][0], &b = r.body["effects"][1];
		CHECK(a["effect_id"] == "5001" && a["type"] == "emoji_reaction" &&
		      a["is_premium"] == false && a["emoji"] == "\xF0\x9F\x94\xA5",
		      "the reaction effect");
		CHECK(a["static_icon_file_id"].is_number() &&
		      a["select_animation_file_id"].is_number() &&
		      a["effect_animation_file_id"].is_number() &&
		      a["sticker_file_id"].is_null() && a["sticker_format"].is_null(),
		      "its icon and two animations");
		CHECK(b["type"] == "premium_sticker" && b["is_premium"] == true &&
		      b["static_icon_file_id"].is_null() &&
		      b["select_animation_file_id"].is_null() &&
		      b["sticker_file_id"].is_number() &&
		      b["effect_animation_file_id"].is_number() &&
		      b["sticker_format"] == "webm",
		      "the premium sticker effect and its animation");
	}
	bool tgs = false;
	for (const auto &f : fake.recorded)
		if (f.tg_unique_id == "UPani")
			tgs = f.file_ext == std::optional<std::string>("tgs") &&
			      f.file_type == "sticker";
	CHECK(tgs, "the premium animation is recorded as a TGS sticker file");

	printf("POST /v1/message-effects again\n");
	fake.effect_error = 429;
	r = post(port, "/v1/message-effects", bearer, R"({"ids": [5002, 5001, 5003]})");
	CHECK(r.status == 200 && r.body.value("outcome", "") == "ok" &&
	      r.body.value("asked", -1) == 0, "answered without TDLib");
	CHECK(fake.effect_asks.size() == 3, "TDLib is asked for none");
	CHECK(r.body["effects"].size() == 2 &&
	      r.body["effects"][0]["effect_id"] == "5002",
	      "in the order asked");

	printf("POST /v1/message-effects while Telegram asks to wait\n");
	r = post(port, "/v1/message-effects", bearer, R"({"ids": [6001, 6002]})");
	CHECK(r.status == 200 && r.body.value("outcome", "") == "flood_wait" &&
	      r.body.value("retry_after", 0) == 30, "flood_wait with retry_after");
	CHECK(fake.effect_asks.size() == 4, "stops at the first failure");
	CHECK(r.body["missing"].empty(), "nothing taken as missing");
	fake.effect_error = 0;

	std::string many_eff = "{\"ids\": [";
	for (int i = 1; i <= 51; i++)
		many_eff += (i > 1 ? "," : "") + std::to_string(i);
	many_eff += "]}";
	r = post(port, "/v1/message-effects", bearer, many_eff);
	CHECK(r.status == 400, "400 for 51 effects");

	api.stop();

	if (g_failures) {
		printf("%d check(s) failed\n", g_failures);
		return 1;
	}
	printf("all checks passed\n");
	return 0;
}
