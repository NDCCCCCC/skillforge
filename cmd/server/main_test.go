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

package main

// Plan 26-09 Task 1 — the boot-assembly tests for the PRODUCTION
// knowledge-base registry (KBM-01/02) and its INSERT-only audit hook
// (KBM-06).
//
// Everything here is hermetic: the sqlite store runs on ":memory:" (the
// D14-C1 single-connection posture), the seed file is a per-test temp
// file, and no live Milvus / mongo / BGE / GLM endpoint is touched. The
// package still links go-duckdb (Phase 15 import assembly), so on this
// host the suite runs under WSL like the other CGO packages.
//
// Two structural locks ride the source of cmd/server/main.go itself:
//
//   - the boot ORDER (kb registry open before the skill loader, audit
//     hooks bound before the retriever) is asserted by line position —
//     T-26-09-1's mitigation is an order that is provable, not just
//     documented;
//   - the 26-03/26-04 read-only seed snapshot must never come back (the
//     assembly contract: one kb.Open, zero scaffold markers).

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/retriever/mongo"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// testSeedYAML mirrors knowledge_bases.yaml's four baselines with ASCII
// display names (tests stay hermetic and CN-literal free, D26-04 form).
// Every field the loader requires is declared explicitly — a silent
// default must never decide retrieval eligibility.
const testSeedYAML = `version: "1"
knowledge_bases:
  - slug: authority
    display_name: Authority
    role: authority
    description: citable evidence source
    retrieval_enabled: true
  - slug: historical
    display_name: Historical
    role: historical
    description: superseded revisions
    retrieval_enabled: true
  - slug: personal
    display_name: Personal
    role: personal
    description: user-private material
    retrieval_enabled: true
  - slug: draft
    display_name: Draft
    role: draft
    description: not yet effective
    retrieval_enabled: true
`

// writeSeedFile writes the given seed content to a temp file and returns
// its path.
func writeSeedFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "knowledge_bases.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	return path
}

