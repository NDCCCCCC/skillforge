// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Tests for the embedded (single-binary) asset path.
//
// HandlerEmbedded must reproduce Handler()'s nginx try_files semantics
// EXACTLY, because the same nginx semantics are the product contract
// (package doc): SPA fallback for extension-less paths, 404 (never HTML)
// for a missing asset, 404 for an unregistered /api path, and never a
// directory listing.
//
// These tests build an in-memory fs.FS, so they run in the DEFAULT build
// too (no `embed_webui` tag) -- the embedded path is not allowed to be
// untested in CI just because CI does not build the release tag.

package webui

import (
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

// embeddedFS builds the same fixture shape newDist() writes on disk, but
// as an fs.FS.
func embeddedFS(t *testing.T) fs.FS {
	t.Helper()
	return fstest.MapFS{
		"index.html":      {Data: []byte(indexHTML)},
		"assets/app.js":   {Data: []byte(assetJS)},
		"assets/app.css":  {Data: []byte("body{}")},
		"favicon.svg":     {Data: []byte("<svg/>")},
		"assets/only.bin": {Data: []byte{0x00, 0x01, 0x02}},
		"robots.txt":      {Data: []byte("User-agent: *\n")},
	}
}

// TestEmbeddedHandlerServesIndexAtRoot is the baseline: the SPA shell
// comes out of the embedded tree with the right content type.
func TestEmbeddedHandlerServesIndexAtRoot(t *testing.T) {
	h := HandlerEmbedded(embeddedFS(t))
	rec := do(h, http.MethodGet, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != indexHTML {
		t.Fatalf("GET / body = %q, want the index.html shell", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("GET / Content-Type = %q, want text/html", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Fatalf("GET / Cache-Control = %q, want no-cache (a stale shell references deleted hash assets)", cc)
	}
}

// TestEmbeddedHandlerServesAsset: a real asset is served verbatim with a
// content type derived from its extension.
func TestEmbeddedHandlerServesAsset(t *testing.T) {
	h := HandlerEmbedded(embeddedFS(t))
	rec := do(h, http.MethodGet, "/assets/app.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /assets/app.js status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != assetJS {
		t.Fatalf("GET /assets/app.js body = %q, want the asset content", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("GET /assets/app.js Content-Type = %q, want a javascript type", ct)
	}
}

// TestEmbeddedHandlerSPAFallback: an extension-less path that is not a
// real route must return the shell, not a 404.
func TestEmbeddedHandlerSPAFallback(t *testing.T) {
	h := HandlerEmbedded(embeddedFS(t))
	for _, p := range []string{"/dashboard", "/skills/simple-qa", "/admin"} {
		rec := do(h, http.MethodGet, p)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200 (SPA fallback)", p, rec.Code)
		}
		if got := rec.Body.String(); got != indexHTML {
			t.Fatalf("GET %s body = %q, want the index.html shell", p, got)
		}
	}
}

// TestEmbeddedHandlerMissingAssetReturns404: a MISSING asset with an
// extension must be 404 -- never HTML. Returning the shell here is the
// classic bug (the browser reports "Unexpected token '<'").
func TestEmbeddedHandlerMissingAssetReturns404(t *testing.T) {
	h := HandlerEmbedded(embeddedFS(t))
	for _, p := range []string{"/assets/missing.js", "/assets/app.css.map", "/nope.png"} {
		rec := do(h, http.MethodGet, p)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404", p, rec.Code)
		}
		if body := rec.Body.String(); strings.Contains(body, "<!doctype html") {
			t.Fatalf("GET %s returned HTML for a missing asset", p)
		}
	}
}

// TestEmbeddedHandlerAPIPrefixNeverFallsBackToHTML: an unregistered
// /api path must be 404 -- it is a missing endpoint, not a frontend
// route.
func TestEmbeddedHandlerAPIPrefixNeverFallsBackToHTML(t *testing.T) {
	h := HandlerEmbedded(embeddedFS(t))
	for _, p := range []string{"/api", "/api/nope", "/api/admin/ingest/jobs"} {
		rec := do(h, http.MethodGet, p)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404", p, rec.Code)
		}
	}
}

// TestEmbeddedHandlerNeverListsDirectories: a directory path must return
// the shell, never a listing.
func TestEmbeddedHandlerNeverListsDirectories(t *testing.T) {
	h := HandlerEmbedded(embeddedFS(t))
	for _, p := range []string{"/assets", "/assets/"} {
		rec := do(h, http.MethodGet, p)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", p, rec.Code)
		}
		if got := rec.Body.String(); got != indexHTML {
			t.Fatalf("GET %s body = %q, want the index.html shell", p, got)
		}
		if strings.Contains(rec.Body.String(), "Index of /") {
			t.Fatalf("GET %s produced a directory listing", p)
		}
	}
}

// TestEmbeddedHandlerRejectsTraversal: a traversal attempt must never
// escape the embedded tree. The embedded tree has no repository files,
// so the assertion is that nothing outside it is served.
func TestEmbeddedHandlerRejectsTraversal(t *testing.T) {
	h := HandlerEmbedded(embeddedFS(t))
	for _, p := range []string{"/../go.mod", "/..%2f..%2fgo.mod", "/assets/../../go.mod"} {
		rec := do(h, http.MethodGet, p)
		if strings.Contains(rec.Body.String(), goModMarker) {
			t.Fatalf("GET %s leaked content from outside the embedded tree", p)
		}
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 404 or 200", p, rec.Code)
		}
	}
}

