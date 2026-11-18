// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package mail sends the service's email through an SMTP relay
// (docs/design/web-accounts.md section 8): set-password links,
// verification links and security notices.
//
// Sending is asynchronous: Send queues a message and returns at once,
// so a request never waits for the relay. Workers deliver the queue,
// retrying a message the relay refused for the time being (a 4xx
// answer, a connection that failed) after each wait of Config.Retries,
// and giving up on a permanent refusal (5xx). Every delivery and every
// failure is logged with the message's kind, recipient and subject,
// never its body, so the links in it reach no log.
//
// The queue lives in memory only. Its messages carry one-time links,
// which are stored nowhere else in plain text (the database keeps
// their hashes); a queue in the database would keep them. A message
// still queued when the service stops is lost, and the log says how
// many were; the links in it can be sent again.
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Security is how the connection to the relay is protected.
type Security string

const (
	// StartTLS connects in plain text and upgrades with STARTTLS
	// before anything is sent; a relay that does not offer it is
	// refused. The usual choice on port 587.
	StartTLS Security = "starttls"
	// TLS connects with TLS from the start (port 465).
	TLS Security = "tls"
	// None sends in plain text, for a relay on the same host or a test
	// server only; net/smtp refuses to send a password over it to
	// anything but localhost.
	None Security = "none"
)

// ParseSecurity reads a Security as the environment names it.
func ParseSecurity(s string) (Security, bool) {
	switch v := Security(strings.ToLower(s)); v {
	case StartTLS, TLS, None:
		return v, true
	}
	return "", false
}

// Config is the relay and the queue's settings.
type Config struct {
	Host     string
	Port     int
	User     string // "" sends without authentication
	Password string
	// From is the sender, "Name <address>" or a bare address.
	From     string
	Security Security
	// TLSConfig, when set, is used for TLS and STARTTLS instead of the
	// system's roots (tests trust their own server's certificate). Its
	// ServerName is set to Host when empty.
	TLSConfig *tls.Config
	// Timeout bounds one delivery attempt, connection to QUIT.
	Timeout time.Duration
	// Retries are the waits before each new attempt after a temporary
	// failure; a message is tried len(Retries)+1 times at most.
	Retries []time.Duration
	// Queue is how many messages may wait at once; Send drops a message
	// beyond it (and logs that).
	Queue int
	// Workers deliver at once at most.
	Workers int
}

// DefaultRetries are the waits between attempts: about a day in all,
// so a relay down for hours still gets the message.
var DefaultRetries = []time.Duration{
	30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute,
	time.Hour, 2 * time.Hour, 4 * time.Hour, 8 * time.Hour, 8 * time.Hour,
}

// Message is one email.
type Message struct {
	To      string // a bare address
	Subject string
	Text    string // the plain text body
	HTML    string // the same as simple HTML; "" sends the text only
	// Kind names the message in the log ("invite", "notice.role", ...).
	Kind string
}

// Mailer queues and delivers messages. A nil *Mailer is mail turned
// off: Enabled is false and Send drops everything.
type Mailer struct {
	cfg    Config
	from   *mail.Address
	log    *slog.Logger
	queue  chan job
	now    func() time.Time
	wg     sync.WaitGroup
	stop   chan struct{}
	closed sync.Once
}

type job struct {
	msg     Message
	attempt int // attempts made so far
}

