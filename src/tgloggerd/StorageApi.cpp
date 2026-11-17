// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "StorageApi.hpp"

namespace tgloggerd {

namespace {

nlohmann::json phase_json(const StorageJanitor::Phase &p)
{
	return { { "files", p.files }, { "bytes", p.bytes } };
}

nlohmann::json sweep_json(const StorageJanitor::Sweep &s)
{
	return {
		{ "started_at", s.started_at },
		{ "finished_at", s.finished_at },
		{ "dry_run", s.dry_run },
		{ "completed", s.completed },
		{ "error", s.error.empty() ? nlohmann::json(nullptr)
					   : nlohmann::json(s.error) },
		{ "retention", phase_json(s.retention) },
		{ "cap", phase_json(s.cap) },
		{ "bytes_before", s.bytes_before },
		{ "bytes_after", s.bytes_after },
		{ "kept", s.kept },
		{ "missing_blobs", s.missing_blobs },
		{ "unlink_errors", s.unlink_errors },
	};
}

} /* namespace */

void registerStorageApi(InternalApi &api, DB &db, StorageJanitor &janitor)
{
	DB *d = &db;
	StorageJanitor *j = &janitor;

	api.route("GET", "/v1/storage", [d, j](const InternalApi::Request &) {
		StoreTotals t = d->storedTotals();
		StorageJanitor::Status st = j->status();

		InternalApi::Response r;
		r.body = {
			{ "files", t.files },
			{ "bytes", t.bytes },
			{ "max_bytes", st.cfg.max_bytes },
			{ "low_watermark", st.low_watermark },
			{ "retention_days", st.cfg.retention_days },
			{ "sweep_secs", st.cfg.sweep_secs },
			{ "dry_run", st.cfg.dry_run },
			{ "sweeping", st.sweeping },
			{ "pending", st.pending },
			{ "last_sweep", st.last ? sweep_json(*st.last)
						: nlohmann::json(nullptr) },
		};
		return r;
	});

	api.route("POST", "/v1/storage/sweep", [j](const InternalApi::Request &req) {
		const auto &b = req.body;
		/* No body, or {} or {"dry_run": bool}. */
		if (!b.is_null() && !b.is_object())
			throw InternalApi::Error::badRequest(
				"want {\"dry_run\": bool} or no body");
		bool dry = j->status().cfg.dry_run;
		if (b.is_object() && b.contains("dry_run")) {
			if (!b["dry_run"].is_boolean())
				throw InternalApi::Error::badRequest(
					"dry_run must be a boolean");
			dry = b["dry_run"].get<bool>();
		}

		bool effective = dry;
		if (!j->requestSweep(dry, &effective))
			throw InternalApi::Error(403, "forbidden",
				"TG_STORAGE_DRY_RUN=1: the daemon only reports"
				" what it would evict");

		InternalApi::Response r;
		r.status = 202;
		r.body = { { "queued", true }, { "dry_run", effective } };
		return r;
	});
}

} /* namespace tgloggerd */
