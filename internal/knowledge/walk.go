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

// walk.go is the single directory walk shared by every consumer that loads
// the knowledge corpus from disk (KBM-06). Today those consumers are
// knowledge.Index (the Milvus backend) and mongo.Client.SeedFromKnowledgeDir
// (the MongoDB backend); before this file existed each kept its own
// hand-copied list of the four tier directories, and the two lists could
// drift independently (26-RESEARCH Pitfall 4). One walker now owns the
// three-state decision table:
//
//	dir ∈ registry, active     → returned (the caller indexes it)
//	dir ∈ registry, deactivated → WARN + skipped, NO audit row — a
//	                             deactivation is a known, operator-authored
//	                             state, not an anomaly worth an audit row
//	dir ∉ registry             → WARN + skipped + audit
//	                             (action_kb_dir_unregistered)
//	non-directory entries      → ignored
//
// The service never fails on an unknown directory (SC-5): an unexpected
// knowledge/<dir>/ must degrade to a visible, audited skip — never take the
// index (or the server boot) down with it, and never silently swallow the
// directory either.
//
// The returned slice is the ascending set of ACTIVE slug directories. The
// sort is load-bearing: the indexer assigns 1-based chunk IDs in walk
// order, so a deterministic walk is what keeps chunk IDs (and therefore
// citation labels) stable across re-seeds.

package knowledge

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/skillforge/skillforge/internal/kb"
)

// DirAuditFn is the audit callback the walker invokes for every decision
// that must be reproducibly recorded. nil means "do not audit" (CLI and
// spike paths); production binds an INSERT-only store.PutAudit wrapper at
// boot (see SetIndexAuditHook). Action strings follow the
// store.AuditRow.Action enum form; detail is a compact JSON object so the
// row lands in the audit store's FrozenArgs column as valid JSON.
type DirAuditFn func(action, detail string)

// actionKBDirUnregistered is the audit action emitted when a knowledge/
// subdirectory is not a registered knowledge base (KBM-06: WARN + skip +
// audit). The row is INSERT-only — it records the anomaly, it never
// mutates or removes anything.
const actionKBDirUnregistered = "action_kb_dir_unregistered"

// WalkRegisteredDirs walks the first-level subdirectories of knowledgeDir
// and dispatches each one through the registry's three-state decision
// table (see the package comment). It returns the ascending set of ACTIVE
// slug directories.
//
// reg is required: a nil registry would make every directory look
// unregistered and silently index nothing, which is indistinguishable from
// a healthy empty corpus — so it fails closed instead. audit is optional
// (nil = do not audit).
//
// An empty knowledge root — or one containing none of the registered
// directories — is a legal degraded state (SC-4): the walk returns an
// empty slice and no error, so the caller's "no corpus right now" posture
// (and the D-09 refusal it produces) holds. A knowledgeDir that cannot be
// read at all, or a nil registry, is an error.
func WalkRegisteredDirs(knowledgeDir string, reg kb.Registry, audit DirAuditFn) ([]string, error) {
	if knowledgeDir == "" {
		return nil, errors.New("knowledge.WalkRegisteredDirs: knowledgeDir is empty")
	}
	if reg == nil {
		return nil, errors.New("knowledge.WalkRegisteredDirs: registry is required (fail-closed: a nil registry would silently skip every directory)")
	}

	entries, err := os.ReadDir(knowledgeDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Absent knowledge root: the same legal degraded posture as an
			// empty one (SC-4).
			return []string{}, nil
		}
		return nil, fmt.Errorf("knowledge.WalkRegisteredDirs: read %s: %w", knowledgeDir, err)
	}

	// A deactivated base is registered-but-switched-off: List() still shows
	// it while Lookup fails closed on it. The difference decides whether a
	// skipped directory is a quiet, expected state or an audited anomaly.
	deactivated := make(map[string]bool)
	for _, e := range reg.List() {
		if !e.RetrievalEnabled {
			deactivated[e.Slug] = true
		}
	}

	// os.ReadDir already returns entries sorted by name; the explicit sort
	// keeps that guarantee local to the function whose contract states it.
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() {
			// Non-directories are ignored: a stray README or editor
			// artifact in knowledge/ must never be treated as a base.
			continue
		}
		if _, ok := reg.Lookup(name); ok {
			out = append(out, name)
			continue
		}
		if deactivated[name] {
			log.Printf("[WARN] knowledge.WalkRegisteredDirs: knowledge dir %s belongs to a deactivated knowledge base %q; skipping (content stays on disk, it just never enters the index)",
				filepath.Join(knowledgeDir, name), name)
			continue
		}
		log.Printf("[WARN] knowledge.WalkRegisteredDirs: unregistered knowledge dir %s (no knowledge base named %q in the registry); skipping — register it via the seed file or the admin endpoint before indexing",
			filepath.Join(knowledgeDir, name), name)
		if audit != nil {
			// INSERT-only audit row: the decision must be reproducible from
			// the audit trail alone, without touching the filesystem again.
			audit(actionKBDirUnregistered, fmt.Sprintf(`{"dir":%q,"knowledge_dir":%q}`, name, knowledgeDir))
		}
	}
	sort.Strings(out)
	return out, nil
}
