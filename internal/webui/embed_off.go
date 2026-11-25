// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build !embed_webui

// Default build: no embedded assets. The handler serves web/dist from
// disk, which is what the hermetic development loop needs (build the
// frontend, refresh, no restart) and what every existing test pins.
package webui

import "io/fs"

// embeddedDist is nil in the default build -- Mount falls back to the
// disk-mounted handler.
var embeddedDist fs.FS

// Embedded reports whether this binary carries the web UI inside it.
func Embedded() bool { return false }
