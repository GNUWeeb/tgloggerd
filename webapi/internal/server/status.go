// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"fmt"
	"sync"
	"syscall"

	"tgloggerd/webapi/internal/daemon"
	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// The dashboard's logger status card (DashboardProps.status): whether
// the daemon's Telegram session works, when the newest message was
// logged, how many forwarding configurations forward, and how full the
// file store is. It says something about the whole archive, so only a
// viewer that sees every group and private chat gets it.

type statusJSON struct {
	Session          string  `json:"session"`
	LastMessageAt    string  `json:"last_message_at"`
	ForwardingActive int64   `json:"forwarding_active"`
	StoreUsed        string  `json:"store_used"`
	StoreTotal       string  `json:"store_total"`
	StoreRatio       float64 `json:"store_ratio"`
}

// seesStatus is whether the scope gets the status card.
func seesStatus(sc scope.Scope) bool {
	return sc.SeesAllGroups() && sc.SeesPrivate()
}

// daemonProbe is the daemon's two answers for the card, asked at once
// and side by side with the dashboard's own queries, so the page waits
// at most the calls' timeout for them.
type daemonProbe struct {
	wg        sync.WaitGroup
	status    daemon.Status
	statusOK  bool
	storage   daemon.Storage
	storageOK bool
}

func (s *Server) probeDaemon(ctx context.Context) *daemonProbe {
	p := &daemonProbe{}
	p.wg.Add(2)
	go func() {
		defer p.wg.Done()
		p.status, p.statusOK = s.daemon.Status(ctx)
	}()
	go func() {
		defer p.wg.Done()
		p.storage, p.storageOK = s.daemon.Storage(ctx)
	}()
	return p
}

// fsSize is the size in bytes of the file system holding path.
func fsSize(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return st.Blocks * uint64(st.Bsize), nil
}

// loggerStatus fills the card once p has answered. The store's size is
// the daemon's when it answers (the bytes its janitor caps), else the
// sum over telegram_files; its total is the cap when one is set, else
// the size of the file system holding WEB_STORAGE_DIR.
func (s *Server) loggerStatus(ctx context.Context, sc scope.Scope, p *daemonProbe) (*statusJSON, error) {
	last, err := logdb.LastMessageAt(ctx, s.ro)
	if err != nil {
		return nil, err
	}
	fwd, err := logdb.ActiveForwarding(ctx, s.ro, sc)
	if err != nil {
		return nil, err
	}
	p.wg.Wait()

	st := &statusJSON{Session: "disconnected", ForwardingActive: fwd}
	if p.statusOK && p.status.TDLib == "ready" {
		st.Session = "connected"
	}
	if last > 0 {
		st.LastMessageAt = iso(last)
	}

	var used, total int64
	if p.storageOK {
		used, total = p.storage.Bytes, p.storage.MaxBytes
	} else if n, ok := logdb.StoredBytes(ctx, s.ro, sc); ok {
		used = n
	}
	if total <= 0 {
		total = 0
		if n, err := s.statfs(s.media.StorageDir); err != nil {
			s.log.Debug("status: store size", "dir", s.media.StorageDir, "err", err)
		} else {
			total = int64(min(n, uint64(1<<63-1)))
		}
	}
	st.StoreUsed, st.StoreTotal = humanBytes(used), humanBytes(total)
	st.StoreRatio = storeRatio(used, total)
	return st, nil
}

// humanBytes is a size in binary units with one decimal, from KiB to
// TiB: "349.2 GiB", "0.5 KiB", "2048.0 TiB".
func humanBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	v := float64(n) / 1024
	unit := "KiB"
	for _, u := range []string{"MiB", "GiB", "TiB"} {
		// 1023.96 KiB would print as "1024.0 KiB".
		if v < 1023.95 {
			break
		}
		v /= 1024
		unit = u
	}
	return fmt.Sprintf("%.1f %s", v, unit)
}

// storeRatio is used/total clamped to 0..1; 0 when total is 0.
func storeRatio(used, total int64) float64 {
	if total <= 0 || used <= 0 {
		return 0
	}
	return min(float64(used)/float64(total), 1)
}
