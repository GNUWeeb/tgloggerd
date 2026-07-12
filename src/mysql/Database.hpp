// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef MYSQL__DATABASE_HPP
#define MYSQL__DATABASE_HPP

#include <mysql/ConnectionPool.hpp>

#include <string>
#include <vector>
#include <cstdint>
#include <variant>
#include <optional>

namespace mysql {

/*
 * A bound query parameter. std::monostate represents SQL NULL. Strings
 * carry both text and binary values (e.g. a BINARY(32) digest).
 */
using Param = std::variant<std::monostate, int64_t, uint64_t, double, std::string>;

/*
 * A single result row: one nullable string per column. Callers convert
 * the textual values to the types they expect.
 */
using Row = std::vector<std::optional<std::string>>;

/*
 * Executes SQL statements against a MySQL server using a ConnectionPool.
 *
 * All statements are prepared and their parameters bound, so callers must
 * never interpolate untrusted values into the SQL text. Errors are
 * reported as std::runtime_error, hiding the underlying JDBC exception
 * type from callers.
 */
class Database {
public:
	explicit Database(const Config &cfg);
	~Database(void);

	Database(const Database &) = delete;
	Database &operator=(const Database &) = delete;

	/* Run an INSERT/UPDATE/DELETE/DDL; returns the affected row count. */
	uint64_t execute(const std::string &sql,
			 const std::vector<Param> &params = {});

	/* Run an INSERT; returns the generated AUTO_INCREMENT id. */
	uint64_t insert(const std::string &sql,
			const std::vector<Param> &params = {});

	/* Run a SELECT; returns all rows. */
	std::vector<Row> query(const std::string &sql,
			       const std::vector<Param> &params = {});

private:
	ConnectionPool pool_;
};

} /* namespace mysql */

#endif /* #ifndef MYSQL__DATABASE_HPP */
