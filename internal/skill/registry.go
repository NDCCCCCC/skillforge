// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package skill

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/skillforge/skillforge/internal/embedder"
)

// ReloadDebounce is the quiet-period window during which consecutive
// fsnotify events for the same SKILL.md coalesce into a single reload.
// Tuned to 1s per the canonical fsnotify #553 debounce pattern (editors
// and the wizard save via temp-file + rename, which produces a burst of
// CREATE/WRITE/CHMOD events within milliseconds; the debouncer waits 1s
// after the last event before reloading).
const ReloadDebounce = 1 * time.Second

// ErrNoSnapshot is returned by Snapshot() if New() failed before the
// initial scan completed. Callers that need to fail-fast at startup can
// pattern-match on this; in normal flow New() either succeeds or fails
// outright (the server binary exits non-zero).
var ErrNoSnapshot = errors.New("skill registry: snapshot not initialized")

// Entry is the in-memory record of one Skill, stored in the Registry's
// atomic.Pointer snapshot. The Manifest is the parsed SKILL.md; DescVec is
// the cached embedding used by the Phase-5 router (D5-01); Version is a
// monotonic counter bumped on each successful reload — used by callers
// that want to detect "this Entry is older than another one I have".
type Entry struct {
	Manifest *Manifest
	DescVec  []float32
	Version  int64
}

// Registry is the runtime source of truth for loaded Skills.
//
// Concurrency model (D5-06 in-flight version pinning):
//   - Writers (Watch goroutine) build a fresh map and atomically swap it
//     into place with Store. In-flight readers that captured the previous
//     snapshot via Snapshot() keep their reference — they do NOT see the
//     reload until they next call Snapshot().
//   - Readers (every /api/chat handler, the router) call Snapshot() once
//     per request and treat the returned map as read-only.
//
// Watch loop pattern:
//   - fsnotify.NewWatcher() watches the skills DIRECTORY (not individual
//     SKILL.md files) — Pitfall 1 in RESEARCH.md notes that atomic
//     save+rename leaves a file-level watch dangling on the renamed-away
//     inode, while a dir-level watch fires reliably for every operation.
//   - events are filtered by HasSuffix(event.Name, "/SKILL.md") so the
//     CREATE/CHMOD bursts that fsnotify emits for the dir itself are
//     ignored.
//   - the 1s debouncer (ReloadDebounce) resets on every event; only a
//     full 1s of quiet triggers reconcileOne(path) (quick-260814-w4n:
//     reload-if-present, remove-if-gone, plus a snapshot existence
//     sweep).
//
// V12 ASVS symlink guard runs on every reloadOne (parseAndValidate
// encapsulates it). A symlink whose EvalSymlinks resolves outside the
// skills root is rejected; the old version is retained and the registered
// LoadError hook is invoked (D5-08 — never silent-drop).
type Registry struct {
	skillsDir string            // as-passed; rootAbs is the canonical absolute form
	rootAbs   string            // absolute resolved skills dir (for Lstat root if ever needed)
	embedder  embedder.Embedder // nil allowed; descriptions get nil DescVec
	snapshot  atomic.Pointer[map[string]*Entry] // immutable; copy-on-write
	watcher   *fsnotify.Watcher                  // owned by Registry; created + Add in New, closed in watchLoop exit
	debounce  struct {
		mu    sync.Mutex
		timer *time.Timer
	}
	loadError func(name string, err error) // D5-08 hook; nil = no-op
	// seq is the per-process monotonic counter backing nextVersion. It
	// replaces the prior time.Now().UnixNano() initial-version scheme so
	// that Entry.Version values stay well below 2^53 — the +1 reload bump
	// is now visible to JS consumers (the GET /api/skills polling client),
	// which could not distinguish a UnixNano±1 pair at double precision
	// (ULP at 1.7e18 ≈ 512). The counter is per-process: it resets on
	// restart, which is fine because the polling client's known-version
	// map also resets on page load.
	seq atomic.Int64
	// failed holds the last validation/parse error per skill name — the
	// D5-08 surface for the SC-2 polling reload toast. Copy-on-write
	// atomic like snapshot. A name present here overlays status:"failed"
	// onto its snapshot entry in the HTTP listing WITHOUT bumping the
	// entry's Version (the keep-old contract pinned by
	// TestRegistry_Reload_ValidationFailure_KeepsOld). Cleared on the
	// next successful reload of the same name.
	failed atomic.Pointer[map[string]string]
}

