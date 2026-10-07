// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD_WEB_AUTH_AUTHFILTER_HPP
#define TGLOGGERD_WEB_AUTH_AUTHFILTER_HPP

#include <drogon/HttpFilter.h>

#include <functional>

namespace tgweb::auth {

/* Outcome of checking a request's session; see verifySession(). */
enum class SessionCheck {
	Valid,      /* signature good and the account's epoch still matches */
	None,       /* no session cookie, or one whose signature is bad */
	Revoked,    /* signed, but issued before the account's last epoch bump */
	Unverified, /* signed, but the epoch lookup failed (transient DB error) */
};

/*
 * Check the request's session: the cookie's signature, then its epoch against
 * web_users.session_epoch, so a password change signs the account out
 * everywhere. Calls @done exactly once, possibly asynchronously. This is the
 * one place that decides whether a request is signed in; both filters below
 * are thin policies over it.
 */
void verifySession(const drogon::HttpRequestPtr &req,
		   std::function<void(SessionCheck)> done);

/*
 * Require an authenticated session. Anonymous requests are redirected to
 * /login; authenticated ones fall through to the handler. Attach to every
 * browsing route (everything except /login and static assets).
 */
class AuthFilter : public drogon::HttpFilter<AuthFilter> {
public:
	void doFilter(const drogon::HttpRequestPtr &req,
		      drogon::FilterCallback &&fcb,
		      drogon::FilterChainCallback &&fccb) override;
};

/*
 * Verify the session but never reject: every request falls through, and the
 * handler asks isAuthenticated() and decides for itself. For routes that have
 * a public form -- the chat of an MCP-exposed group -- where only the handler
 * knows, from its own parsed parameters, whether anonymous access applies.
 */
class OptionalAuthFilter : public drogon::HttpFilter<OptionalAuthFilter> {
public:
	void doFilter(const drogon::HttpRequestPtr &req,
		      drogon::FilterCallback &&fcb,
		      drogon::FilterChainCallback &&fccb) override;
};

/*
 * True only when OptionalAuthFilter ran on this request and found a valid
 * session. A route that forgot the filter reads as anonymous, never as
 * signed in -- the safe direction to fail.
 */
bool isAuthenticated(const drogon::HttpRequestPtr &req);

} /* namespace tgweb::auth */

#endif /* TGLOGGERD_WEB_AUTH_AUTHFILTER_HPP */
