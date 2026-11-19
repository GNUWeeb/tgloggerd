// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package testdb gives each test a database of its own on a test MySQL,
// with a schema's migrations applied and, on request, dummy data from
// the committed fixtures. Nothing is shared between tests, so they may
// run with t.Parallel() and `go test ./...` may run packages at once.
//
//	db := testdb.New(t, testdb.Web, "accounts")
//	h := server.New(server.Deps{App: db.DB, ...})
//
// The server is named in WEBAPI_TEST_MYSQL_DSN, as a go-sql-driver DSN
// of an account that may create and drop databases, without a database
// name (root:test@tcp(mysql:3306)/; docker-compose.test.yml starts
// one). Without it, New skips the test.
//
// Each schema's .up.sql files, then its pending changes (pending/), are
// applied in order, once, into a
// template database named after a hash of their contents
// (test_tpl_<schema>_<hash>), built under a MySQL named lock so test
// processes running at once build it once between them. A test's
// database (test_<schema>_<random>) is created from the template's
// table definitions and dropped when the test ends. Never point the
// variable at a server holding data that matters.
package testdb

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	mrand "math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// EnvDSN names the variable holding the test server's DSN.
const EnvDSN = "WEBAPI_TEST_MYSQL_DSN"

// Schema is a set of migrations: the logger's or the web's.
type Schema struct {
	name string // in database and fixture directory names
	dir  string // the migrations, relative to the repository root
}

var (
	// Logger is the daemon's schema (migrations/), read through the
	// service's read-only pool.
	Logger = Schema{name: "logger", dir: "migrations"}
	// Web is the web's own schema (web/migrations/): accounts, audit,
	// tokens.
	Web = Schema{name: "web", dir: filepath.Join("web", "migrations")}
)

// DB is a test's own database. It is dropped when the test ends, after
// the pool is closed.
type DB struct {
	*sql.DB
	Name   string // the database's name
	DSN    string // a DSN for it, to open another pool
	schema Schema
}

//go:embed fixtures
var fixtures embed.FS

// pending holds schema changes whose migration another part of the
// project is still writing, pending/<schema>/*.sql, applied after the
// migrations. Each must do nothing when its migration has already made
// the change, so the file can stay until someone removes it.
//
//go:embed pending
var pending embed.FS

// readyTable marks a template whose migrations all applied. It is not
// copied into the tests' databases.
const readyTable = "testdb_ready"

// server is the connection to the test MySQL, shared by the tests of
// one process.
type server struct {
	cfg   *mysql.Config // the configured DSN, without a database
	admin *sql.DB       // multiStatements, no default database
}

var (
	srvOnce sync.Once
	srv     *server
	srvErr  error

	tplMu sync.Mutex
	tpls  = map[string]*tplEntry{}
)

type tplEntry struct {
	once sync.Once
	tpl  *template
	err  error
}

type template struct {
	name   string
	tables []table
}

type table struct {
	name   string
	create string // SHOW CREATE TABLE, unqualified
	rows   bool   // the migrations left rows in it
}

func connect() (*server, error) {
	srvOnce.Do(func() {
		cfg, err := mysql.ParseDSN(os.Getenv(EnvDSN))
		if err != nil {
			srvErr = fmt.Errorf("%s: %w", EnvDSN, err)
			return
		}
		cfg.DBName = ""
		ac := cfg.Clone()
		ac.MultiStatements = true
		conn, err := mysql.NewConnector(ac)
		if err != nil {
			srvErr = err
			return
		}
		admin := sql.OpenDB(conn)
		admin.SetMaxOpenConns(16)
		admin.SetConnMaxIdleTime(time.Minute)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := admin.PingContext(ctx); err != nil {
			admin.Close()
			srvErr = fmt.Errorf("test MySQL (%s): %w", EnvDSN, err)
			return
		}
		srv = &server{cfg: cfg, admin: admin}
	})
	return srv, srvErr
}

