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

// Package induction implements LLM-driven TBox schema induction with a
// mandatory human-approval gate (ONT-04).
//
// Flow: admin submits text → Generate calls the LLM for a natural-language
// proposal → the backend derives a PascalCase local name + bilingual
// rdfs:label (D-Induct-01/02) → the proposal persists as pending →
// Approve (human click) appends the proposed triples to a new TBox version
// (v{N+1}.ttl), updates the pointer file, and inserts into the KG store.
package induction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/skillforge/skillforge/internal/ontology/shacl"
	"github.com/skillforge/skillforge/internal/ontology/store"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ErrIRIConflict reports a local-name collision against the current TBox.
var ErrIRIConflict = errors.New("induction: local name already exists in TBox")

// ChangelogWho is the trigger enumeration for changelog entries (D-LOG-02).
type ChangelogWho = string

// ChangelogWhat describes the triple-level change (D-LOG-02).
type ChangelogWhat struct {
	Added   []string `json:"added"`   // IRI list — new IRIs introduced
	Removed []string `json:"removed"` // IRI list — IRIs removed by this operation
}

// ChangelogEntry is one append-only record in tbox_changelog.jsonl (D-LOG-01).
// Version is the resulting TBox version number after the operation.
// Who is one of: "admin", "induction:llm", "system:rollback" (D-LOG-02).
// Why carries the LLM reasoning text, admin note, or rollback reason.
type ChangelogEntry struct {
	Version int           `json:"version"`     // resulting version v{N}
	Who     string        `json:"who"`        // D-LOG-02 enum
	When    string        `json:"when"`       // RFC3339 UTC
	What    ChangelogWhat `json:"what"`       // added/removed IRI lists
	Why     string        `json:"why"`        // LLM reasoning | admin note | rollback reason
}

// Generator is the narrow LLM surface induction needs. It is satisfied by
// Eino's model.ToolCallingChatModel, which lets tests inject a stub.
type Generator interface {
	Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error)
}

// generatePrompt is the fixed instruction (D-Induct-02): the LLM proposes
// in natural language; naming is the backend's job.
const generatePrompt = `You are an ontology engineer for SkillForge.
Analyze the following text and propose ONE new schema element (a Class, an
ObjectProperty, or a DatatypeProperty) that the SkillForge TBox currently
lacks. Answer with a JSON object EXACTLY of the shape:
{"kind": "Class" | "ObjectProperty" | "DatatypeProperty",
 "description_zh": "<one-sentence Chinese description>",
 "description_en": "<one-sentence English description>"}
Output ONLY the JSON object, no fences, no prose.

Text:
%s`

// llmProposalShape mirrors the fixed prompt output contract.
type llmProposalShape struct {
	Kind          string `json:"kind"`
	DescriptionZH string `json:"description_zh"`
	DescriptionEN string `json:"description_en"`
}

// Manager owns the proposal queue and the approve pipeline.
type Manager struct {
	mu      sync.Mutex
	ns      string
	dataDir string // holds proposals.json, tbox_version.txt, skillforge_tbox*.ttl
	chat    Generator
	kg      *store.Store
	queue   []Proposal
	nextID  int
	// onChange (when set) fires after every queue/TBox mutation so the
	// caller can schedule a debounced KG snapshot (D-KG-01/03).
	onChange func()
}

// SetOnChange registers the post-mutation callback (idempotent; the boot
// sequence wires the debounced snapshot writer here).
func (m *Manager) SetOnChange(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}

// NewManager constructs a Manager. dataDir is created if missing; the
// proposal queue is loaded from {dataDir}/proposals.json when present.
func NewManager(ns, dataDir string, chat Generator, kg *store.Store) (*Manager, error) {
	if ns == "" {
		return nil, errors.New("induction: namespace is required")
	}
	if kg == nil {
		return nil, errors.New("induction: KG store is required")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("induction: mkdir %s: %w", dataDir, err)
	}
	m := &Manager{ns: ns, dataDir: dataDir, chat: chat, kg: kg}
	if err := m.loadQueue(); err != nil {
		return nil, err
	}
	return m, nil
}