// newTestStore opens a single-connection in-memory sqlite store (the
// production SQLiteStore — the same type the compile-time
// `var _ kb.Store = (*SQLiteStore)(nil)` assertion covers), so the
// assembly under test exercises the real persistence adapter.
func newTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	st, err := sqlite.New(context.Background(), sqlite.Config{DSN: ":memory:"})
	if err != nil {
		t.Fatalf("sqlite.New(:memory:): %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// auditCaptureStore implements ONLY store.Store's PutAudit. The embedded
// nil interface is the INSERT-only lock: any other store method the audit
// wrapper ever grew (an UPDATE, a DELETE, a session write) would panic on
// a nil receiver inside this test instead of silently mutating a row.
type auditCaptureStore struct {
	store.Store
	rows []store.AuditRow
}

func (f *auditCaptureStore) PutAudit(_ context.Context, row store.AuditRow) (int64, error) {
	f.rows = append(f.rows, row)
	return int64(len(f.rows)), nil
}

// TestBuildKBRegistryFromStore is the production-assembly contract: the
// seed's structure plus the store's runtime state merge into ONE
// store-backed registry, a malformed seed is an error (the caller
// log.Fatalf's it), and an admin deactivation survives a reopen.
func TestBuildKBRegistryFromStore(t *testing.T) {
	t.Run("valid seed yields the four active baselines", func(t *testing.T) {
		st := newTestStore(t)
		seed := writeSeedFile(t, testSeedYAML)

		reg, err := buildKBRegistry(context.Background(), seed, st)
		if err != nil {
			t.Fatalf("buildKBRegistry: %v", err)
		}
		if reg == nil {
			t.Fatal("buildKBRegistry returned a nil registry")
		}
		// draft is ACTIVE (it exists, skills may declare it) but never
		// retrieval-eligible — role semantics outrank the entry switch.
		got := reg.ActiveSlugs()
		want := []string{"authority", "draft", "historical", "personal"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ActiveSlugs = %v, want %v", got, want)
		}
		// The registry must be store-backed, not an in-memory snapshot:
		// only then do admin mutations survive a restart (KBM-02).
		rows, err := st.LoadBases(context.Background())
		if err != nil {
			t.Fatalf("LoadBases: %v", err)
		}
		if len(rows) != len(want) {
			t.Fatalf("persisted rows = %d, want %d (the registry must be backed by the sqlite knowledge_bases table)", len(rows), len(want))
		}
	})

	t.Run("deactivation survives a reopen (Pitfall 6a)", func(t *testing.T) {
		st := newTestStore(t)
		seed := writeSeedFile(t, testSeedYAML)

		reg, err := buildKBRegistry(context.Background(), seed, st)
		if err != nil {
			t.Fatalf("buildKBRegistry: %v", err)
		}
		if _, err := reg.Deactivate("personal"); err != nil {
			t.Fatalf("Deactivate(personal): %v", err)
		}
		if _, ok := reg.Lookup("personal"); ok {
			t.Fatal("deactivated base still resolves (Lookup must fail closed)")
		}

		// Simulated restart: the same store, a fresh registry build. The
		// seed still declares personal, and the seed must NOT resurrect
		// the admin's switch ("structure belongs to config, the switch
		// belongs to ops").
		reg2, err := buildKBRegistry(context.Background(), seed, st)
		if err != nil {
			t.Fatalf("buildKBRegistry (reopen): %v", err)
		}
		if _, ok := reg2.Lookup("personal"); ok {
			t.Fatal("admin deactivation did not survive the reopen — kb.Open resurrected the switch")
		}
		if got := len(reg2.List()); got != 4 {
			t.Fatalf("List() after reopen = %d entries, want 4 (deactivation hides, it never destroys)", got)
		}
	})

	t.Run("malformed seed is an error naming the file", func(t *testing.T) {
		st := newTestStore(t)
		bad := writeSeedFile(t, `version: "1"
knowledge_bases:
  - slug: authority
    display_name: Authority
    role: not_a_role
    description: role outside the closed vocabulary
    retrieval_enabled: true
`)

		reg, err := buildKBRegistry(context.Background(), bad, st)
		if err == nil {
			t.Fatal("expected an error for a seed with an unknown role")
		}
		if reg != nil {
			t.Fatal("a malformed seed must not return a registry")
		}
		if !strings.Contains(err.Error(), bad) {
			t.Fatalf("error %q must name the seed path %q (the operator fixes the file the log names)", err, bad)
		}
	})

	t.Run("store that cannot serve the registry fails closed", func(t *testing.T) {
		seed := writeSeedFile(t, testSeedYAML)
		// auditCaptureStore satisfies store.Store but NOT kb.Store — the
		// production SQLiteStore is the only adapter allowed here.
		reg, err := buildKBRegistry(context.Background(), seed, &auditCaptureStore{})
		if err == nil {
			t.Fatal("expected an error when the store does not satisfy kb.Store")
		}
		if reg != nil {
			t.Fatal("a non-kb.Store must not produce a registry")
		}
	})
}

// TestAuditHookWritesInsertOnly locks the audit seam: the wrapper
// cmd/server binds is INSERT-only — one PutAudit row per decision, the
// detail riding FrozenArgs, and NO session/skill/query columns (this row
// records a corpus anomaly, not a chat turn).
func TestAuditHookWritesInsertOnly(t *testing.T) {
	fake := &auditCaptureStore{}
	hook := kbDirAuditHook(context.Background(), fake)

	// Bind through the production seams exactly as boot does, so the test
	// fails if the setter signature drifts.
	knowledge.SetIndexAuditHook(hook)
	mongo.SetSeedAuditHook(hook)
	t.Cleanup(func() {
		knowledge.SetIndexAuditHook(nil)
		mongo.SetSeedAuditHook(nil)
	})

	hook("action_kb_dir_unregistered", `{"dir":"rogue","knowledge_dir":"knowledge"}`)

	if len(fake.rows) != 1 {
		t.Fatalf("audit rows captured = %d, want exactly 1 (one decision, one INSERT)", len(fake.rows))
	}
	row := fake.rows[0]
	if row.Action != "action_kb_dir_unregistered" {
		t.Fatalf("Action = %q, want action_kb_dir_unregistered", row.Action)
	}
	if !strings.Contains(string(row.FrozenArgs), "rogue") {
		t.Fatalf("FrozenArgs = %q, want the walk detail JSON inside it", string(row.FrozenArgs))
	}
	// An unregistered-directory row is corpus telemetry: it must never be
	// mistaken for a chat turn (the SC-9 session partition stays empty).
	if row.SessionID != "" || row.SkillID != "" || row.Query != "" {
		t.Fatalf("row carries chat columns: session=%q skill=%q query=%q, want all empty", row.SessionID, row.SkillID, row.Query)
	}
	if row.UserID != "system" {
		t.Fatalf("UserID = %q, want system (no human actor drove this decision)", row.UserID)
	}
	if row.Timestamp.IsZero() {
		t.Fatal("Timestamp is zero — the row would sort outside every audit window")
	}
}

// TestAuditHookLandsInSqliteAuditTable proves the same wrapper against
// the REAL store: the row is readable back through QueryAudit, i.e. it
// genuinely landed in the audit table rather than in a parallel sink.
func TestAuditHookLandsInSqliteAuditTable(t *testing.T) {
	st := newTestStore(t)
	hook := kbDirAuditHook(context.Background(), st)

	hook("action_kb_dir_unregistered", `{"dir":"rogue","knowledge_dir":"knowledge"}`)

	rows, err := st.QueryAudit(context.Background(), store.AuditFilter{Action: "action_kb_dir_unregistered"})
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("audit table rows for action_kb_dir_unregistered = %d, want 1", len(rows))
	}
	if !strings.Contains(string(rows[0].FrozenArgs), "rogue") {
		t.Fatalf("persisted FrozenArgs = %q, want the walk detail", string(rows[0].FrozenArgs))
	}
}

// TestEmptyRegistryBootsWithWarning: an ABSENT seed over an empty table
// is the single legal degraded state — the assembly proceeds (it must
// never Fatal: a deployment may run on its persisted registry alone) and
// says so out loud. Silent emptiness is the Pitfall-1 black hole this
// plan exists to close.
func TestEmptyRegistryBootsWithWarning(t *testing.T) {
	st := newTestStore(t)
	absent := filepath.Join(t.TempDir(), "does_not_exist.yaml")

	var buf bytes.Buffer
	oldOut := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(oldOut)
		log.SetFlags(oldFlags)
	})

	reg, err := buildKBRegistry(context.Background(), absent, st)
	if err != nil {
		t.Fatalf("an absent seed is a legal degraded state, got error: %v", err)
	}
	if reg == nil {
		t.Fatal("an empty registry must still be a usable (nil-safe) registry value")
	}
	if got := len(reg.List()); got != 0 {
		t.Fatalf("empty-store + absent-seed registry = %d entries, want 0", got)
	}
	out := buf.String()
	if !strings.Contains(out, "empty KB registry") {
		t.Fatalf("boot log %q does not announce the empty registry", out)
	}
	if !strings.Contains(out, absent) {
		t.Fatalf("boot log %q does not name the seed path %q", out, absent)
	}
	if !strings.Contains(out, "every tier-Skill validation will fail") {
		t.Fatalf("boot log %q does not state the consequence of an empty registry", out)
	}
}

