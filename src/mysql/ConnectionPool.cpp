// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#include <mysql/ConnectionPool.hpp>

#include <mysql_driver.h>
#include <cppconn/connection.h>
#include <cppconn/statement.h>

namespace mysql {

ConnectionPool::ConnectionPool(const Config &cfg)
	: cfg_(cfg)
{
	if (cfg_.pool_size == 0)
		cfg_.pool_size = 1;
}

ConnectionPool::~ConnectionPool(void) = default;

std::unique_ptr<sql::Connection> ConnectionPool::create(void)
{
	sql::mysql::MySQL_Driver *driver = sql::mysql::get_mysql_driver_instance();
	std::string url = "tcp://" + cfg_.host + ":" + std::to_string(cfg_.port);

	std::unique_ptr<sql::Connection> conn(
		driver->connect(url, cfg_.user, cfg_.password));
	conn->setSchema(cfg_.database);

	std::unique_ptr<sql::Statement> stmt(conn->createStatement());
	stmt->execute("SET NAMES utf8mb4");
	return conn;
}

std::unique_ptr<sql::Connection> ConnectionPool::acquire(void)
{
	std::unique_lock<std::mutex> lock(mtx_);

	for (;;) {
		if (!idle_.empty()) {
			auto conn = std::move(idle_.front());
			idle_.pop_front();
			return conn;
		}

		if (created_ < cfg_.pool_size) {
			++created_;
			lock.unlock();
			try {
				return create();
			} catch (...) {
				lock.lock();
				--created_;
				cv_.notify_one();
				throw;
			}
		}

		cv_.wait(lock);
	}
}

void ConnectionPool::release(std::unique_ptr<sql::Connection> conn)
{
	if (!conn)
		return;

	{
		std::lock_guard<std::mutex> lock(mtx_);
		idle_.push_back(std::move(conn));
	}
	cv_.notify_one();
}

} /* namespace mysql */