// New constructs a Registry by scanning skillsDir at startup, validating
// every skills/*/SKILL.md against the 7-field contract, and seeding the
// atomic.Pointer snapshot. The first validation error is fatal (matches
// Loader.Scan behavior — operators see a typo at startup, not in a prod
// log days later).
//
// emb may be nil — the embedder is called lazily on reload to populate
// DescVec; passing nil disables description-embedding (useful for tests
// and for server modes where routing is disabled).
//
// New DOES create the fsnotify watcher and Add the skills dir to it —
// but does NOT start the consuming goroutine (call Watch(ctx) for that).
// This ordering matters on Windows: any directory iteration (filepath.WalkDir,
// os.ReadDir) performed BEFORE fsnotify.Add silently invalidates the
// watcher's event subscription and no further events are delivered. We
// therefore Add first, then scan.
//
// The watcher is owned by the Registry and closed when Watch's goroutine
// exits (i.e., when ctx is cancelled).
func New(skillsDir string, emb embedder.Embedder) (*Registry, error) {
	root, err := filepath.Abs(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute path for %q: %w", skillsDir, err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("skills dir %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skills path %q is not a directory", root)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}
	if err := watcher.Add(root); err != nil {
		_ = watcher.Close()
		return nil, fmt.Errorf("watch %s: %w", root, err)
	}

	// Subscribe each direct subdirectory BEFORE startupScan (but strictly
	// AFTER watcher.Add(root) — see the Windows ordering note above).
	//
	// Why per-subdir Add: fsnotify is non-recursive on Linux (inotify
	// requires one watch descriptor per directory). Without this, a
	// SKILL.md edit inside skills/<name>/ never produces an event on
	// Linux — a gap that was only masked on Windows, where the
	// ReadDirectoryChangesW backend happens to deliver nested events.
	// Explicit Add makes both platforms behave identically. A failed Add
	// is logged and skipped (a racing dir delete must not kill startup).
	if entries, readErr := os.ReadDir(root); readErr != nil {
		slog.Warn("skill registry: read skills dir for subdir watches failed", "dir", root, "err", readErr)
	} else {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if addErr := watcher.Add(filepath.Join(root, e.Name())); addErr != nil {
				slog.Warn("skill registry: watch skill subdir failed (non-fatal)",
					"dir", e.Name(), "err", addErr)
			}
		}
	}

	r := &Registry{
		skillsDir: skillsDir,
		rootAbs:   root,
		embedder:  emb,
		watcher:   watcher,
	}
	if err := r.startupScan(); err != nil {
		_ = watcher.Close()
		return nil, err
	}
	return r, nil
}

// Snapshot returns the current immutable snapshot. The returned map
// MUST be treated as read-only — callers MUST NOT mutate it (mutating
// would race with the next reload's copy-on-write Store).
//
// Snapshot is lock-free: atomic.Pointer.Load is the only access on the
// read path. concurrent readers can each call Snapshot() and see a
// consistent view of the registry at that instant.
func (r *Registry) Snapshot() map[string]*Entry {
	snap := r.snapshot.Load()
	if snap == nil {
		return map[string]*Entry{}
	}
	return *snap
}

// nextVersion returns a process-unique, JS-safe (<< 2^53) monotonic
// version number for a freshly-loaded Entry. Used at startup-scan and
// the first reload of a previously-unseen skill. Subsequent reloads of
// an existing skill use prev.Version+1 (per-skill monotonic, also
// JS-safe from this counter base).
func (r *Registry) nextVersion() int64 {
	return r.seq.Add(1)
}

// Failed returns the current per-skill validation-error map (name →
// error message). The returned map is safe to read concurrently but
// MUST be treated as read-only (callers MUST NOT mutate it). Empty when
// no skill is in a failed-reload state.
func (r *Registry) Failed() map[string]string {
	m := r.failed.Load()
	if m == nil {
		return map[string]string{}
	}
	return *m
}

