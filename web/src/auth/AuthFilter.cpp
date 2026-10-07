// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "auth/AuthFilter.hpp"
#include "auth/Session.hpp"

#include <drogon/HttpAppFramework.h>
#include <drogon/HttpResponse.h>
#include <drogon/orm/DbClient.h>
#include <drogon/orm/Exception.h>
#include <drogon/orm/Result.h>
#include <drogon/orm/Row.h>

#include <cstdint>
#include <memory>

namespace tgweb::auth {

namespace {

/* Set by OptionalAuthFilter; read by isAuthenticated(). */
const char kAuthenticatedAttr[] = "tgweb.authenticated";

/* A redirect to the login page, optionally expiring the session cookie. */
drogon::HttpResponsePtr toLogin(bool clearCookie)
{
	auto resp = drogon::HttpResponse::newRedirectionResponse("/login");
	if (clearCookie)
		session::clear(resp);
	return resp;
}

} /* namespace */

void verifySession(const drogon::HttpRequestPtr &req,
		   std::function<void(SessionCheck)> done)
{
	auto s = session::current(req);
	if (!s) {
		done(SessionCheck::None);
		return;
	}

	/*
	 * The cookie's signature is valid; confirm its epoch still matches the
	 * account's, so a password change (which bumps web_users.session_epoch)
	 * invalidates every previously issued cookie -- signing the account out
	 * everywhere. One indexed primary-key lookup per authenticated request.
	 * The continuation is shared so whichever branch runs owns a live copy.
	 */
	auto db = drogon::app().getDbClient("app");
	auto cb = std::make_shared<std::function<void(SessionCheck)>>(
		std::move(done));
	uint32_t epoch = s->epoch;

	db->execSqlAsync(
		"SELECT session_epoch FROM web_users WHERE id = ?",
		[cb, epoch](const drogon::orm::Result &r) {
			(*cb)(!r.empty() &&
					      r[0]["session_epoch"].as<uint32_t>() == epoch
				      ? SessionCheck::Valid
				      : SessionCheck::Revoked);
		},
		[cb](const drogon::orm::DrogonDbException &) {
			(*cb)(SessionCheck::Unverified);
		},
		s->uid);
}

void AuthFilter::doFilter(const drogon::HttpRequestPtr &req,
			  drogon::FilterCallback &&fcb,
			  drogon::FilterChainCallback &&fccb)
{
	auto pass = std::make_shared<drogon::FilterChainCallback>(std::move(fccb));
	auto fail = std::make_shared<drogon::FilterCallback>(std::move(fcb));

	verifySession(req, [pass, fail](SessionCheck c) {
		switch (c) {
		case SessionCheck::Valid:
			(*pass)();
			break;
		case SessionCheck::Revoked:
			(*fail)(toLogin(true));
			break;
		case SessionCheck::None:
		case SessionCheck::Unverified:
			/*
			 * An unverified session keeps its cookie: the failure was
			 * the DB, not the session, so a retry after recovery still
			 * works (every page needs the DB anyway).
			 */
			(*fail)(toLogin(false));
			break;
		}
	});
}

void OptionalAuthFilter::doFilter(const drogon::HttpRequestPtr &req,
				  drogon::FilterCallback &&fcb,
				  drogon::FilterChainCallback &&fccb)
{
	(void)fcb; /* never rejects */
	auto pass = std::make_shared<drogon::FilterChainCallback>(std::move(fccb));

	verifySession(req, [req, pass](SessionCheck c) {
		/*
		 * Only Valid counts. A revoked or unverifiable session is
		 * treated as anonymous here, exactly as AuthFilter would refuse
		 * it, so a public route can never hand a stale cookie more than
		 * an anonymous visitor gets.
		 */
		req->attributes()->insert(kAuthenticatedAttr,
					  c == SessionCheck::Valid);
		(*pass)();
	});
}

bool isAuthenticated(const drogon::HttpRequestPtr &req)
{
	return req->attributes()->get<bool>(kAuthenticatedAttr);
}

} /* namespace tgweb::auth */