// TestBootAssemblyOrder is the T-26-09-1 mitigation made executable: the
// kb registry is opened BEFORE the skill loader consumes it, the audit
// hooks are bound BEFORE anything can walk the corpus, and the
// seed scaffold those plans left behind is gone for good.
func TestBootAssemblyOrder(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	src := string(raw)

	indexOf := func(needle string) int {
		i := strings.Index(src, needle)
		if i < 0 {
			t.Fatalf("main.go no longer contains %q — update this order lock alongside the boot sequence", needle)
		}
		return i
	}
	// indexOfInMain searches from `func main() {` onward so a doc comment
	// mentioning a call cannot satisfy (or break) a boot-order assertion.
	mainIdx := indexOf("func main() {")
	mainSrc := src[mainIdx:]
	indexOfInMain := func(needle string) int {
		i := strings.Index(mainSrc, needle)
		if i < 0 {
			t.Fatalf("main() no longer contains %q — update this order lock alongside the boot sequence", needle)
		}
		return i
	}

	// 1. The registry exists before anything consumes it. kb.Open is the
	//    production assembly; the skill loader is its first consumer. (The
	//    kb.Open presence check is file-wide; the ORDER check reads main's
	//    body so the package doc cannot satisfy it.)
	if !strings.Contains(src, "kb.Open(") {
		t.Fatal("main.go no longer calls kb.Open — the production registry assembly is gone")
	}
	if boot, loader := indexOfInMain("buildKBRegistry(ctx, "), indexOfInMain("skill.NewLoader("); boot > loader {
		t.Fatalf("boot order broken: the registry build (offset %d in main) must precede skill.NewLoader (offset %d)", boot, loader)
	}

	// 2. The store exists before the registry (kb.Open persists through
	//    it) and the audit hooks are bound before the corpus walk.
	if st, reg := indexOfInMain("st, err := buildStore(ctx)"), indexOfInMain("buildKBRegistry(ctx, "); st > reg {
		t.Fatalf("boot order broken: buildStore (offset %d in main) must precede the registry build (offset %d)", st, reg)
	}
	if hook, ret := indexOfInMain("knowledge.SetIndexAuditHook("), indexOfInMain("buildRetriever(ctx, "); hook > ret {
		t.Fatalf("boot order broken: the audit hook binding (offset %d in main) must precede buildRetriever (offset %d) — the boot-time corpus index writes its unregistered-dir rows through it", hook, ret)
	}

	// 3. The whole registry is ONE value injected everywhere (single
	//    source): loader, skill registry, wizard, knowledge graph, Deps.
	for _, consumer := range []string{
		"skill.NewLoader(\"skills/\", kbReg)",
		"skill.New(\"skills/\", nil, kbReg)",
		"skill.NewWizard(cm, \"skills/\", kbReg)",
		"knowledge.New(r, kbReg)",
		"KBRegistry:  kbReg,",
	} {
		if !strings.Contains(src, consumer) {
			t.Fatalf("main.go does not inject the registry into %q — Deps.KBRegistry must stay the single source", consumer)
		}
	}

	// 4. The audit wrapper is bound to BOTH corpus seams and is
	//    INSERT-only by construction (PutAudit is its only store verb).
	for _, binding := range []string{
		"knowledge.SetIndexAuditHook(kbDirAuditHook(ctx, st))",
		"mongo.SetSeedAuditHook(kbDirAuditHook(ctx, st))",
	} {
		if !strings.Contains(src, binding) {
			t.Fatalf("main.go does not bind the audit wrapper as %q", binding)
		}
	}
	hookBody := src[indexOf("func kbDirAuditHook("):]
	if closing := strings.Index(hookBody, "\n}\n"); closing >= 0 {
		hookBody = hookBody[:closing]
	}
	if !strings.Contains(hookBody, "PutAudit(") {
		t.Fatal("kbDirAuditHook does not write through PutAudit — the unknown-directory row must be a real INSERT")
	}
	for _, verb := range []string{"UpdateAudit", "DeleteAudit", "DeleteSession", "MarkApproved", "MarkRejected"} {
		if strings.Contains(hookBody, verb) {
			t.Fatalf("kbDirAuditHook references %q — the audit seam is INSERT-only by contract", verb)
		}
	}

	// 5. The seed scaffold must never return: zero scaffold markers of
	//    any kind in the assembly file.
	if strings.Contains(src, "PROVISIONAL") {
		t.Fatal("cmd/server/main.go still carries a scaffold marker — the 26-03/26-04 seed snapshots were replaced by kb.Open")
	}
}

