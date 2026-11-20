// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "StatusApi.hpp"

namespace tgloggerd {

void registerStatusApi(InternalApi &api,
		       std::function<TDLib::Status()> status)
{
	api.route("GET", "/v1/status",
		  [status = std::move(status)](const InternalApi::Request &) {
		TDLib::Status st = status();

		InternalApi::Response r;
		r.body = {
			{ "tdlib", TDLib::stateName(st.state) },
			{ "since", st.since },
			{ "account", nullptr },
		};
		if (st.account_user_id != 0)
			r.body["account"] = { { "user_id", st.account_user_id } };
		return r;
	});
}

} /* namespace tgloggerd */