// fireChange invokes the onChange hook if registered (used after every
// queue/TBox mutation, outside the lock ordering concerns — the hook only
// schedules a timer).
func (m *Manager) fireChange() {
	if m.onChange != nil {
		m.onChange()
	}
}

// Generate produces a pending Proposal from free text. The LLM proposes the
// semantics; ToPascalCase derives the local name deterministically (D-Induct-01).
func (m *Manager) Generate(ctx context.Context, text string) (Proposal, error) {
	if strings.TrimSpace(text) == "" {
		return Proposal{}, errors.New("induction: source text is empty")
	}
	if m.chat == nil {
		return Proposal{}, errors.New("induction: no LLM client configured")
	}
	resp, err := m.chat.Generate(ctx, []*schema.Message{
		schema.SystemMessage("You output only valid JSON."),
		schema.UserMessage(fmt.Sprintf(generatePrompt, text)),
	}, model.WithTemperature(0.0))
	if err != nil {
		return Proposal{}, fmt.Errorf("induction: llm generate: %w", err)
	}
	if resp == nil || resp.Content == "" {
		return Proposal{}, errors.New("induction: llm returned empty response")
	}
	raw := extractJSON(resp.Content)
	if raw == "" {
		return Proposal{}, fmt.Errorf("induction: no JSON object in llm response: %s", resp.Content)
	}
	var shape llmProposalShape
	if err := json.Unmarshal([]byte(raw), &shape); err != nil {
		return Proposal{}, fmt.Errorf("induction: unmarshal llm response: %w (raw: %s)", err, raw)
	}
	kind := ProposalKind(shape.Kind)
	switch kind {
	case KindClass, KindObjectProperty, KindDatatypeProperty:
	default:
		return Proposal{}, fmt.Errorf("induction: unsupported kind %q", shape.Kind)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	local, err := m.uniqueLocalName(shape.DescriptionEN)
	if err != nil {
		return Proposal{}, err
	}
	p := Proposal{
		ID:            m.allocID(),
		Kind:          kind,
		DescriptionZH: shape.DescriptionZH,
		DescriptionEN: shape.DescriptionEN,
		LocalName:     local,
		ProposedIRI:   m.ns + local,
		ProposedTriples: proposedTriples(m.ns, kind, local,
			shape.DescriptionZH, shape.DescriptionEN),
		Status:     StatusPending,
		SourceText: text,
		CreatedAt:  time.Now().UTC(),
	}
	if err := p.Validate(); err != nil {
		return Proposal{}, err
	}
	if err := m.persistLocked(); err != nil {
		return Proposal{}, err
	}
	m.queue = append(m.queue, p)
	return p, m.persistLocked()
}

// Approve applies a pending proposal: writes TBox v{N+1}.ttl, bumps the
// version, refreshes the pointer file, and inserts the new triples into the
// KG store. customLocalName overrides the generated name when non-empty
// (D-Induct-04); conflicts return ErrIRIConflict.
func (m *Manager) Approve(proposalID, customLocalName string) (Proposal, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := -1
	for i := range m.queue {
		if m.queue[i].ID == proposalID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Proposal{}, 0, fmt.Errorf("induction: proposal %s not found", proposalID)
	}
	p := &m.queue[idx]
	if p.Status != StatusPending {
		return Proposal{}, 0, fmt.Errorf("induction: proposal %s is %s, not pending", proposalID, p.Status)
	}
	if customLocalName != "" && customLocalName != p.LocalName {
		if err := m.previewIRILocked(customLocalName); err != nil {
			return *p, 0, err
		}
		p.LocalName = customLocalName
		p.ProposedIRI = m.ns + customLocalName
		p.ProposedTriples = proposedTriples(m.ns, p.Kind, p.LocalName, p.DescriptionZH, p.DescriptionEN)
	}

	// Read current TBox, append proposal triples, write as v{N+1}.
	current := m.currentTTLPath()
	base := ""
	data, err := os.ReadFile(current)
	switch {
	case err == nil:
		base = string(data)
	case os.IsNotExist(err):
		// Fresh deployment: the first approve bootstraps the TBox.
		base = ""
	default:
		// WR-01: a read ERROR (permissions, IO) is not the same as an absent
		// file. Proceeding with an empty base would make newTTL contain ONLY
		// this proposal's triples, and the atomic pointer overwrite below
		// would silently truncate the entire existing TBox.
		return *p, 0, fmt.Errorf("induction: read current TBox %s: %w", current, err)
	}
	version := m.readVersion()
	next := version + 1
	newTTL := base
	if newTTL != "" && !strings.HasSuffix(newTTL, "\n") {
		newTTL += "\n"
	}
	newTTL += fmt.Sprintf("\n# === schema induction: proposal %s (approved %s) ===\n", p.ID, time.Now().UTC().Format(time.RFC3339))
	for _, t := range p.ProposedTriples {
		newTTL += t + "\n"
	}

	// WR-02 ordering: persist the approved queue state FIRST. If this fails,
	// nothing on disk changed and the proposal is still pending — a retry is
	// clean. (The pre-fix order flipped the status in memory first and ran
	// persistLocked LAST: a persist failure returned an error while TBox/KG
	// had already mutated and memory said approved vs disk pending — after a
	// restart the still-pending disk queue allowed a duplicate approve with
	// duplicate KG inserts and a second version bump.)
	now := time.Now().UTC()
	p.Status = StatusApproved
	p.DecidedAt = &now
	if err := m.persistLocked(); err != nil {
		// Revert the in-memory flip so memory matches disk (still pending).
		p.Status = StatusPending
		p.DecidedAt = nil
		return *p, 0, fmt.Errorf("induction: persist approved state (nothing written, proposal still pending): %w", err)
	}

	versioned := filepath.Join(m.dataDir, fmt.Sprintf("skillforge_tbox.v%d.ttl", next))
	if err := writeFileAtomic(versioned, []byte(newTTL)); err != nil {
		return *p, 0, fmt.Errorf("induction: write %s (approved state persisted, TBox NOT updated — triples remain in proposals.json for ApprovedTriples replay): %w", versioned, err)
	}
	if err := writeFileAtomic(current, []byte(newTTL)); err != nil {
		return *p, 0, fmt.Errorf("induction: refresh pointer %s (v%d written, pointer NOT updated): %w", current, next, err)
	}
	if err := writeFileAtomic(filepath.Join(m.dataDir, "tbox_version.txt"), []byte(fmt.Sprintf("%d\n", next))); err != nil {
		return *p, 0, fmt.Errorf("induction: write version file (v%d and pointer written, version file NOT updated): %w", next, err)
	}

	// Insert new triples into the KG store so queries see them immediately.
	// InsertUnique (WR-02): a re-approve after a partial failure must not
	// duplicate triples in the store.
	for _, t := range p.ProposedTriples {
		if trip, ok := parseTripleLine(t, m.ns); ok {
			m.kg.InsertUnique(trip)
		}
	}

	m.fireChange()

	// D-LOG-04: append one changelog entry per approve.
	entry := ChangelogEntry{
		Version: next,
		Who:     "induction:llm",
		When:    now.Format(time.RFC3339),
		What:    ChangelogWhat{Added: []string{p.ProposedIRI}, Removed: []string{}},
		Why:     p.SourceText,
	}
	if err := m.AppendChangelog(entry); err != nil {
		// log WARN, don't fail the approve (changelog is audit, not critical path)
		fmt.Fprintf(os.Stderr, "[WARN] induction: append changelog: %v\n", err)
	}

	return *p, next, nil
}

// RollbackTBox reverts to a previous TBox version.
// Returns (new_version, post_rollback_violation_count, error).
// Writes v{N+1} = copy of target version TTL, updates the pointer file,
// rebuilds the KG store from the new TTL, and triggers SHACL revalidation
// per D-LOG-03 + D-PROV-09.
func (m *Manager) RollbackTBox(targetVersion int, reason string) (int, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if targetVersion < 1 {
		return 0, 0, fmt.Errorf("induction: target version %d is invalid", targetVersion)
	}

	current := m.readVersion()
	if targetVersion >= current {
		return 0, 0, fmt.Errorf("induction: cannot rollback to version %d (current is %d)", targetVersion, current)
	}

	// Step 1: Read the target version TTL file.
	targetPath := filepath.Join(m.dataDir, fmt.Sprintf("skillforge_tbox.v%d.ttl", targetVersion))
	targetTTL, err := os.ReadFile(targetPath)
	if err != nil {
		return 0, 0, fmt.Errorf("induction: read target TTL v%d: %w", targetVersion, err)
	}

	currentTTLPath := filepath.Join(m.dataDir, "skillforge_tbox.ttl")

	// Step 1b (CR-01): capture the CURRENT TBox subject set from the pointer
	// TTL BEFORE it is overwritten. Rollback must replace only the TBox
	// layer: the pre-fix behavior deleted EVERY subject in the store, wiping
	// the ECL ABox triples (note/table/entity) that D-KG-02 requires to
	// survive restarts — and Step 8's immediate snapshot persisted that wipe
	// to disk, making the deletion permanent.
	// ABox subjects never appear in a TBox TTL, so subject-set diffing on the
	// pointer TTL is the exact boundary between the two layers. The throwaway
	// store reuses the same Turtle parser as LoadFromTurtle — no text-level
	// re-parsing of TTL lines.
	prevTBox := store.New()
	prevData, prevErr := os.ReadFile(currentTTLPath)
	switch {
	case prevErr == nil:
		if _, err := prevTBox.LoadFromTurtle(string(prevData)); err != nil {
			return 0, 0, fmt.Errorf("induction: parse current pointer TTL: %w", err)
		}
	case os.IsNotExist(prevErr):
		// Fresh deployment without a pointer TTL: nothing to delete.
	default:
		return 0, 0, fmt.Errorf("induction: read current pointer TTL: %w", prevErr)
	}
	oldSubjects := make(map[string]bool, len(prevTBox.Query("", "", "")))
	for _, t := range prevTBox.Query("", "", "") {
		oldSubjects[t.S] = true
	}

	// Step 2: Compute next = current + 1.
	next := current + 1

	// Step 3: Write v{next}.ttl = copy of target version TTL.
	newTTL := string(targetTTL)
	versioned := filepath.Join(m.dataDir, fmt.Sprintf("skillforge_tbox.v%d.ttl", next))
	if err := writeFileAtomic(versioned, []byte(newTTL)); err != nil {
		return 0, 0, fmt.Errorf("induction: write v%d TTL: %w", next, err)
	}

	// Step 4: Update pointer file.
	if err := writeFileAtomic(currentTTLPath, []byte(newTTL)); err != nil {
		return 0, 0, fmt.Errorf("induction: refresh pointer TTL: %w", err)
	}

	// Step 5: Update version file.
	if err := writeFileAtomic(filepath.Join(m.dataDir, "tbox_version.txt"), []byte(fmt.Sprintf("%d\n", next))); err != nil {
		return 0, 0, fmt.Errorf("induction: write version file: %w", err)
	}

	// Step 6 (CR-01): delete ONLY the previous TBox subjects. Every ABox
	// triple — including edges FROM ABox entities INTO TBox class IRIs — has
	// an ABox subject and therefore survives. Quarantine-zone triples are
	// store-keyed separately from the main indices and are not touched by
	// DeleteSubject.
	for s := range oldSubjects {
		m.kg.DeleteSubject(s)
	}

	// Step 7: Reload KG from new TTL.
	if _, err := m.kg.LoadFromTurtle(newTTL); err != nil {
		return 0, 0, fmt.Errorf("induction: load new TTL into KG: %w", err)
	}

	// Step 8: Trigger snapshot write.
	snapshotPath := filepath.Join(m.dataDir, "skillforge_kg.json")
	if err := m.kg.WriteSnapshot(snapshotPath); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] induction: post-rollback snapshot: %v\n", err)
	}

	// Step 9: Fire change hook (debounced snapshot will also fire, but explicit is fine).
	m.fireChange()

	// Step 10: Append changelog entry. Added/Removed are the TBox subject
	// sets that differ between the replaced version and the rolled-back-to
	// version (CR-01: both were hardcoded to empty lists before, which
	// misreported the operation's actual footprint).
	newTBox := store.New()
	if _, err := newTBox.LoadFromTurtle(newTTL); err != nil {
		return 0, 0, fmt.Errorf("induction: reparse new TTL for changelog: %w", err)
	}
	newSubjects := make(map[string]bool)
	for _, t := range newTBox.Query("", "", "") {
		newSubjects[t.S] = true
	}
	added := make([]string, 0, len(newSubjects))
	for s := range newSubjects {
		if !oldSubjects[s] {
			added = append(added, s)
		}
	}
	removed := make([]string, 0)
	for s := range oldSubjects {
		if !newSubjects[s] {
			removed = append(removed, s)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	why := fmt.Sprintf("rollback to v%d: %s", targetVersion, reason)
	entry := ChangelogEntry{
		Version: next,
		Who:     "system:rollback",
		When:    time.Now().UTC().Format(time.RFC3339),
		What:    ChangelogWhat{Added: added, Removed: removed},
		Why:     why,
	}
	if err := m.AppendChangelog(entry); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] induction: append rollback changelog: %v\n", err)
	}

	// Step 11: Run SHACL revalidation and return violation count (D-PROV-09).
	// D-46-02: rollback consumes the same env-driven shapes path list as
	// boot (the single read point lives in the shacl package) — the former
	// hard-coded data/shacl/shapes.ttl join never resolved and reduced this
	// revalidation to a silent no-op.
	report, err := shacl.ValidateWithTrigger(m.kg, shacl.ShapesPathsFromEnv(), "rollback")
	if err != nil {
		// SHACL failure is not a rollback failure — log and return 0
		// violations (D-PROV-09 warn-and-return-0 posture; boot/rollback
		// shapes parity per D-46-02).
		fmt.Fprintf(os.Stderr, "[WARN] induction: post-rollback SHACL validation: %v\n", err)
		return next, 0, nil
	}
	return next, report.ErrorCount, nil
}

