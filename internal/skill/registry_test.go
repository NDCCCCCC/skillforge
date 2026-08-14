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
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubEmbedder is the test double for embedder.Embedder. It returns a
// fixed-length vector derived deterministically from the input text so
// tests can assert the same description produces the same vector (and a
// different description produces a different vector).
type stubEmbedder struct {
	dim  int
	calls atomic.Int64
}

func (s *stubEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	s.calls.Add(1)
	v := make([]float32, s.dim)
	var sum float32
	for _, c := range text {
		sum += float32(c) / 255.0
	}
	for i := range v {
		v[i] = sum
	}
	return v, nil
}

func (s *stubEmbedder) Dimensions() int { return s.dim }

// writeRegistrySkill writes a valid 7-field SKILL.md for the Registry tests.
func writeRegistrySkill(t *testing.T, dir string) string {
	t.Helper()
	return writeSkillFile(t, dir, nil)
}

// writeRegistryBrokenSkill writes a SKILL.md missing `temperature`, so the
// 7-field validator rejects it (SYSK-08). Used to assert reload-keeps-old.
func writeRegistryBrokenSkill(t *testing.T, dir string) {
	t.Helper()
	fm := `description: Broken test.
agent: ChatModelAgent
reads: ["authority/**"]
writes: []
steps: "1. Read tier"
guardrails:
  prohibitions: ["p1", "p2", "p3"]
  required_refusal_phrases: []
  tier_limits: {}
`
	writeCustomSkill(t, dir, fm)
}

func TestRegistry_New_SeedsSnapshot(t *testing.T) {
	// T1 — New(skillsDir, embedder) scans the directory at startup and
	// seeds the atomic.Pointer snapshot. Concurrent readers see the
	// initial snapshot immediately, with no lock.
	tmp := t.TempDir()
	writeRegistrySkill(t, filepath.Join(tmp, "compliance"))
	writeRegistrySkill(t, filepath.Join(tmp, "qa"))

	emb := &stubEmbedder{dim: 4}
	r, err := New(tmp, emb)
	require.NoError(t, err)
	require.NotNil(t, r)

	snap := r.Snapshot()
	require.Len(t, snap, 2)
	assert.Contains(t, snap, "compliance")
	assert.Contains(t, snap, "qa")

	entry := snap["compliance"]
	require.NotNil(t, entry)
	assert.NotNil(t, entry.Manifest)
	assert.Equal(t, "ChatModelAgent", entry.Manifest.Agent)
	assert.Equal(t, []string{"权威"}, entry.Manifest.ResolvedTiers())
	require.Len(t, entry.DescVec, 4, "DescriptionVec must be populated by the startup embed")
	assert.Greater(t, entry.Version, int64(0))
}

func TestRegistry_Snapshot_Atomic(t *testing.T) {
	// T2 — atomic.Pointer snapshot guarantees concurrent readers see a
	// consistent view (no torn reads on the map). This is the D5-06
	// in-flight version-pinning invariant: a reader that captured the
	// snapshot once at run-start keeps using it; a reload that swaps
	// the snapshot is invisible to that reader.
	tmp := t.TempDir()
	writeRegistrySkill(t, filepath.Join(tmp, "compliance"))

	emb := &stubEmbedder{dim: 4}
	r, err := New(tmp, emb)
	require.NoError(t, err)

	// Capture once and store it locally — readers MUST treat this as
	// the immutable in-flight snapshot.
	pinned := r.Snapshot()
	require.Len(t, pinned, 1)
	pinnedVersion := pinned["compliance"].Version

	// Mutate the on-disk Skill and trigger a reload synchronously
	// (calling reloadOne directly is fine — the public Watch path is
	// tested in TestRegistry_Reload_Debounce).
	skillPath := filepath.Join(tmp, "compliance", "SKILL.md")
	require.NoError(t, os.WriteFile(skillPath,
		[]byte("---\n"+
			"description: Trigger phrase test — bumped.\n"+
			"agent: ChatModelAgent\n"+
			"reads: [\"authority/**\"]\n"+
			"writes: []\n"+
			"steps: |\n"+
			"  1. Read tier chunks\n"+
			"  2. Compose verdict\n"+
			"  3. Refuse on zero chunks\n"+
			"guardrails:\n"+
			"  prohibitions:\n"+
			"    - \"Never cite clauses not retrieved\"\n"+
			"    - \"Never paraphrase critical clauses\"\n"+
			"    - \"Never modify authority tier content\"\n"+
			"  required_refusal_phrases:\n"+
			"    - \"no_authorized_source\"\n"+
			"  tier_limits:\n"+
			"    authority: [\"read-only\"]\n"+
			"temperature: 0.0\n"+
			"---\n\n# Body\n"),
		0o644))

	// Reload directly (no goroutine race).
	r.reloadOne(skillPath)

	// The pinned snapshot MUST be unchanged (D5-06 in-flight pinning).
	assert.Equal(t, pinnedVersion, pinned["compliance"].Version,
		"the pinned snapshot must not mutate after a reload")
	assert.Equal(t, pinned["compliance"].Manifest.Description, "Test Skill. Trigger phrase test.",
		"sanity: the pinned snapshot was loaded with the canonical fixture description")
	assert.Equal(t, "Test Skill. Trigger phrase test.", pinned["compliance"].Manifest.Description,
		"the pinned snapshot's manifest must reflect the pre-reload description")

	// Snapshot() AFTER the reload returns a NEW map; the version bumped.
	fresh := r.Snapshot()
	assert.Greater(t, fresh["compliance"].Version, pinnedVersion,
		"a new Snapshot() call after reload must see the bumped version")
	assert.Equal(t, "Trigger phrase test — bumped.", fresh["compliance"].Manifest.Description)
}

