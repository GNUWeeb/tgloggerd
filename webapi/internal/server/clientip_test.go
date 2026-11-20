// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func prefixes(t *testing.T, ss ...string) TrustedProxies {
	var out TrustedProxies
	for _, s := range ss {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}

func TestClientAddress(t *testing.T) {
	// A local proxy in front of the service, and a CDN in front of it.
	trusted := prefixes(t, "10.0.88.1/32", "173.245.48.0/20", "2400:cb00::/32")

	cases := []struct {
		name    string
		proxies TrustedProxies
		peer    string
		xff     []string
		want    string
	}{
		{"no proxies trusted: the peer", nil, "10.0.88.1:4000",
			[]string{"198.51.100.7, 173.245.48.9"}, "10.0.88.1"},
		{"untrusted peer: its header is ignored", trusted, "192.0.2.50:4000",
			[]string{"198.51.100.7"}, "192.0.2.50"},
		{"through the CDN and the local proxy", trusted, "10.0.88.1:4000",
			[]string{"198.51.100.7, 173.245.48.9"}, "198.51.100.7"},
		{"forged entries left of the client are not reached", trusted, "10.0.88.1:4000",
			[]string{"203.0.113.1, 10.0.88.1, 198.51.100.7, 173.245.48.9"}, "198.51.100.7"},
		{"origin reached directly, forging the CDN", trusted, "10.0.88.1:4000",
			[]string{"173.245.48.9, 192.0.2.66"}, "192.0.2.66"},
		{"several header lines", trusted, "10.0.88.1:4000",
			[]string{"198.51.100.7", "173.245.48.9"}, "198.51.100.7"},
		{"an IPv6 client through an IPv6 CDN hop", trusted, "10.0.88.1:4000",
			[]string{"2001:db8::5, 2400:cb00::1"}, "2001:db8::5"},
		{"no header from a trusted peer: the peer", trusted, "10.0.88.1:4000",
			nil, "10.0.88.1"},
		{"garbage stops the walk at the last good hop", trusted, "10.0.88.1:4000",
			[]string{"198.51.100.7, not-an-ip, 173.245.48.9"}, "173.245.48.9"},
		{"all hops trusted: the leftmost", trusted, "10.0.88.1:4000",
			[]string{"173.245.48.2, 173.245.48.9"}, "173.245.48.2"},
		{"IPv4-mapped peer", trusted, "[::ffff:10.0.88.1]:4000",
			[]string{"198.51.100.7, 173.245.48.9"}, "198.51.100.7"},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		r.RemoteAddr = c.peer
		for _, v := range c.xff {
			r.Header.Add("X-Forwarded-For", v)
		}
		if got := c.proxies.Client(r); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

// The handlers see the client through clientIP once the server's
// middleware has run.
func TestClientIPThroughMiddleware(t *testing.T) {
	s := &Server{proxies: prefixes(t, "10.0.88.1/32")}
	var got string
	h := s.withClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = clientIP(r)
	}))
	r := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	r.RemoteAddr = "10.0.88.1:4000"
	r.Header.Set("X-Forwarded-For", "198.51.100.7")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if got != "198.51.100.7" {
		t.Fatalf("clientIP = %s", got)
	}
}