// Reject marks a proposal rejected; the record is kept (audit trail).
func (m *Manager) Reject(proposalID string) (Proposal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.queue {
		if m.queue[i].ID == proposalID {
			if m.queue[i].Status != StatusPending {
				return m.queue[i], fmt.Errorf("induction: proposal %s is %s, not pending", proposalID, m.queue[i].Status)
			}
			now := time.Now().UTC()
			m.queue[i].Status = StatusRejected
			m.queue[i].DecidedAt = &now
			err := m.persistLocked()
			m.fireChange()
			return m.queue[i], err
		}
	}
	return Proposal{}, fmt.Errorf("induction: proposal %s not found", proposalID)
}

// List returns proposals, newest first.
func (m *Manager) List() []Proposal {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Proposal, len(m.queue))
	copy(out, m.queue)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// AppendChangelog atomically appends one JSONL entry to tbox_changelog.jsonl
// (D-LOG-01). The file is never truncated — only O_APPEND writes occur.
func (m *Manager) AppendChangelog(entry ChangelogEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(m.dataDir, "tbox_changelog.jsonl"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

// ListChangelog reads tbox_changelog.jsonl and returns all entries sorted
// newest-first. Missing file is not an error (returns empty slice).
func (m *Manager) ListChangelog() ([]ChangelogEntry, error) {
	path := filepath.Join(m.dataDir, "tbox_changelog.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []ChangelogEntry{}, nil
		}
		return nil, err
	}
	var entries []ChangelogEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e ChangelogEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, e)
	}
	// Newest first
	sort.Slice(entries, func(i, j int) bool { return entries[i].Version > entries[j].Version })
	return entries, nil
}