// New creates a database with the schema's migrations applied, loads
// the named fixtures into it (Load) and opens a pool on it. It skips
// the test when no test MySQL is configured and fails it when the
// configured one cannot be used.
func New(t testing.TB, s Schema, fixtures ...string) *DB {
	t.Helper()
	if os.Getenv(EnvDSN) == "" {
		t.Skip(EnvDSN + " is not set (docker-compose.test.yml, scripts/test-web.sh)")
	}
	sv, err := connect()
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := sv.template(s)
	if err != nil {
		t.Fatal(err)
	}

	var rnd [6]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		t.Fatal(err)
	}
	name := "test_" + s.name + "_" + hex.EncodeToString(rnd[:])
	// Registered before the pool's Close, so it runs after it: a
	// connection left in a transaction cannot hold the drop up.
	t.Cleanup(func() { sv.drop(t, name) })
	if err := retry(func() error { return sv.create(name, tpl) }); err != nil {
		t.Fatalf("creating %s from %s: %v", name, tpl.name, err)
	}

	cfg := sv.cfg.Clone()
	cfg.DBName = name
	c, err := mysql.NewConnector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	d := &DB{DB: sql.OpenDB(c), Name: name, DSN: cfg.FormatDSN(), schema: s}
	t.Cleanup(func() { d.DB.Close() })
	d.Load(t, fixtures...)
	return d
}

// Load runs the named fixtures, fixtures/<schema>/<name>.sql, in the
// database, in the order given.
func (d *DB) Load(t testing.TB, names ...string) {
	t.Helper()
	if len(names) == 0 {
		return
	}
	sv, err := connect()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn, err := sv.admin.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "USE "+quote(d.Name)); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		b, err := fixtures.ReadFile("fixtures/" + d.schema.name + "/" + n + ".sql")
		if err != nil {
			t.Fatalf("fixture %s/%s: %v", d.schema.name, n, err)
		}
		if _, err := conn.ExecContext(ctx, string(b)); err != nil {
			t.Fatalf("fixture %s/%s: %v", d.schema.name, n, err)
		}
	}
}

// create makes the database from the template, from scratch on every
// try.
func (sv *server) create(name string, tpl *template) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn, err := sv.admin.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var b strings.Builder
	fmt.Fprintf(&b, "SET SESSION lock_wait_timeout = 60;\nDROP DATABASE IF EXISTS %[1]s;\n"+
		"CREATE DATABASE %[1]s;\nUSE %[1]s;\nSET FOREIGN_KEY_CHECKS = 0;\n", quote(name))
	for _, tb := range tpl.tables {
		b.WriteString(tb.create + ";\n")
	}
	for _, tb := range tpl.tables {
		if tb.rows {
			fmt.Fprintf(&b, "INSERT INTO %s SELECT * FROM %s.%s;\n", quote(tb.name),
				quote(tpl.name), quote(tb.name))
		}
	}
	b.WriteString("SET FOREIGN_KEY_CHECKS = 1")
	_, err = conn.ExecContext(ctx, b.String())
	return err
}

func (sv *server) drop(t testing.TB, name string) {
	err := retry(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		conn, err := sv.admin.Conn(ctx)
		if err != nil {
			return err
		}
		defer conn.Close()
		_, err = conn.ExecContext(ctx, "SET SESSION lock_wait_timeout = 30; "+
			"DROP DATABASE IF EXISTS "+quote(name))
		return err
	})
	if err != nil {
		t.Errorf("dropping %s: %v", name, err)
	}
}

// retry runs f again while MySQL reports a deadlock: DDL on databases
// with foreign keys, run at once by several sessions, deadlocks in the
// data dictionary now and then, and the statement only needs a rerun.
func retry(f func() error) error {
	var err error
	for i := 0; i < 8; i++ {
		if err = f(); err == nil {
			return nil
		}
		var me *mysql.MySQLError
		if !errors.As(err, &me) || me.Number != 1213 {
			return err
		}
		time.Sleep(time.Duration(10+mrand.IntN(40*(i+1))) * time.Millisecond)
	}
	return err
}

// template returns the schema's template, building it first if no
// process has yet.
func (sv *server) template(s Schema) (*template, error) {
	tplMu.Lock()
	e := tpls[s.name]
	if e == nil {
		e = &tplEntry{}
		tpls[s.name] = e
	}
	tplMu.Unlock()
	e.once.Do(func() { e.tpl, e.err = sv.buildTemplate(s) })
	return e.tpl, e.err
}

type migration struct {
	name string
	sql  string
}

