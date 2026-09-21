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

	"github.com/skillforge/skillforge/internal/ontology/store"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ErrIRIConflict reports a local-name collision against the current TBox.
var ErrIRIConflict = errors.New("induction: local name already exists in TBox")

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
	if data, err := os.ReadFile(current); err == nil {
		base = string(data)
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
	versioned := filepath.Join(m.dataDir, fmt.Sprintf("skillforge_tbox.v%d.ttl", next))
	if err := writeFileAtomic(versioned, []byte(newTTL)); err != nil {
		return *p, 0, fmt.Errorf("induction: write %s: %w", versioned, err)
	}
	if err := writeFileAtomic(current, []byte(newTTL)); err != nil {
		return *p, 0, fmt.Errorf("induction: refresh pointer %s: %w", current, err)
	}
	if err := writeFileAtomic(filepath.Join(m.dataDir, "tbox_version.txt"), []byte(fmt.Sprintf("%d\n", next))); err != nil {
		return *p, 0, fmt.Errorf("induction: write version file: %w", err)
	}

	// Insert new triples into the KG store so queries see them immediately.
	for _, t := range p.ProposedTriples {
		if trip, ok := parseTripleLine(t, m.ns); ok {
			m.kg.Insert(trip)
		}
	}

	now := time.Now().UTC()
	p.Status = StatusApproved
	p.DecidedAt = &now
	err := m.persistLocked()
	m.fireChange()
	return *p, next, err
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

// nonWordRE is retained for callers that need to sanity-check identifiers.
var nonWordRE = regexp.MustCompile(`[^A-Za-z0-9]+`)