// Version reports the current TBox version (reads tbox_version.txt; the
// admin tbox endpoint surfaces it alongside the TTL text).
func (m *Manager) Version() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.readVersion()
}

// DataDir exposes the ontology data directory (pointer TTL path lives
// there; the snapshot handler writes the KG JSON beside it).
func (m *Manager) DataDir() string { return m.dataDir }

// PreviewIRI reports whether localName collides with the current TBox
// (D-Induct-04 drawer rename check).
func (m *Manager) PreviewIRI(localName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.previewIRILocked(localName)
}

// --- internals ---

// queueFile is the persisted shape of {dataDir}/proposals.json.
type queueFile struct {
	NextID int        `json:"next_id"`
	Items  []Proposal `json:"items"`
}

func (m *Manager) loadQueue() error {
	data, err := os.ReadFile(filepath.Join(m.dataDir, "proposals.json"))
	if err != nil {
		if os.IsNotExist(err) {
			m.nextID = 1
			return nil
		}
		return fmt.Errorf("induction: read proposals.json: %w", err)
	}
	var qf queueFile
	if err := json.Unmarshal(data, &qf); err != nil {
		return fmt.Errorf("induction: unmarshal proposals.json: %w", err)
	}
	m.queue = qf.Items
	m.nextID = qf.NextID
	if m.nextID < 1 {
		m.nextID = 1
	}
	return nil
}