// TestEmbeddedHandlerMissingTreeDegradesTo503: a mis-packaged binary
// (nil tree, or a tree without index.html) must answer 503 with an
// actionable body, not a blank page and not a panic.
func TestEmbeddedHandlerMissingTreeDegradesTo503(t *testing.T) {
	for name, tree := range map[string]fs.FS{
		"nil tree":       nil,
		"tree w/o shell": fstest.MapFS{"assets/app.js": {Data: []byte(assetJS)}},
	} {
		h := HandlerEmbedded(tree)
		rec := do(h, http.MethodGet, "/")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: GET / status = %d, want 503", name, rec.Code)
		}
		if body := rec.Body.String(); !strings.Contains(body, "npm run build") {
			t.Fatalf("%s: GET / body = %q, want the actionable build hint", name, body)
		}
	}
}

// TestEmbeddedHandlerContentLengthMatchesBody: the response header must
// describe the actual body (the disk path gets this from
// http.FileServer; the embedded path must do it explicitly).
func TestEmbeddedHandlerContentLengthMatchesBody(t *testing.T) {
	h := HandlerEmbedded(embeddedFS(t))
	for _, p := range []string{"/", "/assets/app.js", "/assets/only.bin"} {
		rec := do(h, http.MethodGet, p)
		cl := rec.Header().Get("Content-Length")
		if cl == "" {
			t.Fatalf("GET %s has no Content-Length", p)
		}
		want, err := strconv.Atoi(cl)
		if err != nil {
			t.Fatalf("GET %s Content-Length = %q, not a number", p, cl)
		}
		if want != rec.Body.Len() {
			t.Fatalf("GET %s Content-Length = %d but the body is %d bytes", p, want, rec.Body.Len())
		}
	}
}

// TestEmbeddedFlagMatchesBuildTag is the wiring lock: the default build
// must report Embedded()==false, and the release build must report true.
// Only the default side is asserted here (the release side is asserted
// by the tagged build's own smoke test).
func TestEmbeddedFlagMatchesBuildTag(t *testing.T) {
	if Embedded() {
		// This binary was built with -tags embed_webui; then the embedded
		// tree must actually carry a shell, otherwise the flag lies.
		if embeddedDist == nil {
			t.Fatal("Embedded() is true but embeddedDist is nil — the embedded build has no assets")
		}
		if _, err := fs.Stat(embeddedDist, "index.html"); err != nil {
			t.Fatalf("Embedded() is true but the embedded tree has no index.html: %v", err)
		}
		return
	}
	if embeddedDist != nil {
		t.Fatal("Embedded() is false but embeddedDist is non-nil — the default build must not carry assets")
	}
}
