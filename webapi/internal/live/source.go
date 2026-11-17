// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package live

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"tgloggerd/webapi/internal/daemon"
)

// Source is where the events come from. Stream connects once, calls
// emit for every event in order, from one goroutine, and returns when
// the connection ends. since is the last seq the Hub has seen (0: start
// from now). The first event of a connection is a hello or a resync,
// as on the daemon's stream.
//
// The daemon's GET /v1/events is the one Source today. With several web
// instances or several publishers a broker would be another.
type Source interface {
	Stream(ctx context.Context, since uint64, emit func(Event)) error
}

// maxFrame bounds one line of the daemon's stream; its events are small
// (a stats.changed with two lists of 1000 ids is the largest).
const maxFrame = 1 << 20

// DaemonSource reads the daemon's GET /v1/events.
type DaemonSource struct {
	Client *daemon.Client
	Log    *slog.Logger
	// Idle is how long the stream may stay silent before it counts as
	// dead; the daemon sends a keepalive every 15 seconds. 0: 45 s.
	Idle time.Duration
}

// errEnded is a stream the daemon closed.
var errEnded = errors.New("the daemon closed the event stream")

// Stream implements Source.
func (d DaemonSource) Stream(ctx context.Context, since uint64, emit func(Event)) error {
	idle := d.Idle
	if idle <= 0 {
		idle = 45 * time.Second
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	body, err := d.Client.OpenEvents(ctx, since)
	if err != nil {
		return err
	}
	defer body.Close()

	// A connection that stops delivering even keepalives is dead, though
	// TCP may not notice for a long time.
	watchdog := time.AfterFunc(idle, cancel)
	defer watchdog.Stop()

	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 4096), maxFrame)
	var data []byte
	for sc.Scan() {
		watchdog.Reset(idle)
		line := sc.Bytes()
		switch {
		case len(line) == 0:
			if len(data) == 0 {
				continue
			}
			var e Event
			if err := json.Unmarshal(data, &e); err != nil || e.Kind == "" {
				d.log().Warn("unreadable daemon event", "data", string(data))
			} else {
				emit(e)
			}
			data = data[:0]
		case line[0] == ':':
			// A comment: the keepalive.
		default:
			field, value, _ := bytes.Cut(line, []byte(":"))
			value = bytes.TrimPrefix(value, []byte(" "))
			if string(field) == "data" {
				if len(data) > 0 {
					data = append(data, '\n')
				}
				data = append(data, value...)
			}
		}
	}
	if err := sc.Err(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("event stream idle or cancelled: %w", err)
		}
		return err
	}
	return errEnded
}

func (d DaemonSource) log() *slog.Logger {
	if d.Log != nil {
		return d.Log
	}
	return slog.Default()
}