func (m *Manager) persistLocked() error {
	qf := queueFile{NextID: m.nextID, Items: m.queue}
	data, err := json.MarshalIndent(qf, "", "  ")
	if err != nil {
		return fmt.Errorf("induction: marshal proposals.json: %w", err)
	}
	return writeFileAtomic(filepath.Join(m.dataDir, "proposals.json"), data)
}

func (m *Manager) allocID() string {
	id := fmt.Sprintf("prop-%04d", m.nextID)
	m.nextID++
	return id
}

func (m *Manager) currentTTLPath() string {
	return filepath.Join(m.dataDir, "skillforge_tbox.ttl")
}

func (m *Manager) readVersion() int {
	data, err := os.ReadFile(filepath.Join(m.dataDir, "tbox_version.txt"))
	if err != nil {
		return 1
	}
	var v int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &v); err != nil || v < 1 {
		return 1
	}
	return v
}

// previewIRILocked checks localName against the current TBox text and the
// queued local names.
func (m *Manager) previewIRILocked(localName string) error {
	if localName == "" {
		return errors.New("induction: local name is empty")
	}
	if data, err := os.ReadFile(m.currentTTLPath()); err == nil {
		if strings.Contains(string(data), "skf:"+localName+" ") ||
			strings.Contains(string(data), fmt.Sprintf("%s%s", m.ns, localName)) {
			return fmt.Errorf("%w: skf:%s", ErrIRIConflict, localName)
		}
	}
	for _, p := range m.queue {
		if p.LocalName == localName && p.Status != StatusRejected {
			return fmt.Errorf("%w: queued proposal %s uses skf:%s", ErrIRIConflict, p.ID, localName)
		}
	}
	return nil
}

