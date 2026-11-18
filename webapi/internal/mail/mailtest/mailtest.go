// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package mailtest is a small SMTP server for tests: it accepts every
// message on a loopback port the system picks and keeps it, so a test
// reads what the service sent without any real mail leaving the
// machine. It speaks enough of SMTP for net/smtp: EHLO, STARTTLS, AUTH
// PLAIN, MAIL, RCPT, DATA, RSET, NOOP and QUIT, in plain text, with
// STARTTLS or with TLS from the start, on a certificate it makes for
// itself (CertPool trusts it).
package mailtest

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Mode is how clients connect.
type Mode int

const (
	Plain    Mode = iota // plain text, no STARTTLS offered
	StartTLS             // plain text, STARTTLS offered
	TLS                  // TLS from the start
)

// Message is one message the server accepted.
type Message struct {
	From    string
	To      []string
	Raw     []byte // as received, after the dot-unstuffing
	Subject string // decoded
	Text    string // the text/plain part, decoded, LF line ends
	HTML    string // the text/html part, decoded, LF line ends
	// TLS and Auth tell how the message came: over TLS, and the user
	// that authenticated ("" for none).
	TLS  bool
	Auth string
}

// Server is the test SMTP server.
type Server struct {
	t        testing.TB
	ln       net.Listener
	mode     Mode
	cert     tls.Certificate
	pool     *x509.CertPool
	mu       sync.Mutex
	user     string // required by AUTH PLAIN before MAIL, when set
	password string
	msgs     []Message
	fail     int // MAIL commands still to refuse
	failCode int
	attempts int // MAIL commands seen
	changed  chan struct{}
	wg       sync.WaitGroup
}

// New starts a server on 127.0.0.1, port 0, and stops it when the test
// ends.
func New(t testing.TB, mode Mode) *Server {
	t.Helper()
	cert, pool := selfSigned(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{t: t, ln: ln, mode: mode, cert: cert, pool: pool, changed: make(chan struct{})}
	s.wg.Add(1)
	go s.serve()
	t.Cleanup(func() {
		ln.Close()
		s.wg.Wait()
	})
	return s
}

// Host and Port are where the server listens.
func (s *Server) Host() string { return "127.0.0.1" }

// Port is the server's port.
func (s *Server) Port() int { return s.ln.Addr().(*net.TCPAddr).Port }

// CertPool trusts the server's certificate.
func (s *Server) CertPool() *x509.CertPool { return s.pool }

// RequireAuth makes MAIL wait for AUTH PLAIN with this user and
// password.
func (s *Server) RequireAuth(user, password string) {
	s.mu.Lock()
	s.user, s.password = user, password
	s.mu.Unlock()
}

// Fail makes the next n MAIL commands answer code (451 is a temporary
// failure, 550 a permanent one).
func (s *Server) Fail(n, code int) {
	s.mu.Lock()
	s.fail, s.failCode = n, code
	s.mu.Unlock()
}

// Attempts is how many MAIL commands the server has seen, refused ones
// included.
func (s *Server) Attempts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attempts
}

// Messages is every message accepted so far.
func (s *Server) Messages() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Message(nil), s.msgs...)
}