// New checks the configuration and returns a Mailer; Run starts it.
func New(cfg Config, log *slog.Logger) (*Mailer, error) {
	if cfg.Host == "" {
		return nil, errors.New("no SMTP host")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, fmt.Errorf("SMTP port %d", cfg.Port)
	}
	if _, ok := ParseSecurity(string(cfg.Security)); !ok {
		return nil, fmt.Errorf("SMTP security %q is not starttls, tls or none", cfg.Security)
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("sender %q: %w", cfg.From, err)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Retries == nil {
		cfg.Retries = DefaultRetries
	}
	if cfg.Queue <= 0 {
		cfg.Queue = 1000
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 2
	}
	return &Mailer{
		cfg:   cfg,
		from:  from,
		log:   log,
		queue: make(chan job, cfg.Queue),
		now:   time.Now,
		stop:  make(chan struct{}),
	}, nil
}

// Enabled is whether mail goes out at all.
func (m *Mailer) Enabled() bool {
	return m != nil
}

// Send queues msg and returns at once. It reports false, and logs why,
// when mail is off, the message is malformed or the queue is full.
func (m *Mailer) Send(msg Message) bool {
	if m == nil {
		return false
	}
	if err := check(msg); err != nil {
		m.log.Error("mail not queued", "kind", msg.Kind, "err", err)
		return false
	}
	select {
	case m.queue <- job{msg: msg}:
		return true
	default:
		m.log.Error("mail not queued: the queue is full",
			"kind", msg.Kind, "to", msg.To, "subject", msg.Subject)
		return false
	}
}

// check refuses what would break the message's header: a recipient
// that is not one bare address, a line break in the subject.
func check(msg Message) error {
	a, err := mail.ParseAddress(msg.To)
	if err != nil || a.Address != msg.To {
		return fmt.Errorf("recipient %q is not an address", msg.To)
	}
	if strings.ContainsAny(msg.Subject, "\r\n") {
		return errors.New("line break in the subject")
	}
	return nil
}

// Run delivers the queue until ctx ends. Messages still waiting then,
// or waiting for a retry, are dropped and counted in the log.
func (m *Mailer) Run(ctx context.Context) {
	if m == nil {
		return
	}
	for range m.cfg.Workers {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.work(ctx)
		}()
	}
	<-ctx.Done()
	m.closed.Do(func() { close(m.stop) })
	m.wg.Wait()
	if n := len(m.queue); n > 0 {
		m.log.Warn("mail dropped on shutdown", "messages", n)
	}
}

func (m *Mailer) work(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-m.queue:
			m.attempt(ctx, j)
		}
	}
}

// attempt delivers one message and schedules its retry on a temporary
// failure. The retry waits on a timer, not in a worker, so a relay
// that is down holds no worker up.
func (m *Mailer) attempt(ctx context.Context, j job) {
	j.attempt++
	id, err := m.deliver(ctx, j.msg)
	if err == nil {
		m.log.Info("mail sent", "kind", j.msg.Kind, "to", j.msg.To,
			"subject", j.msg.Subject, "message_id", id, "attempt", j.attempt)
		return
	}
	if permanent(err) || j.attempt > len(m.cfg.Retries) {
		m.log.Error("mail not sent, giving up", "kind", j.msg.Kind, "to", j.msg.To,
			"subject", j.msg.Subject, "attempt", j.attempt, "err", err)
		return
	}
	wait := m.cfg.Retries[j.attempt-1]
	m.log.Warn("mail not sent, will retry", "kind", j.msg.Kind, "to", j.msg.To,
		"subject", j.msg.Subject, "attempt", j.attempt, "retry_in", wait, "err", err)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		t := time.NewTimer(wait)
		defer t.Stop()
		select {
		case <-t.C:
		case <-m.stop:
			m.log.Warn("mail dropped on shutdown", "kind", j.msg.Kind, "to", j.msg.To)
			return
		}
		select {
		case m.queue <- j:
		default:
			m.log.Error("mail dropped: the queue is full", "kind", j.msg.Kind, "to", j.msg.To)
		}
	}()
}

// permanent is whether trying again cannot help: the relay refused for
// good (a 5xx answer), or it does not offer the STARTTLS required.
func permanent(err error) bool {
	var te *textproto.Error
	return errors.Is(err, errNoStartTLS) || errors.As(err, &te) && te.Code >= 500
}

// errNoStartTLS: the relay does not offer STARTTLS, and nothing is sent
// in plain text to it.
var errNoStartTLS = errors.New("the relay does not offer STARTTLS")

