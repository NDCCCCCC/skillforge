// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Phase 58+ -- the fs.FS serving path, used by the single-binary build
// (`-tags embed_webui`).
//
// It reproduces Handler()'s nginx try_files semantics EXACTLY, but reads
// from an fs.FS instead of the disk:
//  1. no extension and not found -> index.html (SPA fallback)
//  2. has an extension and not found -> 404 (never HTML for a missing .js)
//  3. /api and /api/ -> 404 (an unimplemented endpoint is not a route)
//  4. a directory -> index.html (never a listing)
//
// Handler() (disk) stays the default and is untouched, so every existing
// test and the hermetic dev loop keep their exact behaviour.
package webui

import (
	"io/fs"
	"log"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

// HandlerEmbedded returns the http.Handler for an embedded asset tree.
// A nil tree degrades to the "dist missing" 503 with the same actionable
// body as the disk handler, so a mis-packaged binary is diagnosable
// instead of silently blank.
func HandlerEmbedded(distFS fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "" {
			p = "/"
		}

		// Rule 3 first: an unregistered /api path must never be mistaken
		// for a frontend route (same ordering rationale as Handler).
		if p == "/api" || strings.HasPrefix(p, "/api/") {
			http.NotFound(w, r)
			return
		}

		if distFS == nil {
			serveMissingDist(w)
			return
		}

		indexBytes, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			serveMissingDist(w)
			return
		}

		// Root-anchored clean: /../x collapses to /x, so no ".." survives.
		rel := strings.TrimPrefix(path.Clean("/"+p), "/")
		if rel == "" || rel == "." {
			serveIndexBytes(w, indexBytes)
			return
		}

		info, statErr := fs.Stat(distFS, rel)
		if statErr != nil || info.IsDir() {
			// Rule 4 (directory) and rule 1/2 (missing) both land here.
			if statErr != nil && hasExtension(rel) {
				http.NotFound(w, r)
				return
			}
			serveIndexBytes(w, indexBytes)
			return
		}

		data, readErr := fs.ReadFile(distFS, rel)
		if readErr != nil {
			http.NotFound(w, r)
			return
		}
		// Content-Type from the extension: mime.TypeByExtension covers the
		// Vite output set (.js/.css/.svg/.woff2/.json). An unknown
		// extension falls back to sniffing, which net/http does itself.
		if ct := mime.TypeByExtension(path.Ext(rel)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		// Vite assets carry a content hash, so they are safe to cache hard.
		// index.html is NOT served through this branch (see above), which
		// is what keeps the shell revalidating after a version bump.
		if strings.HasPrefix(rel, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if mod := info.ModTime(); !mod.IsZero() && !mod.Equal(time.Time{}) {
			w.Header().Set("Last-Modified", mod.UTC().Format(http.TimeFormat))
		}
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck // the client may already be gone
		w.Write(data)
	})
}

// serveIndexBytes writes the SPA shell from memory. Same Cache-Control
// posture as serveIndex: the shell must revalidate, otherwise a stale
// shell references deleted hash assets and the page goes blank.
func serveIndexBytes(w http.ResponseWriter, indexBytes []byte) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(indexBytes)))
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // the client may already be gone
	w.Write(indexBytes)
}

// logMountSource prints which asset source this process is serving, so an
// operator can tell a single-binary deployment from a disk-mounted one at
// a glance (the two have very different failure modes).
func logMountSource() {
	if Embedded() {
		log.Printf("[OK] webui: serving the EMBEDDED web UI (single-binary build; no web/dist needed)")
		return
	}
	log.Printf("[INFO] webui: no embedded assets in this binary (default build)")
}
