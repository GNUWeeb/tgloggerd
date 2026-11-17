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