func (m *Mailer) tlsConfig() *tls.Config {
	c := &tls.Config{MinVersion: tls.VersionTLS12}
	if m.cfg.TLSConfig != nil {
		c = m.cfg.TLSConfig.Clone()
	}
	if c.ServerName == "" {
		c.ServerName = m.cfg.Host
	}
	return c
}

// deliver makes one attempt and returns the Message-ID sent.
func (m *Mailer) deliver(ctx context.Context, msg Message) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()
	addr := net.JoinHostPort(m.cfg.Host, strconv.Itoa(m.cfg.Port))
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", err
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	if m.cfg.Security == TLS {
		tc := tls.Client(conn, m.tlsConfig())
		if err := tc.HandshakeContext(ctx); err != nil {
			conn.Close()
			return "", err
		}
		conn = tc
	}
	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		conn.Close()
		return "", err
	}
	defer c.Close()
	if err := c.Hello(helloName(m.from.Address)); err != nil {
		return "", err
	}
	if m.cfg.Security == StartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return "", errNoStartTLS
		}
		if err := c.StartTLS(m.tlsConfig()); err != nil {
			return "", err
		}
	}
	if m.cfg.User != "" {
		if err := c.Auth(smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)); err != nil {
			return "", err
		}
	}
	body, id, err := m.compose(msg)
	if err != nil {
		return "", err
	}
	if err := c.Mail(m.from.Address); err != nil {
		return "", err
	}
	if err := c.Rcpt(msg.To); err != nil {
		return "", err
	}
	w, err := c.Data()
	if err != nil {
		return "", err
	}
	if _, err := w.Write(body); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	_ = c.Quit()
	return id, nil
}

// helloName is the name given in EHLO: the sender's domain, which a
// relay checks more kindly than "localhost".
func helloName(from string) string {
	if i := strings.LastIndexByte(from, '@'); i >= 0 && i+1 < len(from) {
		return from[i+1:]
	}
	return "localhost"
}

// compose builds the message: headers, then the text and, when there
// is one, the HTML version as multipart/alternative, both UTF-8 in
// quoted-printable.
func (m *Mailer) compose(msg Message) ([]byte, string, error) {
	var rnd [12]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return nil, "", err
	}
	id := "<" + hex.EncodeToString(rnd[:]) + "@" + helloName(m.from.Address) + ">"

	var b bytes.Buffer
	h := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	h("From", m.from.String())
	h("To", msg.To)
	h("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	h("Date", m.now().Format(time.RFC1123Z))
	h("Message-ID", id)
	h("MIME-Version", "1.0")
	// An automatic message: no out-of-office answers to it.
	h("Auto-Submitted", "auto-generated")

	part := func(w *bytes.Buffer, s string) error {
		qp := quotedprintable.NewWriter(w)
		if _, err := qp.Write([]byte(crlf(s))); err != nil {
			return err
		}
		return qp.Close()
	}
	if msg.HTML == "" {
		h("Content-Type", "text/plain; charset=utf-8")
		h("Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		if err := part(&b, msg.Text); err != nil {
			return nil, "", err
		}
		return b.Bytes(), id, nil
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	h("Content-Type", "multipart/alternative; boundary="+mw.Boundary())
	b.WriteString("\r\n")
	for _, p := range []struct{ ctype, s string }{
		{"text/plain; charset=utf-8", msg.Text},
		{"text/html; charset=utf-8", msg.HTML},
	} {
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {p.ctype},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, "", err
		}
		var pb bytes.Buffer
		if err := part(&pb, p.s); err != nil {
			return nil, "", err
		}
		if _, err := pw.Write(pb.Bytes()); err != nil {
			return nil, "", err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	b.Write(body.Bytes())
	return b.Bytes(), id, nil
}

// crlf ends every line with CRLF, as SMTP wants.
func crlf(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}