func TestRegistry_Reload_Debounce(t *testing.T) {
	// T3 — the 1s debounce coalesces multiple fsnotify writes within the
	// window into a single reloadOne call. Three writes in <1s result
	// in ONE reload (not three). This is the canonical fsnotify #553
	// debounce pattern.
	if testing.Short() {
		t.Skip("debounce test relies on real time; skip in -short")
	}
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "compliance")
	writeRegistrySkill(t, skillDir)

	emb := &stubEmbedder{dim: 4}
	r, err := New(tmp, emb)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, r.Watch(ctx))

	skillPath := filepath.Join(skillDir, "SKILL.md")
	// Three rapid writes inside the 1s debounce window.
	for i := 0; i < 3; i++ {
		require.NoError(t, os.WriteFile(skillPath, []byte("---\n"+
			"description: bump-"+string(rune('a'+i))+"\n"+
			"agent: ChatModelAgent\n"+
			"reads: [\"authority/**\"]\n"+
			"writes: []\n"+
			"steps: |\n"+
			"  1. step\n"+
			"  2. step\n"+
			"  3. step\n"+
			"guardrails:\n"+
			"  prohibitions:\n"+
			"    - \"p1\"\n"+
			"    - \"p2\"\n"+
			"    - \"p3\"\n"+
			"  required_refusal_phrases: []\n"+
			"  tier_limits: {}\n"+
			"temperature: 0.0\n"+
			"---\n\n# Body\n"), 0o644))
		// Tight loop; each WriteFile fires a fsnotify CREATE/WRITE/CHMOD
		// event; the debounce timer keeps resetting.
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for the debounce window to expire + a small buffer for the
	// AfterFunc goroutine to fire and reloadOne to run.
	time.Sleep(1500 * time.Millisecond)

	snap := r.Snapshot()
	require.Len(t, snap, 1)
	// Final description must be the LAST write (bump-c), proving the
	// debouncer fired exactly once after the 1s quiet period.
	assert.Equal(t, "bump-c", snap["compliance"].Manifest.Description,
		"after 3 writes inside 1s, the snapshot should reflect the LAST write (bump-c)")
}

func TestRegistry_Reload_ValidationFailure_KeepsOld(t *testing.T) {
	// T4 — D5-08 invariant: a SKILL.md that fails the 7-field validator
	// MUST NOT mutate the snapshot. The OLD version is retained AND an
	// error is emitted via the registered LoadError hook.
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "compliance")
	writeRegistrySkill(t, skillDir)

	emb := &stubEmbedder{dim: 4}
	r, err := New(tmp, emb)
	require.NoError(t, err)

	// Capture the good version's manifest description.
	goodDesc := r.Snapshot()["compliance"].Manifest.Description
	goodVersion := r.Snapshot()["compliance"].Version

	// Capture load errors via the hook.
	var loadErrName atomic.Value // string
	var loadErr atomic.Value    // error
	r.SetLoadErrorHook(func(name string, err error) {
		loadErrName.Store(name)
		loadErr.Store(err)
	})

	// Write a SKILL.md missing the required `temperature` field.
	writeRegistryBrokenSkill(t, skillDir)

	skillPath := filepath.Join(skillDir, "SKILL.md")
	r.reloadOne(skillPath)

	// Snapshot unchanged: same description, same version, same DescVec
	// pointer (the broken write did not bump the version).
	snap := r.Snapshot()
	require.Len(t, snap, 1)
	assert.Equal(t, goodDesc, snap["compliance"].Manifest.Description,
		"validation-failure reload MUST NOT modify the snapshot's manifest")
	assert.Equal(t, goodVersion, snap["compliance"].Version,
		"validation-failure reload MUST NOT bump the snapshot's version")

	// LoadError hook called with the skill name + the underlying error.
	gotName, _ := loadErrName.Load().(string)
	gotErr, _ := loadErr.Load().(error)
	assert.Equal(t, "compliance", gotName,
		"LoadError hook should be called with the skill name derived from the parent dir")
	require.Error(t, gotErr, "LoadError hook should be called with the validation error")
	assert.Contains(t, gotErr.Error(), "temperature",
		"LoadError should propagate the underlying validator message (SYSK-08)")
}

