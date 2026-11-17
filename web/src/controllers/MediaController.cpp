// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "controllers/MediaController.hpp"

#include "Config.hpp"
#include "auth/FileToken.hpp"
#include "auth/Session.hpp"
#include "controllers/Common.hpp"
#include "dao/Browse.hpp"
#include "dao/Audit.hpp"
#include "daemon/AccessReport.hpp"
#include "daemon/Api.hpp"
#include "media/Fetch.hpp"

#include <cctype>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <filesystem>
#include <optional>
#include <string>

namespace tgweb::controllers {

namespace {

/* Storage root; must equal the daemon's TG_STORAGE_DIR. Read once. */
const std::string &storageDir(void)
{
	static const std::string dir =
		tgweb::env("WEB_STORAGE_DIR", "data/storage/files");
	return dir;
}

bool isHex64(const std::string &s)
{
	if (s.size() != 64)
		return false;
	for (char c : s)
		if (!std::isxdigit((unsigned char)c))
			return false;
	return true;
}

/* Keep only [a-z0-9] from a file extension, lower-cased, at most 10 chars. */
std::string safeExt(const std::string &e)
{
	std::string o;
	for (char c : e) {
		if (std::isalnum((unsigned char)c))
			o += (char)std::tolower((unsigned char)c);
		if (o.size() >= 10)
			break;
	}
	return o;
}

/* Strip characters unsafe in a Content-Disposition filename. */
std::string safeFilename(const std::string &n)
{
	std::string o;
	for (unsigned char c : n) {
		if (c < 0x20 || c == '"' || c == '\\' || c == '/')
			continue;
		o += (char)c;
	}
	return o;
}

/* Content is viewed inline; documents are offered as a download. */
bool viewInline(const std::string &fileType)
{
	return fileType == "photo" || fileType == "video" ||
	       fileType == "audio" || fileType == "voice" ||
	       fileType == "animation" || fileType == "sticker";
}

/* A single parsed byte range. present: a "bytes=..." header was given;
 * satisfiable: it maps to a real [offset, offset+length) slice. */
struct ByteRange {
	bool	present = false;
	bool	satisfiable = false;
	size_t	offset = 0;
	size_t	length = 0;
};

/*
 * Parse the first range of an HTTP Range header ("bytes=start-end",
 * "bytes=start-", "bytes=-suffix"). Only a single range is honoured; a
 * multi-range request falls back to its first range. Returns present=false
 * for anything that is not a byte range, and satisfiable=false for a byte
 * range that cannot be met (so the caller answers 416).
 */
ByteRange parseRange(const std::string &header, uint64_t filesize)
{
	ByteRange r;
	if (header.rfind("bytes=", 0) != 0 || filesize == 0)
		return r;

	std::string spec = header.substr(6);
	auto comma = spec.find(',');
	if (comma != std::string::npos)
		spec = spec.substr(0, comma);
	auto dash = spec.find('-');
	if (dash == std::string::npos)
		return r;

	r.present = true;
	std::string s = spec.substr(0, dash);
	std::string e = spec.substr(dash + 1);
	auto num = [](const std::string &x) -> uint64_t {
		return strtoull(x.c_str(), nullptr, 10);
	};

	if (s.empty()) {
		/* "-N": the last N bytes. */
		if (e.empty())
			return r;
		uint64_t n = num(e);
		if (n == 0)
			return r;
		if (n > filesize)
			n = filesize;
		r.offset = (size_t)(filesize - n);
		r.length = (size_t)n;
		r.satisfiable = true;
	} else {
		uint64_t start = num(s);
		if (start >= filesize)
			return r;	/* start past EOF -> 416 */
		uint64_t end = e.empty() ? filesize - 1 : num(e);
		if (end >= filesize)
			end = filesize - 1;
		if (end < start)
			return r;
		r.offset = (size_t)start;
		r.length = (size_t)(end - start + 1);
		r.satisfiable = true;
	}
	return r;
}

/*
 * The range to ask the daemon for, from the Range header, when the file is
 * not in the store and its size may be unknown (0). length 0 means "from
 * offset on", which the daemon answers with one chunk. Anything that is not a
 * usable single byte range asks from the start.
 */
struct FetchRange {
	uint64_t offset = 0;
	uint64_t length = 0;
};

FetchRange fetchRange(const std::string &header, uint64_t knownSize)
{
	FetchRange r;
	if (header.rfind("bytes=", 0) != 0)
		return r;
	std::string spec = header.substr(6);
	auto comma = spec.find(',');
	if (comma != std::string::npos)
		spec = spec.substr(0, comma);
	auto dash = spec.find('-');
	if (dash == std::string::npos)
		return r;
	std::string s = spec.substr(0, dash);
	std::string e = spec.substr(dash + 1);
	auto num = [](const std::string &x) -> uint64_t {
		return strtoull(x.c_str(), nullptr, 10);
	};

	if (s.empty()) {
		/* "-N" needs the size to say where the last N bytes start. */
		uint64_t n = e.empty() ? 0 : num(e);
		if (n == 0 || knownSize == 0)
			return r;
		if (n > knownSize)
			n = knownSize;
		r.offset = knownSize - n;
		r.length = n;
		return r;
	}
	uint64_t start = num(s);
	if (e.empty()) {
		r.offset = start;
		return r;
	}
	uint64_t end = num(e);
	if (end < start)
		return r;
	r.offset = start;
	r.length = end - start + 1;
	return r;
}

/* The store path of a row's bytes: <dir>/aa/bb/cc/dd/ee/<hex>[.ext], from
 * the database only. Empty when the row has no valid sha256. */
std::filesystem::path storePath(const dao::browse::FileMeta &blob)
{
	if (!isHex64(blob.hex))
		return {};
	std::string ext = safeExt(blob.ext);
	std::filesystem::path path = storageDir();
	for (int i = 0; i < 5; i++)
		path /= blob.hex.substr((size_t)i * 2, 2);
	std::string name = blob.hex;
	if (!ext.empty())
		name += "." + ext;
	path /= name;
	return path;
}

bool presentInStore(const std::filesystem::path &path)
{
	std::error_code ec;
	return !path.empty() && std::filesystem::is_regular_file(path, ec);
}

/*
 * Every file response advertises range support. A token's bytes never
 * change, so the file stays cacheable, but only for a week and not
 * "immutable": a browser that keeps showing it asks again, and that request
 * is what keeps the file from being evicted as unused.
 */
void fileHeaders(const drogon::HttpResponsePtr &resp)
{
	resp->addHeader("Accept-Ranges", "bytes");
	resp->addHeader("Cache-Control", "public, max-age=604800");
}

/*
 * Serve a file from the store, honouring a single Range. This Drogon's
 * newFileResponse(..., req) does not itself read the Range header, so parse
 * it and use the offset/length overload for a partial (206). The on-disk
 * extension drives the Content-Type.
 */
drogon::HttpResponsePtr serveStored(const drogon::HttpRequestPtr &req,
				    const std::filesystem::path &path,
				    const std::string &attachment)
{
	std::error_code sizeEc;
	uint64_t filesize = std::filesystem::file_size(path, sizeEc);
	ByteRange rng = parseRange(req->getHeader("range"), filesize);

	drogon::HttpResponsePtr resp;
	if (rng.present && !rng.satisfiable) {
		resp = drogon::HttpResponse::newHttpResponse();
		resp->setStatusCode(drogon::k416RequestedRangeNotSatisfiable);
		resp->addHeader("Content-Range",
				"bytes */" + std::to_string(filesize));
	} else if (rng.satisfiable && rng.length < filesize) {
		resp = drogon::HttpResponse::newFileResponse(
			path.string(), rng.offset, rng.length,
			/*setContentRange=*/true, attachment, drogon::CT_NONE,
			"", req);
	} else {
		resp = drogon::HttpResponse::newFileResponse(
			path.string(), attachment, drogon::CT_NONE, "", req);
	}
	fileHeaders(resp);
	return resp;
}

/* 503 with Retry-After: the daemon is busy, away, or not signed in to
 * Telegram; the browser (or the user) may simply ask again. */
drogon::HttpResponsePtr unavailable(const drogon::HttpRequestPtr &req,
				    const std::string &retryAfter)
{
	auto resp = renderStatus(req, drogon::k503ServiceUnavailable,
				 "Telegram is not reachable right now",
				 "This file is not in storage and could not be "
				 "fetched from Telegram at the moment. Try "
				 "again shortly.");
	std::string ra = retryAfter;
	if (ra.empty() || ra.find_first_not_of("0123456789") != std::string::npos)
		ra = "5";
	resp->addHeader("Retry-After", ra);
	return resp;
}

/* A reply the daemon should never give: refuse the request, say why in the
 * log, and do not pass the daemon's detail to the browser. */
drogon::HttpResponsePtr badGateway(const drogon::HttpRequestPtr &req,
				   uint64_t fid, const std::string &why)
{
	fprintf(stderr, "media: fetch of file %llu: %s\n",
		(unsigned long long)fid, why.c_str());
	return renderStatus(req, drogon::k502BadGateway, "Fetch failed",
			    "The file could not be fetched from Telegram.");
}

bool u64Field(const nlohmann::json &b, const char *k, uint64_t &out)
{
	if (!b.contains(k) || !b[k].is_number_unsigned())
		return false;
	out = b[k].get<uint64_t>();
	return true;
}

} /* namespace */

drogon::Task<drogon::HttpResponsePtr>
MediaController::serve(drogon::HttpRequestPtr req, std::string token)
{
	/* The token is the encrypted file id; an invalid or forged one (which a
	 * public visitor probing the URL space would produce) fails to decrypt
	 * and is indistinguishable from a missing file. */
	auto decoded = auth::filetoken::decrypt(token);
	if (!decoded)
		co_return renderStatus(req, drogon::k404NotFound, "File not found",
				       "No file exists with that id.");
	int64_t fid = (int64_t)*decoded;

	auto ro = drogon::app().getDbClient("ro");
	auto meta = co_await dao::browse::getFile(ro, fid);
	if (!meta)
		co_return renderStatus(req, drogon::k404NotFound, "File not found",
				       "No file exists with that id.");

	/*
	 * A row whose bytes turned out to be another row's points at that row
	 * (alias_of) instead of holding the sha256 itself; follow it one hop
	 * for the bytes, as the daemon does. The name and type shown stay this
	 * row's, the one the token was minted for.
	 */
	std::optional<dao::browse::FileMeta> target;
	if (meta->aliasOf)
		target = co_await dao::browse::getFile(ro,
						       (int64_t)meta->aliasOf);
	const dao::browse::FileMeta &blob = target ? *target : *meta;

	auto sess = auth::session::current(req);

	/* Documents are offered as a download under their original name. */
	std::string attachment;
	if (!viewInline(meta->fileType)) {
		attachment = safeFilename(meta->origName);
		if (attachment.empty()) {
			/* The blob's name when there is one, as always; a
			 * file never stored has no sha256 to name it by. */
			auto path = storePath(blob);
			if (!path.empty()) {
				attachment = path.filename().string();
			} else {
				std::string ext = safeExt(meta->ext);
				attachment = "file_" + std::to_string(fid);
				if (!ext.empty())
					attachment += "." + ext;
			}
		}
	}

	/* Audit only authenticated access. The route is public, so anonymous
	 * hits (hotlinked images, crawlers) must not flood the audit log. */
	auto audit = [&]() -> drogon::Task<void> {
		if (!sess)
			co_return;
		auto app = drogon::app().getDbClient("app");
		co_await dao::audit::log(app, sess->uid, "media",
					 req->getPeerAddr().toIp(),
					 "file " + std::to_string(fid));
	};

	/* Stored, and its blob is where the row says: serve it as always.
	 * Count the use for eviction against the row whose bytes are served,
	 * which is the one the store keeps or evicts. Never waits on the
	 * daemon. */
	if (blob.onDisk) {
		auto path = storePath(blob);
		if (presentInStore(path)) {
			daemon::access::record(target ? meta->aliasOf
						      : (uint64_t)fid);
			co_await audit();
			co_return serveStored(req, path, attachment);
		}
		/* A blob gone from disk is treated as not stored: the daemon
		 * notices and fetches it again. */
	}

	/*
	 * Not in the store (evicted, never fetched, over the per-file cap, or
	 * its blob missing): ask the daemon to fetch it from Telegram. Who may
	 * do that is decided in one place, media::mayFetch().
	 */
	switch (media::mayFetch(meta->size, (bool)sess)) {
	case media::FetchDecision::Allowed:
		break;
	case media::FetchDecision::Off:
		co_return renderStatus(req, drogon::k404NotFound,
				       "File not stored",
				       "This file's bytes are not in storage, "
				       "and fetching files from Telegram is "
				       "not configured.");
	case media::FetchDecision::NeedsSession:
		co_return renderStatus(req, drogon::k403Forbidden,
				       "Sign in to load this file",
				       "This file is not in storage. Large "
				       "files are loaded from Telegram for "
				       "signed-in users only.");
	}

	FetchRange want = fetchRange(req->getHeader("range"), meta->size);
	daemon::api::Reply rep =
		co_await daemon::api::fetch((uint64_t)fid, want.offset,
					    want.length);
	if (!rep.sent)
		co_return unavailable(req, "10");
	if (!rep.ok()) {
		switch (rep.status) {
		case 503:
			co_return unavailable(req, rep.retryAfter);
		case 410:
			co_return renderStatus(req, drogon::k410Gone,
					       "File no longer available",
					       "This file is no longer available "
					       "on Telegram.");
		case 504:
			co_return renderStatus(req, drogon::k504GatewayTimeout,
					       "Telegram is slow",
					       "Fetching this file from Telegram "
					       "took too long. It goes on in the "
					       "background; try again shortly.");
		case 404:
			co_return renderStatus(req, drogon::k404NotFound,
					       "File not found",
					       "No file exists with that id.");
		case 416: {
			auto resp = drogon::HttpResponse::newHttpResponse();
			resp->setStatusCode(
				drogon::k416RequestedRangeNotSatisfiable);
			if (meta->size)
				resp->addHeader("Content-Range",
						"bytes */" +
						std::to_string(meta->size));
			co_return resp;
		}
		default:
			co_return badGateway(req, (uint64_t)fid, rep.error);
		}
	}

	const nlohmann::json &b = rep.body;
	std::string state = b.is_object() && b.contains("state") &&
			    b["state"].is_string()
		? b["state"].get<std::string>() : "";

	if (state == "stored") {
		/* Now in the store, possibly as another row's bytes (same
		 * sha256): serve that row's blob exactly as a stored file. */
		uint64_t sid = 0;
		if (!u64Field(b, "id", sid) || sid == 0)
			co_return badGateway(req, (uint64_t)fid,
					     "stored reply without an id");
		auto held = co_await dao::browse::getFile(ro, (int64_t)sid);
		auto path = held && held->onDisk ? storePath(*held)
						 : std::filesystem::path();
		if (!presentInStore(path))
			co_return unavailable(req, "1");
		daemon::access::record(sid);
		co_await audit();
		co_return serveStored(req, path, attachment);
	}

	if (state != "partial")
		co_return badGateway(req, (uint64_t)fid,
				     "unknown fetch state '" + state + "'");

	/*
	 * A big file: `available` bytes from `offset` are in TDLib's cache at
	 * `path`. Streaming one is for signed-in users only; a file of unknown
	 * size can turn out big only now.
	 */
	if (!sess)
		co_return renderStatus(req, drogon::k403Forbidden,
				       "Sign in to load this file",
				       "This file is not in storage. Large "
				       "files are loaded from Telegram for "
				       "signed-in users only.");

	uint64_t off = 0, avail = 0, total = 0;
	if (!u64Field(b, "offset", off) || !u64Field(b, "available", avail) ||
	    !u64Field(b, "size", total) || avail == 0 || off >= total ||
	    avail > total - off || !b.contains("path") ||
	    !b["path"].is_string())
		co_return badGateway(req, (uint64_t)fid,
				     "malformed partial reply");

	/* The daemon checks the path too; never serve one from elsewhere. */
	auto tdpath = media::insideTdlibDir(b["path"].get<std::string>());
	if (!tdpath)
		co_return badGateway(req, (uint64_t)fid,
				     "partial path outside the TDLib "
				     "directory: " +
				     b["path"].get<std::string>());

	/* Serve what was asked for and is there, never more: past
	 * `available` the file may be holes. */
	uint64_t len = avail;
	if (off == want.offset && want.length && want.length < len)
		len = want.length;

	/*
	 * Content-Range carries the file's full size from the daemon: the
	 * partial file on disk may be shorter, and Drogon would put its size
	 * there. TDLib names its files with their extension (videos/file_12.mp4,
	 * a document's own name), which picks the Content-Type, as the blob
	 * name does for a stored file.
	 */
	auto resp = drogon::HttpResponse::newFileResponse(
		tdpath->string(), (size_t)off, (size_t)len,
		/*setContentRange=*/false, attachment, drogon::CT_NONE, "", req);
	auto code = resp->getStatusCode();
	if (code != drogon::k200OK && code != drogon::k206PartialContent)
		/* Shorter on disk than the daemon said, or deleted since. */
		co_return unavailable(req, "1");
	if (off == 0 && len == total) {
		resp->setStatusCode(drogon::k200OK);
	} else {
		resp->setStatusCode(drogon::k206PartialContent);
		resp->addHeader("Content-Range",
				"bytes " + std::to_string(off) + "-" +
				std::to_string(off + len - 1) + "/" +
				std::to_string(total));
	}
	fileHeaders(resp);

	daemon::access::record(target ? meta->aliasOf : (uint64_t)fid);
	co_await audit();
	co_return resp;
}

} /* namespace tgweb::controllers */
