// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AssetBase is the URL prefix of the client bundle (Vite's base).
const AssetBase = "/app/"

// EntryKey is the client entry's key in the Vite manifest.
const EntryKey = "src/entry-client.tsx"

// Assets are the client bundle's files a page links, and the build id
// the render service must have been built with for its markup to
// hydrate (web-ssr.md section 6).
type Assets struct {
	Build   string
	CSS     []string // stylesheets, in order
	Preload []string // modules the entry imports
	Entry   string   // the entry script
}

// manifestChunk is one entry of .vite/manifest.json.
type manifestChunk struct {
	File    string   `json:"file"`
	CSS     []string `json:"css"`
	Imports []string `json:"imports"`
	IsEntry bool     `json:"isEntry"`
}

// LoadAssets reads the client bundle's manifest in dist (the
// directory `vite build` wrote, web/app/dist/client) and lists what a
// page links: the entry script, the styles of the entry and of every
// chunk it imports, and those chunks as module preloads.
func LoadAssets(dist, build string) (Assets, error) {
	raw, err := os.ReadFile(filepath.Join(dist, ".vite", "manifest.json"))
	if err != nil {
		return Assets{Build: build}, err
	}
	return parseManifest(raw, build)
}

func parseManifest(raw []byte, build string) (Assets, error) {
	a := Assets{Build: build}
	var m map[string]manifestChunk
	if err := json.Unmarshal(raw, &m); err != nil {
		return a, fmt.Errorf("manifest: %w", err)
	}
	entry, ok := m[EntryKey]
	if !ok || !entry.IsEntry || entry.File == "" {
		return a, fmt.Errorf("manifest: no entry %s", EntryKey)
	}
	a.Entry = AssetBase + entry.File

	seenCSS := map[string]bool{}
	addCSS := func(files []string) {
		for _, f := range files {
			if !seenCSS[f] {
				seenCSS[f] = true
				a.CSS = append(a.CSS, AssetBase+f)
			}
		}
	}
	addCSS(entry.CSS)

	// The static imports, depth first, each once: a chunk shared by
	// several others is preloaded once.
	seen := map[string]bool{EntryKey: true}
	var walk func(keys []string)
	walk = func(keys []string) {
		for _, k := range keys {
			if seen[k] {
				continue
			}
			seen[k] = true
			c, ok := m[k]
			if !ok || c.File == "" {
				continue
			}
			a.Preload = append(a.Preload, AssetBase+c.File)
			addCSS(c.CSS)
			walk(c.Imports)
		}
	}
	walk(entry.Imports)
	return a, nil
}