// uniqueLocalName derives a free PascalCase name from the English description.
func (m *Manager) uniqueLocalName(descriptionEN string) (string, error) {
	base := ToPascalCase(descriptionEN)
	if base == "" {
		return "", errors.New("induction: cannot derive local name from empty description")
	}
	name := base
	for n := 2; ; n++ {
		if err := m.previewIRILocked(name); err == nil {
			return name, nil
		}
		name = fmt.Sprintf("%s%d", base, n)
		if n > 100 {
			return "", errors.New("induction: cannot find free local name")
		}
	}
}

// ToPascalCase converts a natural-language description to a PascalCase local
// name: split on non-alphanumeric runs, title-case each word, join. Short
// all-caps words (acronyms like SQL, ETL, KPI) are kept verbatim
// (D-Induct-02 deterministic algorithm).
func ToPascalCase(s string) string {
	var words []string
	for _, w := range strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) {
		if w == "" {
			continue
		}
		if isAcronym(w) {
			words = append(words, w)
			continue
		}
		words = append(words, strings.ToUpper(w[:1])+strings.ToLower(w[1:]))
	}
	return strings.Join(words, "")
}

// isAcronym reports an all-uppercase word of at most 5 letters (SQL, ETL,
// KPI, PII — but not "NOTE" which is a shouted ordinary word we still
// canonicalize; the 5-letter cap keeps that distinction pragmatic).
func isAcronym(w string) bool {
	if len(w) > 5 {
		return false
	}
	for _, r := range w {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return len(w) > 1
}

// proposedTriples renders the Turtle lines an approve appends. Every element
// carries a bilingual rdfs:label (D-Induct-01: @zh + @en mandatory).
func proposedTriples(ns string, kind ProposalKind, local, descZH, descEN string) []string {
	typeLine := map[ProposalKind]string{
		KindClass:            "owl:Class",
		KindObjectProperty:   "owl:ObjectProperty",
		KindDatatypeProperty: "owl:DatatypeProperty",
	}[kind]
	if typeLine == "" {
		typeLine = "owl:Class"
	}
	// Avoid re-declaring the skf: prefix inside appended lines; emit full IRIs
	// with angle brackets so the lines are valid regardless of prefix state.
	iri := fmt.Sprintf("<%s%s>", ns, local)
	return []string{
		fmt.Sprintf("%s a %s ;", iri, typeLine),
		fmt.Sprintf(`    rdfs:label "%s"@zh , "%s"@en .`, escapeTurtle(descZH), escapeTurtle(descEN)),
	}
}

// parseTripleLine parses the simple two-line appended shape into a Triple.
// It is best-effort: unparsable lines are skipped (never panic).
func parseTripleLine(line, ns string) (store.Triple, bool) {
	s := strings.TrimSpace(line)
	s = strings.TrimSuffix(s, ";")
	s = strings.TrimSuffix(s, ".")
	s = strings.TrimSpace(s)
	// WR-03: skip the rdfs:label line emitted by proposedTriples. Text-
	// splitting it on " a " produced a garbage type triple whenever the
	// bilingual description contained " a " (e.g. "representing a business
	// concept" — extremely common in single-sentence LLM output), and that
	// garbage was kg.Insert'ed and persisted via snapshots.
	if strings.HasPrefix(s, "rdfs:label ") {
		return store.Triple{}, false
	}
	parts := strings.SplitN(s, " a ", 2)
	if len(parts) != 2 {
		return store.Triple{}, false
	}
	subj := strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(parts[0]), ">"), "<")
	obj := strings.TrimSpace(parts[1])
	if obj == "owl:Class" {
		obj = "http://www.w3.org/2002/07/owl#Class"
	} else if obj == "owl:ObjectProperty" {
		obj = "http://www.w3.org/2002/07/owl#ObjectProperty"
	} else if obj == "owl:DatatypeProperty" {
		obj = "http://www.w3.org/2002/07/owl#DatatypeProperty"
	}
	return store.Triple{
		S: subj,
		P: "http://www.w3.org/1999/02/22-rdf-syntax-ns#type",
		O: obj,
	}, subj != "" && obj != ""
}

