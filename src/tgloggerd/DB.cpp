// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#include <tgloggerd/DB.hpp>

namespace tgloggerd {

DB::DB(const mysql::Config &cfg)
	: db_(cfg)
{
}

DB::~DB(void) = default;

void DB::ping(void)
{
	db_.query("SELECT 1");
}

} /* namespace tgloggerd */