func TestRegistry_Reload_SymlinkEscape_Rejected(t *testing.T) {
	// T6 — the V12 ASVS symlink guard re-runs on every reload. A SKILL.md
	// replaced by a symlink-escape (target outside skills root) is rejected
	// and the old version is retained.
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires Developer Mode / admin on Windows; " +
			"the registry's symlink-rejection code is still compiled in (run on linux/macOS for full coverage)")
	}
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "compliance")
	writeRegistrySkill(t, skillDir)

	emb := &stubEmbedder{dim: 4}
	r, err := New(tmp, emb)
	require.NoError(t, err)

	// Capture the good state.
	goodDesc := r.Snapshot()["compliance"].Manifest.Description
	goodVersion := r.Snapshot()["compliance"].Version

	var hookCalled atomic.Bool
	r.SetLoadErrorHook(func(_ string, _ error) {
		hookCalled.Store(true)
	})

	// Replace SKILL.md with a symlink to an outside file.
	require.NoError(t, os.Remove(filepath.Join(skillDir, "SKILL.md")))
	outside := t.TempDir()
	outsideSkill := filepath.Join(outside, "SKILL.md")
	require.NoError(t, os.WriteFile(outsideSkill, []byte("---\n"+
		"description: hostile takeover\n"+
		"agent: ChatModelAgent\n"+
		"reads: [\"authority/**\"]\n"+
		"writes: []\n"+
		"steps: |\n"+
		"  1. step\n"+
		"  2. step\n"+
		"  3. step\n"+
		"guardrails:\n"+
		"  prohibitions:\n"+
		"    - \"p1\"\n"+
		"    - \"p2\"\n"+
		"    - \"p3\"\n"+
		"  required_refusal_phrases: []\n"+
		"  tier_limits: {}\n"+
		"temperature: 0.0\n"+
		"---\n\n# Body\n"), 0o644))
	require.NoError(t, os.Symlink(outsideSkill, filepath.Join(skillDir, "SKILL.md")))

	skillPath := filepath.Join(skillDir, "SKILL.md")
	r.reloadOne(skillPath)

	// Snapshot unchanged: symlink-escape was rejected at parse time.
	snap := r.Snapshot()
	require.Len(t, snap, 1)
	assert.Equal(t, goodDesc, snap["compliance"].Manifest.Description,
		"symlink-escape reload MUST NOT modify the snapshot")
	assert.Equal(t, goodVersion, snap["compliance"].Version,
		"symlink-escape reload MUST NOT bump the snapshot's version")
	assert.True(t, hookCalled.Load(), "LoadError hook MUST be invoked on symlink-escape (D5-08)")
}

func TestRegistry_Snapshot_ParallelReaders(t *testing.T) {
	// T7 — concurrent readers each get a consistent view. Each reader
	// dereferences the same snapshot pointer; no torn or nil map. This
	// is the lock-free read path that D5-06 in-flight runs rely on.
	tmp := t.TempDir()
	writeRegistrySkill(t, filepath.Join(tmp, "compliance"))
	writeRegistrySkill(t, filepath.Join(tmp, "qa"))

	emb := &stubEmbedder{dim: 4}
	r, err := New(tmp, emb)
	require.NoError(t, err)

	const N = 64
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			snap := r.Snapshot()
			assert.Len(t, snap, 2)
			_, hasCompliance := snap["compliance"]
			_, hasQA := snap["qa"]
			assert.True(t, hasCompliance)
			assert.True(t, hasQA)
		}()
	}
	wg.Wait()
}