// setFailed records (copy-on-write) that `name` failed its most recent
// reload with msg. Called on the reloadOne error path; does NOT touch
// the snapshot or any Entry.Version (keep-old contract, D5-08).
func (r *Registry) setFailed(name, msg string) {
	for {
		old := r.failed.Load()
		var next map[string]string
		if old == nil {
			next = map[string]string{name: msg}
		} else {
			// Idempotent: if the message is unchanged, no swap needed.
			if cur, ok := (*old)[name]; ok && cur == msg {
				return
			}
			next = make(map[string]string, len(*old)+1)
			for k, v := range *old {
				next[k] = v
			}
			next[name] = msg
		}
		if r.failed.CompareAndSwap(old, &next) {
			return
		}
		// Lost the race against another writer — retry with the new base.
	}
}

// clearFailed removes (copy-on-write) any recorded failure for `name`.
// Called on the reloadOne success path so a skill that recovers no
// longer surfaces status:"failed" in the listing.
func (r *Registry) clearFailed(name string) {
	for {
		old := r.failed.Load()
		if old == nil {
			return
		}
		if _, ok := (*old)[name]; !ok {
			return // nothing to clear
		}
		next := make(map[string]string, len(*old)-1)
		for k, v := range *old {
			if k != name {
				next[k] = v
			}
		}
		if r.failed.CompareAndSwap(old, &next) {
			return
		}
	}
}

// Watch spawns the goroutine that consumes events from the watcher created
// in New(). The watcher and its directory subscription are owned by the
// Registry for its entire lifetime; Watch merely starts the consumer.
// The goroutine terminates when ctx is cancelled, at which point the
// watcher is closed.
//
// Watch returns nil on success (the goroutine is fire-and-forget). The
// caller is responsible for cancelling ctx to stop the consumer and
// release the watcher.
func (r *Registry) Watch(ctx context.Context) error {
	if r.watcher == nil {
		return fmt.Errorf("Watch: registry not initialized (call New first)")
	}
	go r.watchLoop(ctx, r.watcher)
	return nil
}

// Reload synchronously re-reads, re-validates, and re-applies the SKILL.md
// at path — the non-debounced twin of the Watch goroutine's reloadOne. On a
// validation/parse failure it records the error via the failed map (status
// overlay) and invokes the LoadError hook WITHOUT touching the snapshot or
// Version (keep-old contract); on success it bumps the Version and clears
// any prior failure.
//
// Reload is the public, deterministic reload trigger used by tests (and a
// future "reload now" admin path) that need to assert reload semantics
// without racing the 1s debounce + fsnotify delivery latency. It is safe
// to call concurrently with Watch — both route through the atomic.Pointer
// copy-on-write snapshot.
func (r *Registry) Reload(path string) {
	r.reloadOne(path)
}

