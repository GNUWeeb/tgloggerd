// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package render

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// A manifest as vite build writes it: the entry imports a shared chunk
// twice over (directly and through a route chunk), dynamic imports are
// not preloaded.
const manifest = `{
  "src/entry-client.tsx": {
    "file": "assets/entry-client-AbC1.js",
    "src": "src/entry-client.tsx",
    "isEntry": true,
    "imports": ["_vendor-X1.js", "_route-Y2.js"],
    "dynamicImports": ["src/lazy.tsx"],
    "css": ["assets/entry-client-D3.css"]
  },
  "_vendor-X1.js": {"file": "assets/vendor-X1.js"},
  "_route-Y2.js": {"file": "assets/route-Y2.js", "imports": ["_vendor-X1.js"], "css": ["assets/route-Z9.css"]},
  "src/lazy.tsx": {"file": "assets/lazy-L.js", "isDynamicEntry": true}
}`

func TestLoadAssets(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".vite"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".vite", "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := LoadAssets(dir, "b1")
	if err != nil {
		t.Fatal(err)
	}
	want := Assets{
		Build:   "b1",
		CSS:     []string{"/app/assets/entry-client-D3.css", "/app/assets/route-Z9.css"},
		Preload: []string{"/app/assets/vendor-X1.js", "/app/assets/route-Y2.js"},
		Entry:   "/app/assets/entry-client-AbC1.js",
	}
	if !reflect.DeepEqual(a, want) {
		t.Errorf("got %+v\nwant %+v", a, want)
	}
}

func TestLoadAssetsMissing(t *testing.T) {
	t.Parallel()
	a, err := LoadAssets(t.TempDir(), "b1")
	if err == nil || a.Build != "b1" || a.Entry != "" {
		t.Errorf("no manifest: %+v, %v", a, err)
	}
	if _, err := parseManifest([]byte(`{"other.js":{"file":"x.js"}}`), "b"); err == nil {
		t.Error("a manifest without the entry is accepted")
	}
}

// writeBundle writes a client bundle's manifest into a directory of
// the test's own, and build.json with the given content unless empty.
func writeBundle(t *testing.T, buildJSON string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".vite"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".vite", "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if buildJSON != "" {
		if err := os.WriteFile(filepath.Join(dir, BuildFile), []byte(buildJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// The build id comes from build.json (as web/app/scripts/build.ts
// writes it) unless one is configured.
func TestLoadAssetsBuildFile(t *testing.T) {
	t.Parallel()
	dir := writeBundle(t, `{"build":"207b3e9e0d73"}`+"\n")
	a, err := LoadAssets(dir, "")
	if err != nil || a.Build != "207b3e9e0d73" || a.Entry == "" {
		t.Errorf("recorded id: %+v, %v", a, err)
	}
	if a, err := LoadAssets(dir, "configured"); err != nil || a.Build != "configured" {
		t.Errorf("configured id: %+v, %v", a, err)
	}

	// A bundle built without the build script records none.
	if a, err := LoadAssets(writeBundle(t, ""), ""); err != nil || a.Build != DevBuild || a.Entry == "" {
		t.Errorf("no build.json: %+v, %v", a, err)
	}
	if b, err := ReadBuild(t.TempDir()); err != nil || b != DevBuild {
		t.Errorf("empty directory: %q, %v", b, err)
	}

	for _, bad := range []string{`{"build":""}`, `{}`, `"x"`, `{"build":`} {
		a, err := LoadAssets(writeBundle(t, bad), "")
		if err == nil || a.Build != DevBuild || a.Entry != "" {
			t.Errorf("build.json %s: %+v, %v", bad, a, err)
		}
	}
}