func TestManifest_DescriptionVec_NotSerialized(t *testing.T) {
	// T8 — Manifest.DescriptionVec is in-memory only (json:"-"). A JSON
	// round-trip must NOT carry the vector on the wire or in stored
	// forms. The vector is a cached computation, not part of the SKILL.md
	// contract.
	m := &Manifest{Description: "x", Agent: "ChatModelAgent"}
	m.SetDescriptionVec([]float32{1.0, 2.0, 3.0})
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	// The serialized JSON must not contain the vec field.
	assert.NotContains(t, string(raw), "DescriptionVec")
	assert.NotContains(t, string(raw), "1.0")
	assert.NotContains(t, string(raw), "2.0")
	assert.NotContains(t, string(raw), "3.0")

	var out Manifest
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Nil(t, out.DescriptionVec, "DescriptionVec must be nil after JSON round-trip")
}

func TestRegistry_FailedMap_SurfacesValidationError(t *testing.T) {
	// SC-2 polling toast (§2): a reload that fails the 7-field validator
	// records the error in the failed map (the status:"failed" overlay
	// surface) WITHOUT bumping Version; a subsequent successful reload
	// clears it. The HTTP listing overlays Failed() onto every snapshot
	// entry, so this map is the load-bearing failure surface.
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "compliance")
	writeRegistrySkill(t, skillDir)

	r, err := New(tmp, &stubEmbedder{dim: 4})
	require.NoError(t, err)
	assert.Empty(t, r.Failed(), "no failures at startup")

	// Break the SKILL.md (missing temperature) + reload synchronously.
	writeRegistryBrokenSkill(t, skillDir)
	r.reloadOne(filepath.Join(skillDir, "SKILL.md"))

	failed := r.Failed()
	require.Contains(t, failed, "compliance")
	assert.Contains(t, failed["compliance"], "temperature",
		"the failed-map message must carry the validator's SYSK-08 detail")

	// Version unchanged by the failure (keep-old) but still seeded.
	snap := r.Snapshot()
	require.Contains(t, snap, "compliance")
	assert.Greater(t, snap["compliance"].Version, int64(0), "startup-seeded version must persist across a failed reload")

	// Reload a GOOD SKILL.md again → failure cleared.
	writeRegistrySkill(t, skillDir)
	r.reloadOne(filepath.Join(skillDir, "SKILL.md"))
	assert.Empty(t, r.Failed(), "a successful reload must clear the failure")
}

// mustWatchRegistry is the shared harness for the real-fsnotify Watch
// tests below: seeds one or two skills, constructs the Registry, and
// starts the Watch consumer goroutine with a cancelled-on-cleanup ctx.
func mustWatchRegistry(t *testing.T, tmp string) *Registry {
	t.Helper()
	emb := &stubEmbedder{dim: 4}
	r, err := New(tmp, emb)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	require.NoError(t, r.Watch(ctx))
	return r
}

func TestRegistry_Watch_NewSkillDirDiscovered(t *testing.T) {
	// Must-have truth 1 — a NEW skills/<name>/SKILL.md created while the
	// watcher is running (new dir + file, e.g. cp -r / git checkout) is
	// automatically discovered and registered: snapshot gains the entry
	// with a parsed manifest and a positive version. Covers the full
	// chain "dir CREATE → subdir watch Add → SKILL.md write → debounce →
	// reconcile registers", including the cp -r race backstop: even if
	// SKILL.md landed BEFORE the subdir Add (so no per-file event ever
	// fires), the dir-Create reconcile's existence check still finds it
	// 1s later and registers it.
	if testing.Short() {
		t.Skip("fsnotify real-time test relies on real debounce timing; skip in -short")
	}
	tmp := t.TempDir()
	writeRegistrySkill(t, filepath.Join(tmp, "compliance"))
	r := mustWatchRegistry(t, tmp)

	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "new-skill"), 0o755))
	writeRegistrySkill(t, filepath.Join(tmp, "new-skill"))

	require.Eventually(t, func() bool {
		snap := r.Snapshot()
		e, ok := snap["new-skill"]
		return ok && e != nil && e.Manifest != nil && e.Version > 0
	}, 8*time.Second, 100*time.Millisecond,
		"a newly created skill dir must be discovered and registered by the watcher")

	// The pre-existing skill must be untouched.
	assert.Contains(t, r.Snapshot(), "compliance")
}

