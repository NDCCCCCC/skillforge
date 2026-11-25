// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build embed_webui

// Phase 58+ release packaging -- the SINGLE-BINARY build.
//
// `go build -tags embed_webui ./cmd/server` compiles the Vite output
// (web/dist) INTO the executable, so the shipped artefact is one file
// that serves the UI and the API from the same origin with no sibling
// directory to lose. The default build (no tag) keeps the disk-mounted
// behaviour the hermetic dev loop relies on ("build the frontend, refresh
// the page, no restart").
//
// The dist tree is copied to internal/webui/dist by the release recipe
// BEFORE the build; the directory is kept in git with a .gitkeep so this
// directive resolves on a clean checkout.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embeddedDistFS embed.FS

// embeddedDist is the sub-tree rooted at the copied dist directory, so the
// handler sees the same layout it would see on disk (index.html at "/").
var embeddedDist fs.FS = mustSub(embeddedDistFS, "dist")

// Embedded reports whether this binary carries the web UI inside it.
func Embedded() bool { return true }

func mustSub(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		// A mis-shaped embed is a build-time packaging bug; surfacing it as
		// a nil FS degrades to the disk path with a loud WARN instead of a
		// panic at startup.
		return nil
	}
	return sub
}
