// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// PhotoListLimit is the most photos one GET /v1/users/{id}/photos
// answers (the daemon refuses more).
const PhotoListLimit = 100

// photoListTimeout bounds a photo list: the daemon gives Telegram up
// to 15 s and answers from its cache for a while after; a daemon
// slower than this counts as away.
const photoListTimeout = 20 * time.Second

// The outcomes of GET /v1/users/{id}/photos and /v1/chats/{id}/photos.
const (
	PhotosOK        = "ok"
	PhotosFloodWait = "flood_wait"
	PhotosError     = "error"
)

// ListedPhoto is one photo of a user's or a chat's list as Telegram
// has it. PhotoID is TDLib's id, a string as it does not fit a JSON
// number, and Date the photo's (Unix seconds); both are empty for a
// group's current photo that no change message shows. FileID is its
// largest size's telegram_files row.
type ListedPhoto struct {
	PhotoID   *string `json:"photo_id"`
	Date      *int64  `json:"date"`
	FileID    uint64  `json:"file_id"`
	IsCurrent bool    `json:"is_current"`
	Animated  bool    `json:"animated"`
}

// PhotoList is the daemon's answer to GET /v1/users/{id}/photos and
// GET /v1/chats/{id}/photos. Answering ok, the daemon has recorded the
// file rows and the history entries it lacked, which is what the web
// asks it for; the list itself is for logs and tests. RetryAfter is in
// seconds, for PhotosFloodWait only; Message is Telegram's, for
// PhotosError.
type PhotoList struct {
	Outcome    string
	Message    string
	RetryAfter int64
	Photos     []ListedPhoto
	Total      int64
}

// UserPhotos is GET /v1/users/{id}/photos?offset=0&limit=100: the
// user's profile photos from Telegram (getUserProfilePhotos), newest
// first. ok is false when there is no answer to go by: the client is
// off, the daemon is away, slow or not ready, the user unknown to it
// (404), or its answer is not a photo list's. A flood wait or a
// Telegram error is an answer (Outcome). reply is the call itself,
// for logs.
func (c *Client) UserPhotos(ctx context.Context, userID int64) (PhotoList, bool, Reply) {
	return c.photoList(ctx, "/v1/users/"+strconv.FormatInt(userID, 10)+
		"/photos?offset=0&limit="+strconv.Itoa(PhotoListLimit), photoListTimeout)
}

// ChatPhotos is GET /v1/chats/{id}/photos: a group's or channel's
// current photo and its earlier ones as Telegram has them, answered as
// UserPhotos is.
func (c *Client) ChatPhotos(ctx context.Context, chatID int64) (PhotoList, bool, Reply) {
	return c.photoList(ctx, "/v1/chats/"+strconv.FormatInt(chatID, 10)+"/photos",
		photoListTimeout)
}

func (c *Client) photoList(ctx context.Context, path string, timeout time.Duration) (PhotoList, bool, Reply) {
	r := c.Call(ctx, http.MethodGet, path, nil, timeout)
	var raw struct {
		Outcome    string         `json:"outcome"`
		Message    string         `json:"message"`
		RetryAfter int64          `json:"retry_after"`
		Photos     *[]ListedPhoto `json:"photos"`
		Total      *int64         `json:"total"`
	}
	if !r.OK() || r.Body == nil || json.Unmarshal(r.Body, &raw) != nil {
		return PhotoList{}, false, r
	}
	l := PhotoList{Outcome: raw.Outcome, Message: raw.Message}
	switch raw.Outcome {
	case PhotosOK:
		if raw.Photos == nil {
			return PhotoList{}, false, r
		}
		l.Photos = *raw.Photos
		l.Total = int64(len(l.Photos))
		if raw.Total != nil {
			l.Total = *raw.Total
		}
	case PhotosFloodWait:
		l.RetryAfter = max(raw.RetryAfter, 0)
	case PhotosError:
	default:
		return PhotoList{}, false, r
	}
	return l, true, r
}