func TestRegistry_Watch_RemovedSkillDirDropped(t *testing.T) {
	// Must-have truth 2 — deleting skills/<name>/ while the watcher runs
	// removes the entry from the snapshot (GET /api/skills stops listing
	// it) and leaves NO phantom residue in the failed map. Windows
	// (ReadDirectoryChangesW often reports only the dir name) and Linux
	// (subdir watch reports the SKILL.md remove) both converge through
	// the same reconcile — no per-platform branching needed here.
	if testing.Short() {
		t.Skip("fsnotify real-time test relies on real debounce timing; skip in -short")
	}
	tmp := t.TempDir()
	writeRegistrySkill(t, filepath.Join(tmp, "compliance"))
	writeRegistrySkill(t, filepath.Join(tmp, "qa"))
	r := mustWatchRegistry(t, tmp)
	require.Len(t, r.Snapshot(), 2)

	require.NoError(t, os.RemoveAll(filepath.Join(tmp, "compliance")))

	require.Eventually(t, func() bool {
		snap := r.Snapshot()
		_, hasCompliance := snap["compliance"]
		_, hasQA := snap["qa"]
		return !hasCompliance && hasQA
	}, 8*time.Second, 100*time.Millisecond,
		"removing a skill dir must drop the entry while keeping unrelated skills")

	assert.Empty(t, r.Failed(),
		"a removed skill must not leave a phantom failed-map entry behind")
}

func TestRegistry_Watch_EditorAtomicSave_NotDropped(t *testing.T) {
	// Must-have truth 3 — editors and the wizard save atomically
	// (write temp file, remove SKILL.md, rename temp over it). The
	// remove+rename burst must NOT be mistaken for a skill deletion:
	// after the 1s debounce the file exists again, so reconcile takes
	// the reload branch — the skill stays registered AND the new
	// description is picked up. Budget covers the 1s debounce plus
	// fsnotify delivery latency.
	if testing.Short() {
		t.Skip("fsnotify real-time test relies on real debounce timing; skip in -short")
	}
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "compliance")
	skillPath := writeRegistrySkill(t, skillDir)
	r := mustWatchRegistry(t, tmp)
	require.Contains(t, r.Snapshot(), "compliance")

	tmpFile := filepath.Join(skillDir, "SKILL.md.tmp")
	require.NoError(t, os.WriteFile(tmpFile, []byte("---\n"+
		"description: atomic-save-new\n"+
		"agent: ChatModelAgent\n"+
		"reads: [\"authority/**\"]\n"+
		"writes: []\n"+
		"steps: |\n"+
		"  1. step\n"+
		"  2. step\n"+
		"  3. step\n"+
		"guardrails:\n"+
		"  prohibitions:\n"+
		"    - \"p1\"\n"+
		"    - \"p2\"\n"+
		"    - \"p3\"\n"+
		"  required_refusal_phrases: []\n"+
		"  tier_limits: {}\n"+
		"temperature: 0.0\n"+
		"---\n\n# Body\n"), 0o644))
	require.NoError(t, os.Remove(skillPath))
	require.NoError(t, os.Rename(tmpFile, skillPath))

	require.Eventually(t, func() bool {
		snap := r.Snapshot()
		e, ok := snap["compliance"]
		return ok && e != nil && e.Manifest != nil &&
			e.Manifest.Description == "atomic-save-new"
	}, 8*time.Second, 100*time.Millisecond,
		"an atomic editor save must reload the skill (new description), not drop it")

	assert.Empty(t, r.Failed())
}

func TestRegistry_Watch_InterleavedBurst_Converges(t *testing.T) {
	// Must-have truth 4 — the motivating interleaved burst: "rm -r
	// skills/compliance" and "add skills/burst-new" executed back-to-back
	// with NO wait in between, so both events land in the SAME 1s
	// debounce window and the new skill's schedule overwrites the pending
	// path of the removal (single timer, single pending path). The
	// reconcile callback's O(n) snapshot existence sweep must backstop
	// the overwritten removal: compliance is gone, burst-new is
	// registered, qa survives, no phantom failure.
	if testing.Short() {
		t.Skip("fsnotify real-time test relies on real debounce timing; skip in -short")
	}
	tmp := t.TempDir()
	writeRegistrySkill(t, filepath.Join(tmp, "compliance"))
	writeRegistrySkill(t, filepath.Join(tmp, "qa"))
	r := mustWatchRegistry(t, tmp)
	require.Len(t, r.Snapshot(), 2)

	// Back-to-back, no polling/waiting between the two operations — the
	// whole point is that both events share one debounce window.
	require.NoError(t, os.RemoveAll(filepath.Join(tmp, "compliance")))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "burst-new"), 0o755))
	writeRegistrySkill(t, filepath.Join(tmp, "burst-new"))

	require.Eventually(t, func() bool {
		snap := r.Snapshot()
		_, hasCompliance := snap["compliance"]
		_, hasQA := snap["qa"]
		burst, hasBurst := snap["burst-new"]
		return !hasCompliance && hasQA && hasBurst &&
			burst != nil && burst.Manifest != nil && burst.Version > 0
	}, 8*time.Second, 100*time.Millisecond,
		"interleaved same-window burst must converge: A removed, B registered, qa intact")

	assert.Empty(t, r.Failed())
}