// watchLoop is the goroutine body. It selects between ctx.Done, the
// fsnotify events channel, and the fsnotify errors channel. Errors are
// logged and the loop continues — a transient fsnotify error must not
// kill the watcher (T-5-04).
//
// Per-file vs dir-level events: on Linux/macOS, dir-level watching emits
// per-file events with the absolute file path (event.Name ends in
// "/SKILL.md"). On Windows, fsnotify uses ReadDirectoryChangesW, which
// emits dir-level WRITE/CHMOD events WITHOUT the file name — event.Name
// is the parent directory. We handle both shapes by deriving the
// candidate SKILL.md path two ways:
//   1. event.Name ends in SKILL.md → use it directly.
//   2. event.Name is a directory containing a SKILL.md → join them.
// If neither matches, the event is a CREATE/REMOVE on an unrelated
// file (or on the skills dir itself) and we ignore it.
func (r *Registry) watchLoop(ctx context.Context, watcher *fsnotify.Watcher) {
	defer watcher.Close()
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("skill registry: watchLoop panic", "panic", rec)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// Directory-level events (quick-260814-w4n). Before the
			// per-file resolveSkillPath filter, classify CREATE and
			// REMOVE/RENAME on skills/ subdirectories so that new skill
			// dirs are discovered and deleted ones are dropped:
			//
			//   - CREATE on a fresh direct subdirectory (cp -r / git
			//     checkout / editor mkdir): attach a per-dir watch (Linux
			//     inotify needs it to see later per-file events) AND
			//     schedule a reconcile for <dir>/SKILL.md. The reconcile
			//     runs after the 1s debounce — by then cp -r has finished,
			//     and its existence check closes the race where SKILL.md
			//     landed BEFORE our Add (no per-file event would ever
			//     arrive for it).
			//   - REMOVE/RENAME: Windows ReadDirectoryChangesW frequently
			//     reports a directory deletion as just the dir name (no
			//     per-file events for its contents), while Linux inotify
			//     reports the per-file REMOVE from the subdir watch. Both
			//     shapes are normalized to <dir>/SKILL.md and routed into
			//     the same debounce → reconcile. watcher.Add on an
			//     already-deleted dir returns an error — ignored.
			if event.Op&fsnotify.Create != 0 && r.isDirectSubdir(event.Name, true) {
				if err := watcher.Add(event.Name); err != nil {
					slog.Warn("skill registry: watch new skill subdir failed (non-fatal)",
						"dir", event.Name, "err", err)
				}
				r.scheduleReconcile(filepath.Join(event.Name, "SKILL.md"))
				continue
			}
			if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				if filepath.Base(event.Name) == "SKILL.md" {
					r.scheduleReconcile(event.Name)
					continue
				}
				// Strict path comparison only — the dir is already gone,
				// so Stat-based detection is impossible here.
				if r.isDirectSubdir(event.Name, false) {
					r.scheduleReconcile(filepath.Join(event.Name, "SKILL.md"))
					continue
				}
			}
			skillPath := resolveSkillPath(event.Name)
			if skillPath == "" {
				continue
			}
			r.scheduleReconcile(skillPath)
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			if err != nil {
				slog.Warn("skill registry: watcher channel error", "err", err)
			}
		}
	}
}

// resolveSkillPath maps a raw fsnotify event name to the SKILL.md path
// we want to reload. Returns "" if the event doesn't pertain to a Skill.
//
// Two cases are accepted:
//   - "C:/skills/compliance/SKILL.md" → "/skills/compliance/SKILL.md"
//     (Linux/macOS per-file event).
//   - "C:/skills/compliance" → "/skills/compliance/SKILL.md"
//     (Windows dir-level event where the file name is not in event.Name;
//     verified by a cheap Stat).
//
// Other events (CREATE on a sibling file, CHMOD on the skills root, etc.)
// return "" and the caller skips them.
func resolveSkillPath(eventName string) string {
	if eventName == "" {
		return ""
	}
	if filepath.Base(eventName) == "SKILL.md" {
		return eventName
	}
	candidate := filepath.Join(eventName, "SKILL.md")
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}

// scheduleReconcile arms (or re-arms) the 1s debounce timer for path. Every
// event resets the timer; reconcileOne only fires after 1s of quiet. This
// is the canonical fsnotify #553 pattern — editors save via temp-file
// + rename which produces a burst of CREATE/WRITE/CHMOD events within
// milliseconds, and we want a SINGLE reconcile after the editor finishes.
//
// NOTE: the debounce holds a single pending path (single timer). A later
// scheduleReconcile overwrites the pending path — the earlier path's
// event is NOT replayed. This is safe for reload semantics (any SKILL.md
// event for the same file converges to "re-read whatever is on disk")
// but would silently lose a REMOVE if it were the overwritten event. That
// loss is exactly what reconcileOne's snapshot existence sweep repairs.
func (r *Registry) scheduleReconcile(path string) {
	r.debounce.mu.Lock()
	defer r.debounce.mu.Unlock()
	if r.debounce.timer != nil {
		r.debounce.timer.Stop()
	}
	r.debounce.timer = time.AfterFunc(ReloadDebounce, func() {
		r.reconcileOne(path)
	})
}