// extractJSON pulls the first {...} block out of an LLM response (defensive
// against fences/prose, same contract as internal/llm.ExtractJSON but kept
// local to avoid the package dependency edge).
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	var esc bool
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

func escapeTurtle(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// writeFileAtomic writes via temp + rename (mirrors store.atomicWriteFile).
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".induction-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// ApprovedTriples returns the ProposedTriples lines of every APPROVED
// proposal persisted in {dataDir}/proposals.json, in queue file order.
//
// This is the replay source for the Phase 39 TBox upgrade (emitter.
// UpgradeIfNeeded, Plan 39-06 / RESEARCH Pitfall 4): emitting a fresh 8-class
// TBox must not silently drop triples a human approved under the previous
// version, so the upgrader appends these lines to the fresh TTL text. The
// helper lives HERE — not in the emitter — because proposals.json is this
// package's persistence format and the queue shape must have exactly one
// owner. A missing queue file is not an error (nil, nil): environments that
// never used induction have nothing to replay.
func ApprovedTriples(dataDir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, "proposals.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("induction: read proposals.json: %w", err)
	}
	var qf queueFile
	if err := json.Unmarshal(data, &qf); err != nil {
		return nil, fmt.Errorf("induction: unmarshal proposals.json: %w", err)
	}
	var out []string
	for _, p := range qf.Items {
		if p.Status == StatusApproved {
			out = append(out, p.ProposedTriples...)
		}
	}
	return out, nil
}

// nonWordRE is retained for callers that need to sanity-check identifiers.
var nonWordRE = regexp.MustCompile(`[^A-Za-z0-9]+`)
