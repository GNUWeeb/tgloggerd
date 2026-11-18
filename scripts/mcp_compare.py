#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-2.0-or-later
#
# Side-by-side check of two MCP endpoints, before /mcp moves from the
# Drogon web to the Go service (webapi/README, MCP): the same requests go
# to both, one right after the other, and the answers must be the same
# bytes, status and Content-Type included.
#
# The requests cover the transport, the token refusals and every tool,
# with ids found through the endpoints themselves (the exposed groups,
# their busiest senders, recent and edited messages), so it runs against
# any archive. Read-only, apart from the last_used_at stamp of the token.
# Message answers of a live archive can change between the two requests;
# a difference is asked again once before it counts.
#
# Usage:
#   MCP_TOKEN=tgmcp_... scripts/mcp_compare.py \
#       --a http://drogon-host:8080/mcp --b http://webapi-host:8081/mcp
#   [--groups N]   how many exposed groups to look into (default 3)
#   [--verbose]    print every request, not only differences
#
# The token is read from MCP_TOKEN, so it stays out of the shell history
# and the process list. Exits 1 when any answer differs.

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request

PARSE_ERROR_PREFIX = '"message":"Parse error: '


def send(url, method, body, headers):
    req = urllib.request.Request(url, data=body, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=60) as r:
            return r.status, r.headers.get("Content-Type", ""), r.headers.get("Allow", ""), r.read()
    except urllib.error.HTTPError as e:
        return e.code, e.headers.get("Content-Type", ""), e.headers.get("Allow", ""), e.read()


class Compare:
    def __init__(self, a, b, token, verbose):
        self.a, self.b, self.token, self.verbose = a, b, token, verbose
        self.ok = self.bad = 0
        self.rid = 0

    def request(self, name, body, method="POST", auth=True, query="", headers=None):
        """Send one request to both; returns A's decoded body (or None)."""
        h = {"Content-Type": "application/json"}
        if auth:
            h["Authorization"] = "Bearer " + self.token
        h.update(headers or {})
        if isinstance(body, (dict, list)):
            body = json.dumps(body).encode()
        elif isinstance(body, str):
            body = body.encode()
        sep = "&" if "?" in self.a else "?"
        qa = (self.a + sep + query) if query else self.a
        sep = "&" if "?" in self.b else "?"
        qb = (self.b + sep + query) if query else self.b
        for attempt in range(2):
            ra = send(qa, method, body, h)
            rb = send(qb, method, body, h)
            same = head(ra) == head(rb) and (ra[3] == rb[3] or both_parse_errors(ra[3], rb[3]))
            if same or attempt == 1:
                break
            time.sleep(1)
        if same:
            self.ok += 1
            if self.verbose:
                print("  same  %s (%d, %d bytes)" % (name, ra[0], len(ra[3])))
        else:
            self.bad += 1
            print("  DIFF  %s" % name)
            if head(ra) != head(rb):
                print("        A: %s %s %s" % head(ra))
                print("        B: %s %s %s" % head(rb))
            show_difference(ra[3], rb[3])
        try:
            return json.loads(ra[3])
        except ValueError:
            return None

    def rpc(self, name, method, params=None, **kw):
        self.rid += 1
        m = {"jsonrpc": "2.0", "id": self.rid, "method": method}
        if params is not None:
            m["params"] = params
        return self.request(name, m, **kw)

    def tool(self, name, tool, args):
        d = self.rpc(name, "tools/call", {"name": tool, "arguments": args})
        try:
            return d["result"].get("structuredContent") or {}
        except (TypeError, KeyError, AttributeError):
            return {}


def head(r):
    """Status, Content-Type (only with a body: the Drogon web types an
    empty 202 as text/html, the Go service not at all) and Allow."""
    return r[0], r[1] if r[3] else "", r[2]


