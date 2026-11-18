// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"tgloggerd/webapi/internal/mail/mailtest"
)

// syncBuffer is a log sink the workers may write to while the test
// reads it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// start runs a Mailer against srv until the test ends.
func start(t *testing.T, srv *mailtest.Server, sec Security, tweak func(*Config)) (*Mailer, *syncBuffer) {
	t.Helper()
	cfg := Config{
		Host: srv.Host(), Port: srv.Port(), From: "tgloggerd <noreply@example.org>",
		Security: sec, TLSConfig: &tls.Config{RootCAs: srv.CertPool()},
		Timeout: 5 * time.Second, Retries: []time.Duration{10 * time.Millisecond, 10 * time.Millisecond},
	}
	if tweak != nil {
		tweak(&cfg)
	}
	logs := &syncBuffer{}
	m, err := New(cfg, slog.New(slog.NewTextHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		m.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return m, logs
}

const secret = "https://tgd.example.org/password/set#token=SECRET-TOKEN"

func msg(to string) Message {
	return Message{
		To: to, Subject: "Choose your password, Zoë", Kind: "invite",
		Text: "Hello Zoë,\n\nOpen " + secret + " to choose it.\n",
		HTML: "<p>Hello Zoë,</p><p>Open <a href=\"" + secret + "\">this link</a>.</p>",
	}
}

// A message arrives with its subject, both parts and UTF-8 intact, in
// each of the three connection modes; the log names it but never
// holds its body or the link in it.
func TestDeliver(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		mode mailtest.Mode
		sec  Security
		auth bool
	}{
		{"plain", mailtest.Plain, None, false},
		{"starttls", mailtest.StartTLS, StartTLS, true},
		{"tls", mailtest.TLS, TLS, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			srv := mailtest.New(t, c.mode)
			if c.auth {
				srv.RequireAuth("relay-user", "relay-pass")
			}
			m, logs := start(t, srv, c.sec, func(cfg *Config) {
				if c.auth {
					cfg.User, cfg.Password = "relay-user", "relay-pass"
				}
			})
			if !m.Send(msg("zoe@example.org")) {
				t.Fatal("not queued")
			}
			got := srv.Wait("zoe@example.org", 1, 10*time.Second)[0]
			if got.Subject != "Choose your password, Zoë" || got.From != "noreply@example.org" {
				t.Fatalf("subject %q from %q", got.Subject, got.From)
			}
			if !strings.Contains(got.Text, "Hello Zoë,\n\nOpen "+secret) ||
				!strings.Contains(got.HTML, `<a href="`+secret+`">`) {
				t.Fatalf("text %q html %q", got.Text, got.HTML)
			}
			if got.TLS != (c.sec != None) {
				t.Fatalf("tls %v", got.TLS)
			}
			if c.auth && got.Auth != "relay-user" {
				t.Fatalf("auth %q", got.Auth)
			}
			for _, h := range []string{"Auto-Submitted: auto-generated", "Message-ID: <", "MIME-Version: 1.0"} {
				if !bytes.Contains(got.Raw, []byte(h)) {
					t.Fatalf("no %q in\n%s", h, got.Raw)
				}
			}
			waitLog(t, logs, "mail sent")
			if l := logs.String(); !strings.Contains(l, "kind=invite") ||
				!strings.Contains(l, "to=zoe@example.org") || strings.Contains(l, "SECRET") {
				t.Fatalf("log %s", l)
			}
		})
	}
}

func waitLog(t *testing.T, logs *syncBuffer, s string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(logs.String(), s) {
		if time.Now().After(deadline) {
			t.Fatalf("no %q in the log:\n%s", s, logs.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A temporary refusal is retried after each wait until it goes
// through; one past the last retry gives up.
func TestRetries(t *testing.T) {
	t.Parallel()
	srv := mailtest.New(t, mailtest.Plain)
	srv.Fail(2, 451)
	m, logs := start(t, srv, None, nil)
	m.Send(msg("retry@example.org"))
	srv.Wait("retry@example.org", 1, 10*time.Second)
	if n := srv.Attempts(); n != 3 {
		t.Fatalf("%d attempts, want 3", n)
	}
	waitLog(t, logs, "attempt=3")
	if !strings.Contains(logs.String(), "will retry") {
		t.Fatalf("log %s", logs.String())
	}

	srv2 := mailtest.New(t, mailtest.Plain)
	srv2.Fail(3, 451)
	m2, logs2 := start(t, srv2, None, nil)
	m2.Send(msg("never@example.org"))
	waitLog(t, logs2, "giving up")
	if n := srv2.Attempts(); n != 3 || len(srv2.Messages()) != 0 {
		t.Fatalf("%d attempts, %d messages", n, len(srv2.Messages()))
	}
	if strings.Contains(logs2.String(), "SECRET") {
		t.Fatalf("log %s", logs2.String())
	}
}

// A permanent refusal is not retried, and neither is a relay that
// does not offer the STARTTLS asked for: nothing goes out in plain
// text.
func TestPermanentFailures(t *testing.T) {
	t.Parallel()
	srv := mailtest.New(t, mailtest.Plain)
	srv.Fail(1, 550)
	m, logs := start(t, srv, None, nil)
	m.Send(msg("bounce@example.org"))
	waitLog(t, logs, "giving up")
	if n := srv.Attempts(); n != 1 {
		t.Fatalf("%d attempts", n)
	}

	plain := mailtest.New(t, mailtest.Plain)
	m2, logs2 := start(t, plain, StartTLS, nil)
	m2.Send(msg("clear@example.org"))
	waitLog(t, logs2, "giving up")
	if n := plain.Attempts(); n != 0 {
		t.Fatalf("%d MAIL commands in plain text", n)
	}
}

// Send never blocks: a full queue drops the message, and a nil Mailer
// (mail off) drops everything. Malformed messages are refused before
// they reach a header.
func TestSendNeverBlocks(t *testing.T) {
	t.Parallel()
	var off *Mailer
	if off.Enabled() || off.Send(msg("a@example.org")) {
		t.Fatal("a nil Mailer sends")
	}
	logs := &syncBuffer{}
	m, err := New(Config{Host: "127.0.0.1", Port: 25, From: "noreply@example.org",
		Security: None, Queue: 1}, slog.New(slog.NewTextHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	// Not running: the first fills the queue, the second is dropped.
	if !m.Send(msg("a@example.org")) || m.Send(msg("b@example.org")) {
		t.Fatal("queue of one")
	}
	if !strings.Contains(logs.String(), "queue is full") {
		t.Fatalf("log %s", logs.String())
	}
	for _, bad := range []Message{
		{To: "Zoe <zoe@example.org>", Subject: "x"},
		{To: "zoe@example.org", Subject: "x\r\nBcc: evil@example.org"},
	} {
		if m.Send(bad) {
			t.Fatalf("queued %+v", bad)
		}
	}
}

// New refuses a configuration that cannot work.
func TestNewChecks(t *testing.T) {
	t.Parallel()
	log := slog.New(slog.NewTextHandler(&syncBuffer{}, nil))
	ok := Config{Host: "smtp.example.org", Port: 587, From: "noreply@example.org", Security: StartTLS}
	if _, err := New(ok, log); err != nil {
		t.Fatal(err)
	}
	for _, tweak := range []func(*Config){
		func(c *Config) { c.Host = "" },
		func(c *Config) { c.Port = 0 },
		func(c *Config) { c.Security = "ssl" },
		func(c *Config) { c.From = "not an address" },
	} {
		c := ok
		tweak(&c)
		if _, err := New(c, log); err == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
}
