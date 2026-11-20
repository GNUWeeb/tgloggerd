// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// The address a request came from, for the audit log, the login
// throttle and the request log.
//
// Behind reverse proxies the connection's peer is the nearest proxy,
// not the client. Each proxy appends the address it was reached from
// to X-Forwarded-For, so the chain, read from the right, is: the peer,
// then each hop before it. Every hop that is a trusted proxy is
// skipped; the first one that is not is the client. A client can put
// anything in the header it sends, but what it writes lands to the
// left of the hops the trusted proxies append, so it is never reached
// while those hops are trusted. With no trusted proxies configured the
// peer is the client, as before.

// TrustedProxies are the networks whose hops are skipped
// (config.Config.TrustedProxies).
type TrustedProxies []netip.Prefix

func (t TrustedProxies) trusts(a netip.Addr) bool {
	a = a.Unmap()
	for _, p := range t {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// Client is the address the request came from.
func (t TrustedProxies) Client(r *http.Request) string {
	peer := r.RemoteAddr
	if host, _, err := net.SplitHostPort(peer); err == nil {
		peer = host
	}
	addr, err := netip.ParseAddr(peer)
	if err != nil || !t.trusts(addr) {
		return peer
	}

	// The peer is a trusted proxy: walk its chain from the right.
	var hops []string
	for _, v := range r.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(v, ",")...)
	}
	client := addr
	for i := len(hops) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			// Not an address: nothing left of it can be trusted, and
			// the last hop known good is the best answer.
			break
		}
		client = a.Unmap()
		if !t.trusts(client) {
			break
		}
	}
	return client.String()
}

type clientKey struct{}

// withClient records the request's client address for clientIP.
func (s *Server) withClient(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), clientKey{}, s.proxies.Client(r))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// clientIP is the address the request came from (see TrustedProxies).
func clientIP(r *http.Request) string {
	if v, ok := r.Context().Value(clientKey{}).(string); ok {
		return v
	}
	return TrustedProxies(nil).Client(r)
}