def both_parse_errors(x, y):
    # The one known difference: a JSON parse error quotes each parser's
    # own message.
    return PARSE_ERROR_PREFIX.encode() in x and PARSE_ERROR_PREFIX.encode() in y


def show_difference(x, y):
    i = 0
    while i < min(len(x), len(y)) and x[i] == y[i]:
        i += 1
    lo = max(0, i - 80)
    print("        first difference at byte %d of %d / %d" % (i, len(x), len(y)))
    print("        A: ...%s" % x[lo:i + 80].decode("utf-8", "replace"))
    print("        B: ...%s" % y[lo:i + 80].decode("utf-8", "replace"))


def day(ts):
    return time.strftime("%Y-%m-%d", time.gmtime(ts))


def main():
    p = argparse.ArgumentParser(description="Compare the answers of two MCP endpoints.")
    p.add_argument("--a", required=True, help="the first /mcp URL (the Drogon web)")
    p.add_argument("--b", required=True, help="the second /mcp URL (the Go service)")
    p.add_argument("--groups", type=int, default=3)
    p.add_argument("--verbose", action="store_true")
    o = p.parse_args()
    token = os.environ.get("MCP_TOKEN", "")
    if not token:
        sys.exit("set MCP_TOKEN to a live MCP token")
    c = Compare(o.a, o.b, token, o.verbose)
    bogus = "tgmcp_" + "0" * 64

    print("transport and tokens")
    c.rpc("no token", "ping", auth=False)
    c.rpc("unknown token", "ping", auth=False, headers={"Authorization": "Bearer " + bogus})
    c.rpc("malformed token", "ping", auth=False, headers={"Authorization": "Bearer tgmcp_1"})
    c.rpc("token in the query", "ping", auth=False, query="key=" + token)
    c.request("GET", None, method="GET", auth=False)
    c.request("DELETE", None, method="DELETE", auth=False)
    c.request("empty body", b"")
    c.request("not JSON", b"{x")
    c.request("batch", [{"jsonrpc": "2.0", "id": 1, "method": "ping"}])
    c.request("notification", {"jsonrpc": "2.0", "method": "notifications/initialized"})
    c.rpc("malformed version", "ping", headers={"MCP-Protocol-Version": "2025-6-18"})
    c.rpc("unknown method", "resources/list")
    c.rpc("initialize", "initialize", {"protocolVersion": "2025-06-18", "capabilities": {},
                                       "clientInfo": {"name": "mcp_compare", "version": "1"}})
    c.rpc("initialize, unknown version", "initialize", {"protocolVersion": "2099-01-01"})
    c.rpc("tools/list", "tools/list")
    c.rpc("unknown tool", "tools/call", {"name": "telegram_nope"})

    print("tools")
    groups = c.tool("list_groups", "telegram_list_groups", {"limit": 200}).get("groups", [])
    c.tool("list_groups limit 1", "telegram_list_groups", {"limit": 1})
    ids = [g["group_id"] for g in groups][:o.groups]
    if not ids:
        print("  no exposed group: only the refusals and empty answers are compared")
    c.tool("get_group batch", "telegram_get_group", {"group_ids": ids + [-1, 1]} if ids else {"group_ids": [1]})
    c.tool("recent", "telegram_list_recent_messages", {"limit": 20})
    c.tool("search, no filter", "telegram_search_messages", {"limit": 20, "include_total": True})
    c.tool("search, bad filter", "telegram_search_messages", {"filter": {"field": "nope"}})
    now = int(time.time())
    week = [day(now - 8 * 86400), day(now - 86400)]  # closed days: stable
    users = set()
    for g in ids:
        tag = " (%d)" % g
        c.tool("get_group" + tag, "telegram_get_group", {"group_id": g})
        c.tool("group_history" + tag, "telegram_get_group_history", {"group_id": g, "limit": 20})
        c.tool("admins" + tag, "telegram_list_group_admins", {"group_id": g})
        s = c.tool("senders" + tag, "telegram_list_group_senders", {"group_id": g, "limit": 10, "include_total": True})
        c.tool("senders, a week" + tag, "telegram_list_group_senders",
               {"group_id": g, "start_date": week[0], "end_date": week[1], "limit": 10})
        senders = [x["user_id"] for x in s.get("senders", [])][:3]
        users.update(senders)
        for u in senders[:1]:
            c.tool("count_user" + tag, "telegram_count_user_messages", {"group_id": g, "user_id": u})
            c.tool("count_user, a week" + tag, "telegram_count_user_messages",
                   {"group_id": g, "user_id": u, "start_date": week[0], "end_date": week[1]})
        w = c.tool("popular_words, a week" + tag, "telegram_popular_words",
                   {"group_id": g, "start_date": week[0], "end_date": week[1], "limit": 20})
        c.tool("popular_words raw" + tag, "telegram_popular_words",
               {"group_id": g, "start_date": week[0], "end_date": week[1], "limit": 20,
                "include_stopwords": True, "max_messages": 500})
        r = c.tool("recent" + tag, "telegram_list_recent_messages", {"group_id": g, "limit": 10, "compact": True})
        mids = [m["message_id"] for m in r.get("messages", [])]
        c.tool("search, a week" + tag, "telegram_search_messages",
               {"group_id": g, "filter": {"field": "date", "op": "between", "value": week},
                "limit": 20, "include_total": True})
        page = c.tool("search, a week, asc" + tag, "telegram_search_messages",
                      {"group_id": g, "filter": {"field": "date", "op": "between", "value": week},
                       "order": "asc", "limit": 5})
        if page.get("next_cursor"):
            c.tool("search, next page" + tag, "telegram_search_messages",
                   {"group_id": g, "filter": {"field": "date", "op": "between", "value": week},
                    "order": "asc", "limit": 5, "cursor": page["next_cursor"]})
        words = [x["word"] for x in w.get("words", []) if len(x["word"]) >= 4][:1]
        for word in words:
            c.tool("search, match %r%s" % (word, tag), "telegram_search_messages",
                   {"group_id": g, "filter": {"field": "text", "op": "match", "value": word},
                    "limit": 10, "fields": ["message_id", "sender_username", "text"], "truncate_text": 80})
        ed = c.tool("search, edited" + tag, "telegram_search_messages",
                    {"group_id": g, "filter": {"field": "edited", "op": "is_not_null"},
                     "order_by": "edit_date", "limit": 5})
        c.tool("search, deleted" + tag, "telegram_search_messages",
               {"group_id": g, "order_by": "deleted_at", "limit": 5})
        edited = [m["message_id"] for m in ed.get("messages", [])]
        if mids or edited:
            c.tool("get_messages" + tag, "telegram_get_messages",
                   {"group_id": g, "message_ids": (mids + edited)[:20] + [1], "include_history": True})
        for m in edited[:2]:
            c.tool("message_history %d%s" % (m, tag), "telegram_get_message_history",
                   {"group_id": g, "message_id": m})
            c.tool("message_history diff %d%s" % (m, tag), "telegram_get_message_history",
                   {"group_id": g, "message_id": m, "format": "diff"})
    for u in sorted(users)[:5]:
        c.tool("get_user %d" % u, "telegram_get_user", {"user_id": u})
        c.tool("user_history %d" % u, "telegram_get_user_history", {"user_id": u, "limit": 10})
    if users:
        c.tool("get_users batch", "telegram_get_users",
               {"filter": {"field": "user_id", "op": "in", "value": sorted(users) + [1]}})
    c.tool("get_users, a bot", "telegram_get_users", {"filter": {"field": "is_bot", "op": "=", "value": True}, "limit": 5})
    c.tool("get_users, no filter", "telegram_get_users", {})

    print("\n%d same, %d different" % (c.ok, c.bad))
    sys.exit(1 if c.bad else 0)


if __name__ == "__main__":
    main()