func readMigrations(s Schema) ([]migration, string, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, "", err
	}
	dir := filepath.Join(root, s.dir)
	paths, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return nil, "", err
	}
	if len(paths) == 0 {
		return nil, "", fmt.Errorf("no migrations in %s", dir)
	}
	sort.Strings(paths) // zero-padded versions: name order is version order
	h := sha256.New()
	var out []migration
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, "", err
		}
		n := filepath.Base(p)
		fmt.Fprintf(h, "%s\x00%d\x00", n, len(b))
		h.Write(b)
		out = append(out, migration{name: n, sql: string(b)})
	}
	extra, _ := fs.Glob(pending, "pending/"+s.name+"/*.sql")
	sort.Strings(extra)
	for _, p := range extra {
		b, err := pending.ReadFile(p)
		if err != nil {
			return nil, "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", p, len(b))
		h.Write(b)
		out = append(out, migration{name: p, sql: string(b)})
	}
	return out, hex.EncodeToString(h.Sum(nil))[:16], nil
}

// repoRoot finds the repository from the working directory, which go
// test sets to the package's directory.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if st, err := os.Stat(filepath.Join(dir, "webapi", "go.mod")); err == nil && !st.IsDir() {
			return dir, nil
		}
		up := filepath.Dir(dir)
		if up == dir {
			return "", errors.New("testdb: no repository (webapi/go.mod) above the working directory")
		}
		dir = up
	}
}

func (sv *server) buildTemplate(s Schema) (*template, error) {
	migs, sum, err := readMigrations(s)
	if err != nil {
		return nil, err
	}
	name := "test_tpl_" + s.name + "_" + sum
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	conn, err := sv.admin.Conn(ctx)
	if err != nil {
		return nil, err
	}
	// Closing the connection releases the lock as well.
	defer conn.Close()

	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 600)", name).Scan(&got); err != nil {
		return nil, err
	}
	if !got.Valid || got.Int64 != 1 {
		return nil, fmt.Errorf("testdb: timed out waiting for the lock on %s", name)
	}
	defer conn.ExecContext(context.Background(), "DO RELEASE_LOCK(?)", name)

	var ready int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES "+
		"WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?", name, readyTable).Scan(&ready); err != nil {
		return nil, err
	}
	if ready == 0 {
		// Absent, or left half-built by a process that died.
		if _, err := conn.ExecContext(ctx, "DROP DATABASE IF EXISTS "+quote(name)+
			"; CREATE DATABASE "+quote(name)+"; USE "+quote(name)); err != nil {
			return nil, err
		}
		for _, m := range migs {
			if _, err := conn.ExecContext(ctx, m.sql); err != nil {
				return nil, fmt.Errorf("testdb: %s/%s: %w", s.dir, m.name, err)
			}
		}
		if _, err := conn.ExecContext(ctx, "CREATE TABLE "+quote(name)+"."+readyTable+
			" (id INT)"); err != nil {
			return nil, err
		}
	}
	return snapshot(ctx, conn, name)
}

// snapshot reads the template's table definitions.
func snapshot(ctx context.Context, conn *sql.Conn, name string) (*template, error) {
	var others int
	if err := conn.QueryRowContext(ctx, "SELECT "+
		"(SELECT COUNT(*) FROM information_schema.VIEWS WHERE TABLE_SCHEMA = ?) + "+
		"(SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA = ?) + "+
		"(SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = ?) + "+
		"(SELECT COUNT(*) FROM information_schema.EVENTS WHERE EVENT_SCHEMA = ?)",
		name, name, name, name).Scan(&others); err != nil {
		return nil, err
	}
	if others != 0 {
		return nil, fmt.Errorf("testdb: %s has views, triggers, routines or events, "+
			"which testdb does not copy yet", name)
	}

	rows, err := conn.QueryContext(ctx, "SELECT TABLE_NAME FROM information_schema.TABLES "+
		"WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE' AND TABLE_NAME <> ? "+
		"ORDER BY TABLE_NAME", name, readyTable)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tpl := &template{name: name}
	for _, n := range names {
		tb := table{name: n}
		var ignored string
		if err := conn.QueryRowContext(ctx, "SHOW CREATE TABLE "+quote(name)+"."+quote(n)).
			Scan(&ignored, &tb.create); err != nil {
			return nil, err
		}
		var any int
		if err := conn.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM "+quote(name)+
			"."+quote(n)+")").Scan(&any); err != nil {
			return nil, err
		}
		tb.rows = any == 1
		tpl.tables = append(tpl.tables, tb)
	}
	return tpl, nil
}

func quote(id string) string {
	return "`" + strings.ReplaceAll(id, "`", "``") + "`"
}