// isDirectSubdir reports whether path names a child directory of the
// skills root (filepath.Dir(path) == rootAbs, strict equality — prevents
// nested paths and the root itself from matching). When statOK is true a
// Stat is required to confirm the path currently exists as a directory
// (used for CREATE events, where the dir is present); pass statOK=false
// for REMOVE/RENAME events where the path is already gone and only the
// path shape can be checked.
func (r *Registry) isDirectSubdir(path string, statOK bool) bool {
	if path == "" || filepath.Clean(filepath.Dir(path)) != r.rootAbs {
		return false
	}
	base := filepath.Base(path)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return false
	}
	if !statOK {
		return true
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// reconcileOne is the debounce callback: after the 1s quiet period it
// converges the registry to on-disk truth for path (quick-260814-w4n).
//
//   - SKILL.md still exists → reloadOne (register/reload, including lazy
//     embed). This is the normal single-file reload path, unchanged.
//   - SKILL.md is GONE → removeFromSnapshot. Crucially, the missing-file
//     branch must NOT fall through to reloadOne: parseAndValidate would
//     fail on lstat and record a setFailed entry — a phantom "failed"
//     skill that the GET /api/skills listing would surface forever. A
//     deleted skill is a removal, not a load failure.
//   - Finally, an O(n) existence sweep over the whole snapshot: every
//     registered entry whose <root>/<name>/SKILL.md no longer lstats is
//     removed. Why: the debounce is a single timer with a single pending
//     path, so in an interleaved burst ("rm -r skills/A" + "cp -r
//     skills/B" inside one 1s window) B's schedule overwrites A's pending
//     path and A's removal event is dropped. The sweep is the backstop
//     that makes removal converge regardless of which events were
//     delivered or overwritten. Cost is one lstat per registered skill,
//     at most once per debounce window — negligible at v1 scale (tens of
//     skills).
func (r *Registry) reconcileOne(path string) {
	if _, err := os.Lstat(path); err == nil {
		// File exists → normal reload path (parse/validate/embed/store).
		r.reloadOne(path)
	} else if name := skillNameFromPath(path); name != "" {
		// File gone and the path names a skill → drop it (no setFailed).
		r.removeFromSnapshot(name)
	}

	// Existence sweep (see comment above): catch removals whose events
	// were lost to the single-pending-path debounce, and orphans whose
	// deletion never produced an event at all.
	snap := r.Snapshot()
	for name := range snap {
		if _, err := os.Lstat(filepath.Join(r.rootAbs, name, "SKILL.md")); err == nil {
			continue
		}
		r.removeFromSnapshot(name)
	}
}

// removeFromSnapshot removes (copy-on-write) the entry for name, clearing
// any recorded failure so the HTTP listing's Failed() overlay does not
// surface a phantom "failed" entry for a skill that no longer exists on
// disk. No-op when name is not in the snapshot (idempotent).
func (r *Registry) removeFromSnapshot(name string) {
	for {
		old := r.snapshot.Load()
		if old == nil {
			return
		}
		if _, ok := (*old)[name]; !ok {
			return // not registered — nothing to remove
		}
		next := make(map[string]*Entry, len(*old)-1)
		for k, v := range *old {
			if k != name {
				next[k] = v
			}
		}
		if r.snapshot.CompareAndSwap(old, &next) {
			r.clearFailed(name)
			slog.Info("skill registry: removed skill (SKILL.md no longer on disk)", "skill", name)
			return
		}
		// Lost the race against another writer — retry with the new base.
	}
}

// reloadOne re-reads path, re-runs the V12 symlink guard, re-parses,
// re-validates, re-embeds the description, and atomically swaps the
// snapshot map (D5-06 copy-on-write). On ANY error the snapshot is
// left untouched and the registered LoadError hook is invoked (D5-08 —
// validation failure keeps the old version; never silent-drop).
//
// reloadOne is safe to call directly (no fsnotify required); the
// registry tests use it to assert reload semantics deterministically
// without racing against the goroutine + timer.
func (r *Registry) reloadOne(path string) {
	name := skillNameFromPath(path)
	if name == "" {
		// Path does not live at <root>/<skill>/SKILL.md — ignore. The
		// watcher already filtered by suffix, but defend against
		// direct callers (tests) anyway.
		if r.loadError != nil {
			r.loadError(path, fmt.Errorf("reloadOne: path %q is not a SKILL.md in a skill subdirectory", path))
		}
		return
	}
	m, tiers, err := parseAndValidate(path)
	if err != nil {
		// Record the failure for the HTTP listing's status:"failed"
		// overlay (SC-2 polling toast). The snapshot + Entry.Version are
		// left UNTOUCHED — the keep-old contract pinned by
		// TestRegistry_Reload_ValidationFailure_KeepsOld / _SymlinkEscape_Rejected.
		r.setFailed(name, err.Error())
		if r.loadError != nil {
			r.loadError(name, err)
		}
		return
	}
	m.resolvedTiers = tiers
	// Re-embed the description. Embedder errors are NON-FATAL: the
	// entry still becomes routable on its previous (or nil) DescVec
	// until the next successful reload. Rationale: routing is a UX
	// feature, not a security boundary — a temporarily-stale vector
	// is preferable to dropping a working Skill from the registry
	// (D5-08 forbids silent-drop, and the cost of an embedder hiccup
	// should not be "the Skill vanishes").
	var descVec []float32
	if r.embedder != nil {
		if v, embedErr := r.embedder.Embed(context.Background(), m.Description); embedErr == nil {
			descVec = v
		} else {
			slog.Warn("skill registry: embed description failed (non-fatal; reusing prior vec if any)",
				"skill", name, "err", embedErr)
		}
	}
	m.SetDescriptionVec(descVec)

	// Copy-on-write (D5-06): clone the current snapshot, install the
	// new entry, atomically Store the new map pointer. atomic.Pointer
	// guarantees concurrent readers see either the old or new map,
	// never a torn intermediate.
	oldSnap := r.snapshot.Load()
	var next map[string]*Entry
	if oldSnap == nil {
		next = make(map[string]*Entry, 1)
	} else {
		next = make(map[string]*Entry, len(*oldSnap)+1)
		for k, v := range *oldSnap {
			next[k] = v
		}
	}
	var newVersion int64
	if prev, ok := next[name]; ok && prev != nil {
		newVersion = prev.Version + 1
	} else {
		newVersion = r.nextVersion()
	}
	next[name] = &Entry{
		Manifest: m,
		DescVec:  descVec,
		Version:  newVersion,
	}
	r.snapshot.Store(&next)
	// A successful reload clears any prior failure state for this skill
	// so the listing flips back to status:"ok".
	r.clearFailed(name)
}

// startupScan is the synchronous seed of the snapshot at New() time.
// It mirrors Loader.Scan but loads results into the atomic.Pointer
// snapshot (not the legacy cached []*Manifest field).
func (r *Registry) startupScan() error {
	out := make(map[string]*Entry)
	walkErr := filepath.WalkDir(r.rootAbs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) != "SKILL.md" {
			return nil
		}
		rel, err := filepath.Rel(r.rootAbs, filepath.Dir(path))
		if err != nil || rel == "." || strings.Contains(rel, string(filepath.Separator)) {
			return nil
		}
		name := filepath.Base(filepath.Dir(path))
		m, tiers, err := parseAndValidate(path)
		if err != nil {
			return err
		}
		m.resolvedTiers = tiers
		var descVec []float32
		if r.embedder != nil {
			if v, embedErr := r.embedder.Embed(context.Background(), m.Description); embedErr == nil {
				descVec = v
			}
		}
		m.SetDescriptionVec(descVec)
		out[name] = &Entry{
			Manifest: m,
			DescVec:  descVec,
			Version:  r.nextVersion(),
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	r.snapshot.Store(&out)
	return nil
}

// SetLoadErrorHook registers a callback invoked whenever reloadOne
// rejects a SKILL.md (parse failure, validation failure, V12 symlink
// escape). The hook receives the skill name (parent directory basename)
// and the underlying error. This is the D5-08 surface for the SC-2 UI
// toast — the front-end renders the error without the watcher having
// to know anything about SSE/HTTP.
//
// Calling with nil clears the hook (errors are then ignored).
func (r *Registry) SetLoadErrorHook(fn func(name string, err error)) {
	r.loadError = fn
}

// skillNameFromPath derives the Skill name from a SKILL.md path. The
// name is the parent directory's basename (e.g., "compliance" for
// /skills/compliance/SKILL.md). Returns "" if the path is not of the
// expected shape (e.g., a loose SKILL.md in the root, or path does not
// end in SKILL.md).
func skillNameFromPath(path string) string {
	if filepath.Base(path) != "SKILL.md" {
		return ""
	}
	parent := filepath.Dir(path)
	name := filepath.Base(parent)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return ""
	}
	return name
}