// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package config reads the service's settings from the environment.
//
// The database settings are the Drogon web's (web/src/Config.hpp), with
// the same names, defaults and fallbacks, so one .env serves both web
// services while they run side by side. Only the listening address is
// the service's own (WEBAPI_*), since both listen at the same time.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// DB is one database pool's settings.
type DB struct {
	Name     string // pool name in logs and health output: "ro" or "app"
	Host     string
	Port     uint16
	DBName   string
	User     string
	Password string
	Conns    int // most open connections in the pool
}

// Config is the whole service configuration.
type Config struct {
	ListenAddr string
	ListenPort uint16
	LogLevel   slog.Level

	RO  DB // SELECT-only over the logger's schema
	App DB // read-write over the web's own schema (accounts, audit)
}

// env returns the variable's value, or def when it is unset or empty.
// An empty value counts as unset, as in the daemon and the Drogon web.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func port(key, def string) (uint16, error) {
	s := env(key, def)
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("%s: %q is not a TCP port", key, s)
	}
	return uint16(n), nil
}

func positive(key, def string) (int, error) {
	s := env(key, def)
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s: %q is not a positive number", key, s)
	}
	return n, nil
}

func level(key, def string) (slog.Level, error) {
	s := env(key, def)
	var l slog.Level
	if err := l.UnmarshalText([]byte(strings.ToUpper(s))); err != nil {
		return 0, fmt.Errorf("%s: %q is not one of debug, info, warn, error", key, s)
	}
	return l, nil
}

// FromEnv builds the configuration from the environment. A malformed
// number is an error rather than a silent zero, so a typo in .env stops
// the service at start instead of misconfiguring it.
func FromEnv() (*Config, error) {
	var (
		c   Config
		err error
	)

	c.ListenAddr = env("WEBAPI_LISTEN_ADDR", "127.0.0.1")
	if c.ListenPort, err = port("WEBAPI_LISTEN_PORT", "8081"); err != nil {
		return nil, err
	}
	if c.LogLevel, err = level("WEBAPI_LOG_LEVEL", "info"); err != nil {
		return nil, err
	}

	// WEB_DB_HOST/PORT set the server for both pools and fall back to
	// the daemon's TG_DB_HOST/PORT, as in the Drogon web.
	host := env("WEB_DB_HOST", env("TG_DB_HOST", "127.0.0.1"))
	dbPort, err := port("WEB_DB_PORT", env("TG_DB_PORT", "3306"))
	if err != nil {
		return nil, err
	}

	c.RO = DB{
		Name:     "ro",
		Host:     host,
		Port:     dbPort,
		DBName:   env("WEB_DB_RO_NAME", "tgloggerd"),
		User:     env("WEB_DB_RO_USER", "web_ro"),
		Password: env("WEB_DB_RO_PASSWORD", ""),
	}
	if c.RO.Conns, err = positive("WEB_DB_RO_CONNS", "4"); err != nil {
		return nil, err
	}

	c.App = DB{
		Name:     "app",
		Host:     host,
		Port:     dbPort,
		DBName:   env("WEB_DB_APP_NAME", "tgloggerd_web"),
		User:     env("WEB_DB_APP_USER", "web_app"),
		Password: env("WEB_DB_APP_PASSWORD", ""),
	}
	if c.App.Conns, err = positive("WEB_DB_APP_CONNS", "2"); err != nil {
		return nil, err
	}

	return &c, nil
}