func TestRegistry_Reconcile_MissingFile_RemovesDeterministic(t *testing.T) {
	// Deterministic same-package companion to the Watch tests above (no
	// fsnotify, no debounce, never skipped in -short). Pins the two
	// reconcileOne branches plus the existence sweep:
	//   a) file missing → entry removed, Failed() stays empty (removal
	//      is NOT a load failure — no phantom failed entry);
	//   b) file present → reload branch updates the description;
	//   c) an ORPHAN entry whose dir was deleted without any event is
	//      swept by the O(n) existence check on an unrelated reconcile.
	tmp := t.TempDir()
	compliancePath := writeRegistrySkill(t, filepath.Join(tmp, "compliance"))
	qaPath := writeRegistrySkill(t, filepath.Join(tmp, "qa"))
	orphanPath := writeRegistrySkill(t, filepath.Join(tmp, "orphan"))

	r, err := New(tmp, &stubEmbedder{dim: 4})
	require.NoError(t, err)
	require.Len(t, r.Snapshot(), 3)

	// (c) setup: register the orphan via a direct reload. Its dir is
	// deleted LATER (below) without any watcher arbitration — its
	// removal event never fires.
	r.reloadOne(orphanPath)
	require.Contains(t, r.Snapshot(), "orphan")

	// (a) delete compliance, reconcile its path directly: the missing
	// file must take the remove branch (not reloadOne → setFailed).
	require.NoError(t, os.RemoveAll(filepath.Join(tmp, "compliance")))
	r.reconcileOne(compliancePath)

	snap := r.Snapshot()
	assert.NotContains(t, snap, "compliance",
		"reconcile on a deleted SKILL.md path must remove the entry")
	assert.Empty(t, r.Failed(),
		"removal must not surface as a failed-load entry")
	assert.Contains(t, snap, "qa")
	assert.Contains(t, snap, "orphan",
		"sanity: the orphan is still registered — its dir still exists at this point")

	// (c) delete the orphan's dir (no event, no reconcile yet), then
	// reconcile an UNRELATED path (qa, which exists → reload branch):
	// the O(n) existence sweep must remove the orphan whose dir is gone.
	require.NoError(t, os.RemoveAll(filepath.Join(tmp, "orphan")))
	r.reconcileOne(qaPath)
	snap = r.Snapshot()
	assert.NotContains(t, snap, "orphan",
		"the existence sweep must remove entries whose SKILL.md is gone, even without their own event")
	assert.Contains(t, snap, "qa")
	assert.Empty(t, r.Failed())

	// (b) recreate compliance with a new description; reconcile takes the
	// reload branch and the new content shows up.
	writeCustomSkill(t, filepath.Join(tmp, "compliance"),
		"description: reconciled-new\n"+
			"agent: ChatModelAgent\n"+
			"reads: [\"authority/**\"]\n"+
			"writes: []\n"+
			"steps: |\n"+
			"  1. step\n"+
			"  2. step\n"+
			"  3. step\n"+
			"guardrails:\n"+
			"  prohibitions:\n"+
			"    - \"p1\"\n"+
			"    - \"p2\"\n"+
			"    - \"p3\"\n"+
			"  required_refusal_phrases: []\n"+
			"  tier_limits: {}\n"+
			"temperature: 0.0\n")
	r.reconcileOne(compliancePath)

	snap = r.Snapshot()
	require.Contains(t, snap, "compliance",
		"reconcile on an existing SKILL.md must (re)register the skill")
	require.NotNil(t, snap["compliance"].Manifest)
	assert.Equal(t, "reconciled-new", snap["compliance"].Manifest.Description,
		"the reload branch must pick up the new file content")
	assert.Greater(t, snap["compliance"].Version, int64(0))
}