// TestKBSeedEnvDefault locks the env surface: SKILLFORGE_KB_SEED selects
// the controlled seed file and defaults to the in-repo controlled file.
func TestKBSeedEnvDefault(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(raw), `envOr("SKILLFORGE_KB_SEED", "knowledge_bases.yaml")`) {
		t.Fatal("cmd/server must read SKILLFORGE_KB_SEED with the controlled default knowledge_bases.yaml")
	}
}

// TestMilvusRecreateGateWiring locks the assembly side of the one-shot
// re-seed gate (T-26-09-2): the destructive path is reachable only behind
// the explicit SKILLFORGE_MILVUS_RECREATE gate, the WARN names the env var
// and its cost, and the gate is evaluated BEFORE the collection is ensured
// and the corpus indexed (a recreate that runs after the seed would waste
// the whole embed pass).
func TestMilvusRecreateGateWiring(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	src := string(raw)

	gate := "milvus.ShouldRecreate(os.Getenv(milvus.RecreateEnvVar))"
	recreate := "RecreateCollection(ctx, defaultCollection)"
	for _, needle := range []string{gate, recreate} {
		if !strings.Contains(src, needle) {
			t.Fatalf("main.go does not wire the re-seed path (%q missing)", needle)
		}
	}

	// Everything below reads the milvus branch of buildRetriever only, so
	// an unrelated call elsewhere in the file cannot satisfy the lock.
	i := strings.Index(src, "func buildRetriever(")
	if i < 0 {
		t.Fatal("buildRetriever not found")
	}
	branch := src[i:]

	gateAt := strings.Index(branch, gate)
	ensureAt := strings.Index(branch, "mc.Ensure(ctx)")
	if ensureAt < 0 {
		t.Fatal("buildRetriever's milvus branch no longer ensures the collection")
	}
	if gateAt < 0 {
		t.Fatal("buildRetriever's milvus branch does not read the RECREATE gate")
	}
	if gateAt > ensureAt {
		t.Fatalf("gate read must precede mc.Ensure (offset %d > %d) — a recreate after the ensure/seed wastes the entire embed pass", gateAt, ensureAt)
	}
	recreateAt := strings.Index(branch, recreate)
	if recreateAt < gateAt {
		t.Fatalf("the recreate call (offset %d) must sit inside the gate branch (gate at %d)", recreateAt, gateAt)
	}

	// The WARN is the operator's only in-log signal that a destructive
	// one-shot ran; it must name the env var and the re-embed consequence.
	warn := branch[strings.Index(branch, "[WARN]"):]
	if !strings.Contains(warn[:min(len(warn), 600)], "recreating collection") ||
		!strings.Contains(warn[:min(len(warn), 600)], "SKILLFORGE_MILVUS_RECREATE") ||
		!strings.Contains(warn[:min(len(warn), 600)], "re-embedded") {
		t.Fatalf("the RECREATE WARN must name the collection, the env var and the re-embed cost; got %.600s", warn)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