// To is the messages accepted for one recipient.
func (s *Server) To(addr string) []Message {
	var out []Message
	for _, m := range s.Messages() {
		for _, r := range m.To {
			if strings.EqualFold(r, addr) {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// Wait waits until addr has received n messages and returns them,
// failing the test after timeout.
func (s *Server) Wait(addr string, n int, timeout time.Duration) []Message {
	s.t.Helper()
	deadline := time.After(timeout)
	for {
		s.mu.Lock()
		ch := s.changed
		s.mu.Unlock()
		if got := s.To(addr); len(got) >= n {
			return got
		}
		select {
		case <-ch:
		case <-deadline:
			s.t.Fatalf("mailtest: %s got %d messages, want %d", addr, len(s.To(addr)), n)
			return nil
		}
	}
}

func (s *Server) serve() {
	defer s.wg.Done()
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(time.Minute))
			s.session(c)
		}()
	}
}

func (s *Server) session(c net.Conn) {
	isTLS := false
	if s.mode == TLS {
		tc := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{s.cert}})
		if err := tc.Handshake(); err != nil {
			return
		}
		c, isTLS = tc, true
	}
	r, w := bufio.NewReader(c), bufio.NewWriter(c)
	reply := func(lines ...string) bool {
		for _, l := range lines {
			w.WriteString(l + "\r\n")
		}
		return w.Flush() == nil
	}
	if !reply("220 mailtest ESMTP") {
		return
	}
	var (
		from string
		to   []string
		user string
	)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		verb, arg, _ := strings.Cut(line, " ")
		switch strings.ToUpper(verb) {
		case "EHLO", "HELO":
			ext := []string{"250-mailtest"}
			if s.mode == StartTLS && !isTLS {
				ext = append(ext, "250-STARTTLS")
			}
			ext = append(ext, "250-AUTH PLAIN", "250 8BITMIME")
			reply(ext...)
		case "STARTTLS":
			if s.mode != StartTLS || isTLS {
				reply("502 no")
				continue
			}
			reply("220 go ahead")
			tc := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{s.cert}})
			if err := tc.Handshake(); err != nil {
				return
			}
			c, isTLS = tc, true
			r, w = bufio.NewReader(c), bufio.NewWriter(c)
		case "AUTH":
			mech, resp, _ := strings.Cut(arg, " ")
			if !strings.EqualFold(mech, "PLAIN") {
				reply("504 PLAIN only")
				continue
			}
			b, err := base64.StdEncoding.DecodeString(resp)
			parts := strings.Split(string(b), "\x00")
			s.mu.Lock()
			wantUser, wantPW := s.user, s.password
			s.mu.Unlock()
			if err != nil || len(parts) != 3 || parts[1] != wantUser || parts[2] != wantPW {
				reply("535 authentication failed")
				continue
			}
			user = parts[1]
			reply("235 ok")
		case "MAIL":
			s.mu.Lock()
			s.attempts++
			fail, code := s.fail > 0, s.failCode
			if fail {
				s.fail--
			}
			needAuth := s.user != ""
			s.mu.Unlock()
			s.notify()
			switch {
			case fail:
				reply(strconv.Itoa(code) + " refused for the test")
				continue
			case needAuth && user == "":
				reply("530 authentication required")
				continue
			}
			from, to = addrArg(arg), nil
			reply("250 ok")
		case "RCPT":
			to = append(to, addrArg(arg))
			reply("250 ok")
		case "DATA":
			reply("354 go ahead")
			raw, err := readData(r)
			if err != nil {
				return
			}
			m := parse(raw)
			m.From, m.To, m.TLS, m.Auth = from, to, isTLS, user
			s.mu.Lock()
			s.msgs = append(s.msgs, m)
			s.mu.Unlock()
			s.notify()
			reply("250 queued")
		case "RSET", "NOOP":
			reply("250 ok")
		case "QUIT":
			reply("221 bye")
			return
		default:
			reply("500 unknown command")
		}
	}
}

// notify wakes the waiters.
func (s *Server) notify() {
	s.mu.Lock()
	close(s.changed)
	s.changed = make(chan struct{})
	s.mu.Unlock()
}

// addrArg is the address of "FROM:<a@b> SIZE=..." or "TO:<a@b>".
func addrArg(arg string) string {
	i, j := strings.IndexByte(arg, '<'), strings.IndexByte(arg, '>')
	if i < 0 || j < i {
		return ""
	}
	return arg[i+1 : j]
}

func readData(r *bufio.Reader) ([]byte, error) {
	var b bytes.Buffer
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if line == ".\r\n" || line == ".\n" {
			return b.Bytes(), nil
		}
		b.WriteString(strings.TrimPrefix(line, "."))
	}
}

// parse decodes what a test reads: the subject and the text and HTML
// parts. A malformed message keeps only Raw.
func parse(raw []byte) Message {
	m := Message{Raw: raw}
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return m
	}
	var dec mime.WordDecoder
	if s, err := dec.DecodeHeader(msg.Header.Get("Subject")); err == nil {
		m.Subject = s
	}
	ctype, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		return m
	}
	body := func(r io.Reader, enc string) string {
		if strings.EqualFold(enc, "quoted-printable") {
			r = quotedprintable.NewReader(r)
		}
		b, _ := io.ReadAll(r)
		return strings.ReplaceAll(string(b), "\r\n", "\n")
	}
	if !strings.HasPrefix(ctype, "multipart/") {
		m.Text = body(msg.Body, msg.Header.Get("Content-Transfer-Encoding"))
		return m
	}
	mr := multipart.NewReader(msg.Body, params["boundary"])
	for {
		p, err := mr.NextPart()
		if err != nil {
			return m
		}
		pt, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		// The multipart reader decodes quoted-printable itself.
		s := body(p, "")
		switch pt {
		case "text/plain":
			m.Text = s
		case "text/html":
			m.HTML = s
		}
	}
}

// selfSigned makes a certificate for 127.0.0.1 and localhost.
func selfSigned(t testing.TB) (tls.Certificate, *x509.CertPool) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "mailtest"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:     []string{"localhost"},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         true,

		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}
