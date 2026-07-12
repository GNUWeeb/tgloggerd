// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef MYSQL__CONNECTION_POOL_HPP
#define MYSQL__CONNECTION_POOL_HPP

#include <deque>
#include <mutex>
#include <memory>
#include <string>
#include <cstdint>
#include <condition_variable>

namespace sql {
class Connection;
} /* namespace sql */

namespace mysql {

/*
 * Settings used to open connections to a MySQL server.
 */
struct Config {
	std::string	host = "127.0.0.1";
	uint16_t	port = 3306;
	std::string	user;
	std::string	password;
	std::string	database;
	size_t		pool_size = 4;
};

/*
 * A small thread-safe pool of MySQL (JDBC) connections.
 *
 * Connections are created lazily up to Config::pool_size. Callers borrow
 * a connection with acquire() and hand it back with release(); acquire()
 * blocks when the pool is exhausted until a connection is returned.
 */
class ConnectionPool {
public:
	explicit ConnectionPool(const Config &cfg);
	~ConnectionPool(void);

	ConnectionPool(const ConnectionPool &) = delete;
	ConnectionPool &operator=(const ConnectionPool &) = delete;

	/* Borrow a connection, blocking until one is available. */
	std::unique_ptr<sql::Connection> acquire(void);

	/* Return a previously acquired connection to the pool. */
	void release(std::unique_ptr<sql::Connection> conn);

	const Config &config(void) const { return cfg_; }

private:
	std::unique_ptr<sql::Connection> create(void);

	Config					cfg_;
	std::mutex				mtx_;
	std::condition_variable			cv_;
	std::deque<std::unique_ptr<sql::Connection>> idle_;
	size_t					created_ = 0;
};

} /* namespace mysql */

#endif /* #ifndef MYSQL__CONNECTION_POOL_HPP */
