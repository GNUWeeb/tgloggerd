// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#include <tgloggerd/helpers/common.h>
#include <tgloggerd/TgLoggerd.hpp>
#include <stdexcept>
#include <cstdlib>
#include <cstring>
#include <cstdio>
#include <cctype>
#include <fstream>
#include <algorithm>
#include <filesystem>

#include <openssl/evp.h>

namespace fs = std::filesystem;

namespace tgloggerd {

namespace {

/* Compute the lowercase hex SHA-256 digest of a file's contents. */
std::optional<std::string> sha256_file_hex(const std::string &path)
{
	std::ifstream f(path, std::ios::binary);
	if (!f)
		return std::nullopt;

	EVP_MD_CTX *ctx = EVP_MD_CTX_new();
	if (!ctx)
		return std::nullopt;

	if (EVP_DigestInit_ex(ctx, EVP_sha256(), nullptr) != 1) {
		EVP_MD_CTX_free(ctx);
		return std::nullopt;
	}

	char buf[65536];
	while (f) {
		f.read(buf, sizeof(buf));
		std::streamsize n = f.gcount();
		if (n > 0)
			EVP_DigestUpdate(ctx, buf, (size_t)n);
	}

	unsigned char digest[EVP_MAX_MD_SIZE];
	unsigned int len = 0;
	if (EVP_DigestFinal_ex(ctx, digest, &len) != 1 || len != 32) {
		EVP_MD_CTX_free(ctx);
		return std::nullopt;
	}
	EVP_MD_CTX_free(ctx);

	static const char hex[] = "0123456789abcdef";
	std::string out;
	out.reserve(64);
	for (unsigned int i = 0; i < len; i++) {
		out.push_back(hex[digest[i] >> 4]);
		out.push_back(hex[digest[i] & 0x0f]);
	}
	return out;
}

} /* namespace */

TgLoggerd::TgLoggerd(uint32_t api_id, const char *api_hash, const char *data_dir) noexcept
{
	this->api_id_ = api_id;
	snprintf(this->api_hash_, sizeof(this->api_hash_), "%s", api_hash);
	snprintf(this->data_dir_, sizeof(this->data_dir_), "%s", data_dir);
}

TgLoggerd::~TgLoggerd(void)
{
	pr_debug(l_, "Destroying TgLoggerd...");
	pr_free(l_);
}

inline int TgLoggerd::initDataDirC(const char *dir)
{
	char path[sizeof(this->data_dir_) + 256];
	int r;

	snprintf(path, sizeof(path), "%s/%s", this->data_dir_, dir);
	pr_debug(l_, "Initializing data directory: %s", path);
	r = mkdir_recursive(path, 0700);
	if (r < 0) {
		if (r == -EEXIST) {
			pr_debug(l_, "Data directory already exists: %s", path);
			return 0;
		}

		pr_error(l_, "Failed to create data directory: %s, error: %s",
			 path, strerror(-r));
		return r;
	} else {
		pr_debug(l_, "Data directory created: %s", path);
		return 0;
	}
}

inline int TgLoggerd::initDataDir(void)
{
	int r;

	r = initDataDirC("logs");
	if (r < 0)
		return r;

	r = initDataDirC("tdlib");
	if (r < 0)
		return r;

	return 0;
}

int TgLoggerd::start(void)
{
	if (initDataDir())
		return -1;

	pr_info(l_, "Starting tgloggerd...");
	pr_debug(l_, "api_id: %u", this->api_id_);
	pr_debug(l_, "api_hash: %s", this->api_hash_);
	pr_debug(l_, "data_dir: %s", this->data_dir_);

	auto env = [](const char *key, const char *def) -> std::string {
		const char *v = getenv(key);
		return (v && *v) ? std::string(v) : std::string(def);
	};

	mysql::Config db_cfg;
	db_cfg.host = env("TG_DB_HOST", "127.0.0.1");
	db_cfg.port = (uint16_t)atoi(env("TG_DB_PORT", "3306").c_str());
	db_cfg.user = env("TG_DB_USER", "tgloggerd");
	db_cfg.password = env("TG_DB_PASSWORD", "tgloggerd");
	db_cfg.database = env("TG_DB_NAME", "tgloggerd");

	pr_debug(l_, "db: %s@%s:%u/%s", db_cfg.user.c_str(), db_cfg.host.c_str(),
		 db_cfg.port, db_cfg.database.c_str());

	db_ = std::make_unique<DB>(db_cfg);
	try {
		db_->ping();
	} catch (const std::exception &e) {
		pr_error(l_, "Failed to connect to the database: %s", e.what());
		return -1;
	}

	storage_dir_ = env("TG_STORAGE_DIR", "./data/storage/files");
	try {
		fs::create_directories(storage_dir_);
	} catch (const std::exception &e) {
		pr_error(l_, "Failed to create storage directory %s: %s",
			 storage_dir_.c_str(), e.what());
		return -1;
	}
	pr_debug(l_, "storage_dir: %s", storage_dir_.c_str());

	char tdlib_path[sizeof(this->data_dir_) + 32];
	snprintf(tdlib_path, sizeof(tdlib_path), "%s/tdlib", this->data_dir_);

	tdlib_ = std::make_unique<TDLib>(this->api_id_, this->api_hash_,
					 tdlib_path);
	tdlib_->setUserHandler([this](const models::User &u) {
		try {
			db_->upsertUser(u);
		} catch (const std::exception &e) {
			pr_error(l_, "Failed to store user %lld: %s",
				 (long long)u.id, e.what());
		}
	});
	tdlib_->setProfilePhotoHandler([this](const ProfilePhoto &p) {
		try {
			onProfilePhoto(p);
		} catch (const std::exception &e) {
			pr_error(l_, "Failed to store profile photo for user"
				 " %lld: %s", (long long)p.user_id, e.what());
		}
	});
	tdlib_->setMessageHandler([this](const TextMessage &msg) {
		pr_info(l_, "New message | sender_id=%lld name=\"%s\" "
			    "username=\"%s\" msg_id=%lld text=\"%s\"",
			(long long)msg.sender_id, msg.sender_name.c_str(),
			msg.sender_username.c_str(), (long long)msg.message_id,
			msg.text.c_str());
	});

	pr_info(l_, "Listening for incoming messages...");
	while (!tdlib_->isStopped())
		tdlib_->loop(10);

	return 0;
}

int TgLoggerd::stop(void)
{
	pr_info(l_, "Stopping tgloggerd...");
	if (tdlib_)
		tdlib_->close();
	return 0;
}

void TgLoggerd::onProfilePhoto(const ProfilePhoto &p)
{
	auto hex = sha256_file_hex(p.local_path);
	if (!hex.has_value()) {
		pr_error(l_, "Failed to hash profile photo: %s",
			 p.local_path.c_str());
		return;
	}

	/* Content-addressed destination name: <sha256>[.ext]. */
	std::string ext = fs::path(p.local_path).extension().string();
	if (!ext.empty() && ext[0] == '.')
		ext.erase(0, 1);
	std::transform(ext.begin(), ext.end(), ext.begin(),
		       [](unsigned char c) { return (char)std::tolower(c); });

	std::string name = *hex;
	if (!ext.empty())
		name += "." + ext;
	fs::path dest = fs::path(storage_dir_) / name;

	std::error_code ec;
	if (!fs::exists(dest, ec))
		fs::copy_file(p.local_path, dest, ec);
	if (ec) {
		pr_error(l_, "Failed to copy profile photo to %s: %s",
			 dest.c_str(), ec.message().c_str());
		return;
	}

	models::File f;
	f.tg_file_id = p.tg_file_id;
	f.file_type = "photo";
	f.file_size = (uint64_t)(p.file_size < 0 ? 0 : p.file_size);
	f.sha256_hex = *hex;
	if (!ext.empty())
		f.file_ext = ext;

	uint64_t file_id = db_->upsertFile(f);
	db_->setUserProfilePhoto(p.user_id, file_id);

	pr_info(l_, "Stored profile photo | user_id=%lld file_id=%llu sha256=%s",
		(long long)p.user_id, (unsigned long long)file_id,
		hex->c_str());
}

void TgLoggerd::setLogger(log_hd_t *h) noexcept
{
	l_ = h;
}

void TgLoggerd::setLoggerLevel(int8_t log_level) noexcept
{
	pr_set_log_level(l_, log_level);
}

} /* namespace tgloggerd */
