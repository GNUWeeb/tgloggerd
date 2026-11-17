// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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

// BuildFile is the file in the client bundle's directory where the
// build (web/app/scripts/build.ts) records the build id it gave the
// client bundle and the render entry alike.
const BuildFile = "build.json"

// DevBuild is the build id of a bundle that recorded none: what Vite
// gives both bundles when they are built without the build script.
const DevBuild = "dev"

// ReadBuild returns the build id recorded in dist's BuildFile, or
// DevBuild when there is no such file. A file that is there but does
// not hold an id is an error, since guessing would only make every
// render fail with a build mismatch.
func ReadBuild(dist string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dist, BuildFile))
	if errors.Is(err, fs.ErrNotExist) {
		return DevBuild, nil
	}
	if err != nil {
		return DevBuild, err
	}
	var b struct {
		Build string `json:"build"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return DevBuild, fmt.Errorf("%s: %w", BuildFile, err)
	}
	if b.Build == "" {
		return DevBuild, fmt.Errorf("%s: no build id", BuildFile)
	}
	return b.Build, nil
}

// LoadAssets reads the client bundle's manifest in dist (the
// directory `vite build` wrote, web/app/dist/client) and lists what a
// page links: the entry script, the styles of the entry and of every
// chunk it imports, and those chunks as module preloads. build is the
// bundle's build id; empty means the one the build recorded
// (ReadBuild).
func LoadAssets(dist, build string) (Assets, error) {
	if build == "" {
		var err error
		if build, err = ReadBuild(dist); err != nil {
			return Assets{Build: build}, err
		}
	}
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
