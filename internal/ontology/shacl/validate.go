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

// Package shacl implements a pure-Go SHACL validation engine.
//
// D-SHACL-01: Zero third-party Go SHACL library dependencies. Shapes are
// expressed in standard SHACL Turtle syntax and parsed via goRDFlib's
// turtle.Parse — the same library used for TBox loading in store.go.
// Seven constraint types are implemented via switch cases: sh:minCount,
// sh:maxCount, sh:class, sh:datatype, sh:nodeKind, sh:pattern, sh:in.
// Adding a new constraint type requires only a new case branch.
//
// D-SHACL-02: Violations are split into error (structural / quarantined)
// and warning (optional property missing / provenance-missing). Error
// entities are added to Quarantined and excluded from the main KG index.
// Warning entities are allowed through with violations recorded.
//
// D-46-01: sh:severity is data, not engine policy. A PropertyShape declaring
// sh:severity sh:Warning or sh:Info yields warning-level violations (allowed
// through); sh:Violation, any unknown IRI, or no declaration at all yields
// error-level violations (W3C SHACL §2.1.4: "sh:Violation is the default if
// sh:severity is unspecified"). Defaulting to error keeps the pre-existing
// TBox shapes behaviour byte-identical; the former sourcePath→warning engine
// special case (D-PROV-10) retired into explicit sh:severity declarations in
// shapes.ttl.
//
// D-46-03: every violation is stamped with the layer it was validated
// against ("tbox" | "entity") plus the target class of the NodeShape that
// produced it, both copied deterministically from parse-time shape metadata
// in the Validate stamping loop. Cross-entity integrity violations
// (checkGlobalReferences) carry LayerGlobal and no target class — they
// belong to no single NodeShape class, and the absent target_class
// (omitempty) is the FE sentinel for the dashed "cross-entity" badge
// (46-UI-SPEC §Component 2).
//
// D-46-04: quarantine stays subject-only (store.Quarantine is keyed by
// t.S — SC2 formal semantics: a quarantined entity is invisible in subject
// position, while its incoming edges remain visible in object position and
// are disclosed honestly via per-entity dangling_edge_count). Cascading
// quarantine was rejected: Quarantine is subject-keyed and snapshots
// serialize only the main store, so cascade would corrupt healthy notes and
// permanently persist the deletion.
package shacl

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"

	"github.com/skillforge/skillforge/internal/ontology/store"
)

// ViolationSeverity categorises a constraint violation.
type ViolationSeverity string

const (
	SeverityError   ViolationSeverity = "error"
	SeverityWarning ViolationSeverity = "warning"
)

// Layer classification stamped on nodeShape at parse time (D-46-02):
// source-file tracking, zero class-name hard-coding. The enum strings are
// locked by the 46-UI-SPEC ViolationLayer type.
const (
	LayerTBox   = "tbox"   // shapes from any file other than entities.ttl
	LayerEntity = "entity" // shapes from entities.ttl (ABox entities)
	LayerGlobal = "global" // cross-entity integrity violations (checkGlobalReferences)
)

// Violation records one SHACL constraint violation.
type Violation struct {
	Severity   ViolationSeverity `json:"severity"`
	Entity     string            `json:"entity"`
	Constraint string            `json:"constraint"`
	Detail     string            `json:"detail"`
	SourcePath string            `json:"source_path,omitempty"`
	LineStart  int               `json:"line_start,omitempty"`
	LineEnd    int               `json:"line_end,omitempty"`
	// D-46-03: layer/target_class are stamped in the Validate loop from
	// parse-time nodeShape metadata (deterministic derivation, testable).
	// omitempty keeps legacy reports wire-compatible; LayerGlobal violations
	// carry no TargetClass — the missing field IS the FE sentinel
	// (46-UI-SPEC §Component 2).
	TargetClass string `json:"target_class,omitempty"`
	Layer       string `json:"layer,omitempty"`
}

// QuarantinedEntity is an entity that has at least one error-level violation.
type QuarantinedEntity struct {
	Entity      string `json:"entity"`
	TripleCount int    `json:"triple_count"`
	// D-46-04: incoming-edge count at validation time — counted BEFORE any
	// quarantine happens (the store indices still hold the full graph, A4).
	// No omitempty: the BE must always ship this field so a count of 0
	// reaches the FE and renders as 0, never as "—" (46-UI-SPEC §Component 4
	// Honesty red line 8).
	DanglingEdgeCount int         `json:"dangling_edge_count"`
	Violations        []Violation `json:"violations"`
}

// Report is the output of a SHACL validation run.
type Report struct {
	Health       string            `json:"health"`
	ErrorCount   int               `json:"error_count"`
	WarningCount int               `json:"warning_count"`
	Warnings     []Violation       `json:"warnings"`
	Quarantined  []QuarantinedEntity `json:"quarantined"`
	TripleCount  int               `json:"triple_count"`
	ShapesPath   string            `json:"shapes_path"`
	ValidatedAt  string            `json:"validated_at"`
	Trigger      string            `json:"trigger"`
}

// Validate runs full SHACL validation over the given KG store using shapes
// parsed from the given shapes path list (variadic — single-path callers
// compile unchanged). Returns a Report or an error.
//
// Health derivation (D-SHACL-03 three-colour mapping):
//   - "error"  if any error-level violations exist (at least one entity quarantined)
//   - "warn"   if only warning-level violations exist
//   - "ok"     if zero violations
func Validate(kg *store.Store, shapesPaths ...string) (*Report, error) {
	return ValidateWithTrigger(kg, shapesPaths, "boot")
}

// ValidateWithTrigger is the same as Validate but allows the caller to
// specify the trigger context ("boot" or "rollback").
//
// D-46-02: shapesPaths is a path list. Every path is parsed and merged into
// one shape set; if ANY path fails to read/parse the whole validation fails
// (errors.Join aggregates all failures — a half-populated shape set silently
// skipping missing paths would validate against incomplete rules). The
// single production read point for the path list is ShapesPathsFromEnv.
func ValidateWithTrigger(kg *store.Store, shapesPaths []string, trigger string) (*Report, error) {
	parsed := &parsedShapes{}
	var parseErrs []error
	for _, path := range shapesPaths {
		fileParsed, err := parseShapes(path)
		if err != nil {
			parseErrs = append(parseErrs, err)
			continue
		}
		// D-46-02: layer is derived at parse time from the source file name —
		// entities.ttl → entity, everything else → tbox (user decision
		// 2026-10-04: no TargetClass-name hard-coding).
		layer := LayerTBox
		if filepath.Base(path) == "entities.ttl" {
			layer = LayerEntity
		}
		for i := range fileParsed.NodeShapes {
			fileParsed.NodeShapes[i].Layer = layer
		}
		parsed.NodeShapes = append(parsed.NodeShapes, fileParsed.NodeShapes...)
		parsed.GlobalRefs = append(parsed.GlobalRefs, fileParsed.GlobalRefs...)
		parsed.GlobalBelongsTo = append(parsed.GlobalBelongsTo, fileParsed.GlobalBelongsTo...)
		parsed.GlobalHasRead = append(parsed.GlobalHasRead, fileParsed.GlobalHasRead...)
		parsed.GlobalHasWrite = append(parsed.GlobalHasWrite, fileParsed.GlobalHasWrite...)
		parsed.GlobalPoweredBy = append(parsed.GlobalPoweredBy, fileParsed.GlobalPoweredBy...)
		parsed.GlobalRequires = append(parsed.GlobalRequires, fileParsed.GlobalRequires...)
		parsed.GlobalHasSubCat = append(parsed.GlobalHasSubCat, fileParsed.GlobalHasSubCat...)
		parsed.GlobalBelongsNote = append(parsed.GlobalBelongsNote, fileParsed.GlobalBelongsNote...)
	}
	if len(parseErrs) > 0 {
		return nil, fmt.Errorf("shacl: parse shapes: %w", errors.Join(parseErrs...))
	}

	report := &Report{
		// D-46-02: join with the OS path-list separator — the dual of
		// filepath.SplitList (single path → no separator, unchanged legacy
		// display; FE type stays a single string).
		ShapesPath:  strings.Join(shapesPaths, string(filepath.ListSeparator)),
		ValidatedAt: storeNow(),
		Trigger:     trigger,
	}

	// Collect all triples for triple-count reporting.
	report.TripleCount = kg.Len()

	// Collect all quarantined entities and warnings.
	entityErrors := map[string][]Violation{} // entity → violations (errors only)
	entityWarnings := map[string][]Violation{} // entity → violations (warnings only)

	// Walk each NodeShape.
	for _, ns := range parsed.NodeShapes {
		// Collect all individuals of the target class.
		individuals := collectIndividuals(kg, ns.TargetClass)

		for _, entity := range individuals {
			for _, prop := range ns.Properties {
				violations := evaluateProperty(kg, entity, prop, parsed)
				for _, v := range violations {
					v.Entity = entity
					// D-46-03: stamp layer + target class from the parse-time
					// nodeShape record (the loop variable ns shadows the ns()
					// namespace helper in this scope — existing loop idiom).
					v.TargetClass = ns.TargetClass
					v.Layer = ns.Layer
					if v.Severity == SeverityError {
						entityErrors[entity] = append(entityErrors[entity], v)
					} else {
						entityWarnings[entity] = append(entityWarnings[entity], v)
					}
				}
			}
		}
	}

	// Global reference integrity: every skf:NoteChunk with skf:referencesTable
	// must point to an existing skf:TableSchema.
	globalRefs := checkGlobalReferences(kg, parsed)
	for _, v := range globalRefs {
		if v.Severity == SeverityError {
			entityErrors[v.Entity] = append(entityErrors[v.Entity], v)
		} else {
			entityWarnings[v.Entity] = append(entityWarnings[v.Entity], v)
		}
	}

	// Build quarantined list from entities that have at least one error.
	for entity, violations := range entityErrors {
		tripleCount := len(kg.Query(entity, "", ""))
		report.Quarantined = append(report.Quarantined, QuarantinedEntity{
			Entity:      entity,
			TripleCount: tripleCount,
			// D-46-04: counted during validation, BEFORE any quarantine —
			// the store indices still hold the full graph here. SC2 formal
			// semantics: after the caller quarantines (subject-keyed), the
			// entity vanishes from subject position while these incoming
			// edges stay visible in object position and this count
			// discloses them honestly.
			DanglingEdgeCount: countDanglingEdges(kg, entity),
			Violations:        violations,
		})
		report.ErrorCount += len(violations)
	}

	// Collect pure warnings (entities with only warnings, no errors).
	for entity, violations := range entityWarnings {
		if _, hasError := entityErrors[entity]; !hasError {
			report.Warnings = append(report.Warnings, violations...)
			report.WarningCount += len(violations)
		}
	}

	// Derive health.
	switch {
	case report.ErrorCount > 0:
		report.Health = "error"
	case report.WarningCount > 0:
		report.Health = "warn"
	default:
		report.Health = "ok"
	}

	return report, nil
}

// ShapesPathsFromEnv resolves the SHACL shapes path list from the
// SKILLFORGE_SHACL_SHAPES environment variable (D-46-02). This is the SINGLE
// read point for that env — boot (cmd/server) and rollback (induction) both
// consume it, so the two call sites can never drift apart.
//
// A non-empty value is split on the OS path-list separator
// (filepath.SplitList). An unset or empty value falls back to the default
// two-file list (shapes.ttl + entities.ttl). The emptiness check must come
// first: filepath.SplitList("") returns an empty slice (not [""]) and an
// empty list would silently validate against zero shapes.
func ShapesPathsFromEnv() []string {
	if v := os.Getenv("SKILLFORGE_SHACL_SHAPES"); v != "" {
		return filepath.SplitList(v)
	}
	return []string{
		filepath.Join("internal", "ontology", "shacl", "shapes.ttl"),
		filepath.Join("internal", "ontology", "shacl", "entities.ttl"),
	}
}

// tripleRec is a local triple record used during shape parsing.
type tripleRec struct {
	S, P, O string
}

// parsedShapes holds the in-memory representation of a SHACL shapes file.
type parsedShapes struct {
	NodeShapes        []nodeShape
	GlobalRefs        []propertyConstraint // skf:referencesTable → TableSchema
	GlobalBelongsTo   []propertyConstraint // skf:belongsToTier → Tier
	GlobalHasRead     []propertyConstraint
	GlobalHasWrite    []propertyConstraint
	GlobalPoweredBy   []propertyConstraint
	GlobalRequires   []propertyConstraint
	GlobalHasSubCat   []propertyConstraint
	GlobalBelongsNote []propertyConstraint
}

type nodeShape struct {
	TargetClass string
	Properties  []propertyConstraint
	Layer       string // D-46-02: parse-time source-file tracking (LayerTBox | LayerEntity)
}

type propertyConstraint struct {
	Path       string // property IRI (e.g. skf:categoryName)
	MinCount   *int
	MaxCount   *int
	Class      string // sh:class target IRI
	Datatype   string // XSD datatype IRI
	NodeKind   string // sh:nodeKind (IRI | BlankNode | Literal)
	Pattern    string
	In         []string // sh:in enumeration values
	HasValue   string   // sh:hasValue
	Message    string
	Severity   ViolationSeverity // derived from context; provenance-missing → warning
}

// parseShapes reads and parses a SHACL TTL file via goRDFlib.
func parseShapes(path string) (*parsedShapes, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read shapes file: %w", err)
	}

	g := graph.NewGraph()
	if err := turtle.Parse(g, strings.NewReader(string(data))); err != nil {
		return nil, fmt.Errorf("turtle parse: %w", err)
	}

	parsed := &parsedShapes{}

	// Walk all triples to build shape structures.
	// We collect sh:NodeShape subjects and their sh:property sh:PropertyShape blank nodes.

	// Build a map of blank-node PropertyShapes by their parent NodeShape.
	// Strategy: collect all (subject, predicate, object) triples and
	// assemble structures in-memory.
	var triplesList []tripleRec
	for tr := range g.Triples(nil, nil, nil) {
		triplesList = append(triplesList, tripleRec{
			S: subjectStr(tr.Subject),
			P: tr.Predicate.Value(),
			O: objectStr(tr.Object),
		})
	}

	// Index triples by subject.
	byS := map[string][]tripleRec{}
	for _, tr := range triplesList {
		byS[tr.S] = append(byS[tr.S], tr)
	}

	// Find all sh:NodeShape subjects.
	for _, subs := range byS {
		isNodeShape := false
		for _, tr := range subs {
			if tr.P == ns("type") && tr.O == sh("NodeShape") {
				isNodeShape = true
				break
			}
		}
		if !isNodeShape {
			continue
		}

		ns := nodeShape{}

		// Extract targetClass.
		for _, tr := range subs {
			if tr.P == sh("targetClass") {
				ns.TargetClass = tr.O
				break
			}
		}

		// Extract property references (sh:property → blank node or IRI).
		for _, tr := range subs {
			if tr.P != sh("property") {
				continue
			}
			propRef := tr.O // blank node ID or IRI
			constraint := parsePropertyShape(byS, propRef)
			if constraint.Path != "" {
				ns.Properties = append(ns.Properties, constraint)
			}
		}

		parsed.NodeShapes = append(parsed.NodeShapes, ns)
	}

	// Extract global property shapes (PropertyShapes attached directly, not via NodeShape).
	// These encode cross-entity integrity rules like skf:referencesTable → skf:TableSchema.
	for s, subs := range byS {
		isPropShape := false
		for _, tr := range subs {
			if tr.P == ns("type") && tr.O == sh("PropertyShape") {
				isPropShape = true
				break
			}
		}
		if !isPropShape {
			continue
		}

		constraint := parsePropertyShape(byS, s)
		if constraint.Path == "" {
			continue
		}

		// Class-based property shapes (global integrity).
		switch constraint.Path {
		case pf("referencesTable"):
			parsed.GlobalRefs = append(parsed.GlobalRefs, constraint)
		case pf("belongsToTier"):
			parsed.GlobalBelongsTo = append(parsed.GlobalBelongsTo, constraint)
		case pf("hasReadAccess"):
			parsed.GlobalHasRead = append(parsed.GlobalHasRead, constraint)
		case pf("hasWriteAccess"):
			parsed.GlobalHasWrite = append(parsed.GlobalHasWrite, constraint)
		case pf("poweredBy"):
			parsed.GlobalPoweredBy = append(parsed.GlobalPoweredBy, constraint)
		case pf("requiresRefusal"):
			parsed.GlobalRequires = append(parsed.GlobalRequires, constraint)
		case pf("hasSubCategory"):
			parsed.GlobalHasSubCat = append(parsed.GlobalHasSubCat, constraint)
		case pf("belongsToNote"):
			parsed.GlobalBelongsNote = append(parsed.GlobalBelongsNote, constraint)
		}
	}

	return parsed, nil
}

func parsePropertyShape(byS map[string][]tripleRec, subject string) propertyConstraint {
	subs := byS[subject]
	if len(subs) == 0 {
		return propertyConstraint{}
	}

	c := propertyConstraint{}

	// Extract sh:path (required).
	for _, tr := range subs {
		if tr.P == sh("path") {
			c.Path = expandPrefixed(tr.O)
			break
		}
	}
	if c.Path == "" {
		// No path — skip.
		return propertyConstraint{}
	}

	// sh:minCount
	for _, tr := range subs {
		if tr.P == sh("minCount") {
			if v := parseInt(tr.O); v > 0 {
				c.MinCount = &v
			}
			break
		}
	}

	// sh:maxCount
	for _, tr := range subs {
		if tr.P == sh("maxCount") {
			if v := parseInt(tr.O); v > 0 {
				c.MaxCount = &v
			}
			break
		}
	}

	// sh:class
	for _, tr := range subs {
		if tr.P == sh("class") {
			c.Class = expandPrefixed(tr.O)
			break
		}
	}

	// sh:datatype
	for _, tr := range subs {
		if tr.P == sh("datatype") {
			c.Datatype = expandPrefixed(tr.O)
			break
		}
	}

	// sh:nodeKind
	for _, tr := range subs {
		if tr.P == sh("nodeKind") {
			c.NodeKind = expandPrefixed(tr.O)
			break
		}
	}

	// sh:pattern
	for _, tr := range subs {
		if tr.P == sh("pattern") {
			c.Pattern = strings.Trim(tr.O, `"`)
			break
		}
	}

	// sh:in (list of values).
	for _, tr := range subs {
		if tr.P == sh("in") {
			c.In = append(c.In, expandPrefixed(tr.O))
		}
	}

	// sh:hasValue
	for _, tr := range subs {
		if tr.P == sh("hasValue") {
			c.HasValue = expandPrefixed(tr.O)
			break
		}
	}

	// sh:message
	for _, tr := range subs {
		if tr.P == sh("message") {
			c.Message = strings.Trim(tr.O, `"`)
			break
		}
	}

	// D-46-01: sh:severity is parsed from the shapes data, not derived by the
	// engine (W3C SHACL §2.1.4: "sh:Violation is the default if sh:severity
	// is unspecified"). sh:Warning / sh:Info → warning-level (allowed through,
	// recorded); sh:Violation, any unknown IRI, or an unspecified value →
	// error-level (quarantined). Severity is read at PropertyShape granularity
	// only — the engine does not read NodeShape-level sh:severity.
	// D-PROV-10 semantics are preserved: the sourcePath→warning policy moved
	// from this (retired) hard-coded engine special case into the shapes data
	// (shapes.ttl declares sh:severity sh:Warning on both sourcePath shapes).
	c.Severity = SeverityError
	for _, tr := range subs {
		if tr.P == sh("severity") {
			if sev := expandPrefixed(tr.O); sev == sh("Warning") || sev == sh("Info") {
				c.Severity = SeverityWarning
			}
			break
		}
	}

	return c
}

// collectIndividuals returns all KG entities that are rdf:type of the given classIRI.
func collectIndividuals(kg *store.Store, classIRI string) []string {
	triples := kg.Query("", ns("type"), classIRI)
	out := make([]string, 0, len(triples))
	seen := map[string]bool{}
	for _, tr := range triples {
		if !seen[tr.S] {
			seen[tr.S] = true
			out = append(out, tr.S)
		}
	}
	return out
}

// evaluateProperty evaluates all constraints of one property shape against one entity.
// Returns zero or more violations.
func evaluateProperty(kg *store.Store, entity string, c propertyConstraint, parsed *parsedShapes) []Violation {
	var violations []Violation

	// Collect all values for this property on this entity.
	values := kg.Query(entity, c.Path, "")

	// Evaluate each constraint type.
	if c.MinCount != nil {
		violations = append(violations, checkMinCount(entity, c, len(values))...)
	}
	if c.MaxCount != nil {
		violations = append(violations, checkMaxCount(entity, c, len(values))...)
	}
	if c.Class != "" {
		for _, val := range values {
			violations = append(violations, checkClass(kg, entity, c, val.O)...)
		}
	}
	if c.Datatype != "" {
		for _, val := range values {
			violations = append(violations, checkDatatype(entity, c, val.O)...)
		}
	}
	if c.NodeKind != "" {
		for _, val := range values {
			violations = append(violations, checkNodeKind(entity, c, val.O)...)
		}
	}
	if c.Pattern != "" {
		for _, val := range values {
			violations = append(violations, checkPattern(entity, c, val.O)...)
		}
	}
	if len(c.In) > 0 {
		for _, val := range values {
			violations = append(violations, checkIn(entity, c, val.O)...)
		}
	}
	if c.HasValue != "" {
		violations = append(violations, checkHasValue(entity, c, values)...)
	}

	return violations
}

// checkMinCount fires when the number of property values is below minCount.
func checkMinCount(entity string, c propertyConstraint, actualCount int) []Violation {
	if c.MinCount != nil && actualCount < *c.MinCount {
		sev := c.Severity
		return []Violation{{
			Severity:   sev,
			Entity:     entity,
			Constraint: "sh:minCount",
			Detail:     fmt.Sprintf("property %s has %d value(s), minimum required is %d", c.Path, actualCount, *c.MinCount),
		}}
	}
	return nil
}

// checkMaxCount fires when the number of property values exceeds maxCount.
func checkMaxCount(entity string, c propertyConstraint, actualCount int) []Violation {
	if c.MaxCount != nil && actualCount > *c.MaxCount {
		return []Violation{{
			Severity:   c.Severity,
			Entity:     entity,
			Constraint: "sh:maxCount",
			Detail:     fmt.Sprintf("property %s has %d value(s), maximum allowed is %d", c.Path, actualCount, *c.MaxCount),
		}}
	}
	return nil
}

// checkClass fires when a property value is not an instance of the required class.
func checkClass(kg *store.Store, entity string, c propertyConstraint, value string) []Violation {
	// Check if value is rdf:type of the required class.
	triples := kg.Query(value, ns("type"), c.Class)
	if len(triples) == 0 {
		return []Violation{{
			Severity:   c.Severity,
			Entity:     entity,
			Constraint: "sh:class",
			Detail:     fmt.Sprintf("value <%s> of property %s is not a %s", value, c.Path, c.Class),
		}}
	}
	return nil
}

// checkDatatype fires when a literal value does not match the required XSD datatype.
func checkDatatype(entity string, c propertyConstraint, value string) []Violation {
	// value is a literal string like `"some text"`.
	lit := strings.Trim(value, `"`)
	if lit == value {
		// Not a literal — was an IRI. If datatype is required, this is a violation.
		return []Violation{{
			Severity:   c.Severity,
			Entity:     entity,
			Constraint: "sh:datatype",
			Detail:     fmt.Sprintf("value <%s> of property %s is not a literal (expected %s)", value, c.Path, c.Datatype),
		}}
	}
	// Validate XSD datatype.
	dt := c.Datatype
	if strings.HasSuffix(dt, "string") || dt == "" {
		// xsd:string — all strings are valid.
		return nil
	}
	if strings.HasSuffix(dt, "integer") || strings.HasSuffix(dt, "int") || strings.HasSuffix(dt, "long") || strings.HasSuffix(dt, "short") || strings.HasSuffix(dt, "byte") {
		if _, err := parseInteger(lit); err != nil {
			return []Violation{{
				Severity:   c.Severity,
				Entity:     entity,
				Constraint: "sh:datatype",
				Detail:     fmt.Sprintf("value '%s' of property %s is not a valid %s", lit, c.Path, dt),
			}}
		}
		return nil
	}
	if strings.HasSuffix(dt, "boolean") || strings.HasSuffix(dt, "bool") {
		if lit != "true" && lit != "false" && lit != "1" && lit != "0" {
			return []Violation{{
				Severity:   c.Severity,
				Entity:     entity,
				Constraint: "sh:datatype",
				Detail:     fmt.Sprintf("value '%s' of property %s is not a valid %s", lit, c.Path, dt),
			}}
		}
		return nil
	}
	if strings.HasSuffix(dt, "decimal") || strings.HasSuffix(dt, "float") || strings.HasSuffix(dt, "double") {
		if _, err := parseFloat(lit); err != nil {
			return []Violation{{
				Severity:   c.Severity,
				Entity:     entity,
				Constraint: "sh:datatype",
				Detail:     fmt.Sprintf("value '%s' of property %s is not a valid %s", lit, c.Path, dt),
			}}
		}
		return nil
	}
	// Unknown datatype — allow.
	return nil
}

// checkNodeKind fires when the node kind of a value does not match the expected kind.
func checkNodeKind(entity string, c propertyConstraint, value string) []Violation {
	isIRI := strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "skf:") || strings.Contains(value, ":")
	isBlank := strings.HasPrefix(value, "_:")
	isLiteral := strings.HasPrefix(value, `"`)

	var expectedKind string
	var valid bool
	switch c.NodeKind {
	case sh("IRI"):
		expectedKind = "IRI"
		valid = isIRI
	case sh("BlankNode"):
		expectedKind = "BlankNode"
		valid = isBlank
	case sh("Literal"):
		expectedKind = "Literal"
		valid = isLiteral
	case sh("IRIOrLiteral"):
		expectedKind = "IRIOrLiteral"
		valid = isIRI || isLiteral
	default:
		return nil
	}
	if !valid {
		return []Violation{{
			Severity:   c.Severity,
			Entity:     entity,
			Constraint: "sh:nodeKind",
			Detail:     fmt.Sprintf("value '%s' of property %s is not a %s", value, c.Path, expectedKind),
		}}
	}
	return nil
}

// checkPattern fires when a string value does not match the required regex pattern.
func checkPattern(entity string, c propertyConstraint, value string) []Violation {
	if c.Pattern == "" {
		return nil
	}
	lit := strings.Trim(value, `"`)
	if lit == value {
		// Not a literal — pattern doesn't apply.
		return nil
	}
	matched, err := regexp.MatchString(c.Pattern, lit)
	if err != nil {
		return nil // Invalid regex — skip.
	}
	if !matched {
		return []Violation{{
			Severity:   c.Severity,
			Entity:     entity,
			Constraint: "sh:pattern",
			Detail:     fmt.Sprintf("value '%s' of property %s does not match pattern '%s'", lit, c.Path, c.Pattern),
		}}
	}
	return nil
}

// checkIn fires when a value is not in the permitted enumeration list.
func checkIn(entity string, c propertyConstraint, value string) []Violation {
	if len(c.In) == 0 {
		return nil
	}
	lit := strings.Trim(value, `"`)
	for _, permitted := range c.In {
		if permitted == value || strings.Trim(permitted, `"`) == lit {
			return nil
		}
	}
	return []Violation{{
		Severity:   c.Severity,
		Entity:     entity,
		Constraint: "sh:in",
		Detail:     fmt.Sprintf("value '%s' of property %s is not in enumeration %v", value, c.Path, c.In),
	}}
}

// checkHasValue fires when no property value equals the required hasValue.
func checkHasValue(entity string, c propertyConstraint, values []store.Triple) []Violation {
	for _, val := range values {
		if val.O == c.HasValue {
			return nil
		}
	}
	return []Violation{{
		Severity:   c.Severity,
		Entity:     entity,
		Constraint: "sh:hasValue",
		Detail:     fmt.Sprintf("property %s has no value equal to '%s'", c.Path, c.HasValue),
	}}
}

// checkGlobalReferences enforces cross-entity integrity:
// every skf:NoteChunk with skf:referencesTable must point to an existing skf:TableSchema.
func checkGlobalReferences(kg *store.Store, parsed *parsedShapes) []Violation {
	var violations []Violation

	// Get all NoteChunks with skf:referencesTable.
	noteRefs := kg.Query("", pf("referencesTable"), "")
	seenRef := map[string]bool{}
	for _, tr := range noteRefs {
		if seenRef[tr.S] {
			continue
		}
		seenRef[tr.S] = true
		target := tr.O
		// Check that target exists as a skf:TableSchema.
		if !existsType(kg, target, ns("type"), pf("TableSchema")) {
			violations = append(violations, Violation{
				Severity:   SeverityError,
				Entity:     tr.S,
				Constraint: "sh:class",
				// D-46-03 global sentinel: cross-entity integrity violations
				// belong to no single NodeShape class — LayerGlobal is set
				// and TargetClass stays empty (omitempty drops it in JSON),
				// which is the FE's dashed "cross-entity" badge signal.
				Layer: LayerGlobal,
				Detail: fmt.Sprintf("skf:referencesTable target <%s> is not a skf:TableSchema instance", target),
			})
		}
	}

	return violations
}

// existsType checks whether an entity has a given rdf:type triple.
func existsType(kg *store.Store, entity, _, typeIRI string) bool {
	return len(kg.Query(entity, ns("type"), typeIRI)) > 0
}

// danglingEdgePredicates enumerates the ABox predicates whose edges can be
// left pointing at a quarantined entity (abox.go:66-82 predicate family,
// TBox v7 prop-0003/0004/0005).
var danglingEdgePredicates = []string{
	pf("involvesInsuranceProduct"),
	pf("involvesOrganization"),
	pf("parentOrganization"),
}

// countDanglingEdges counts incoming edges to entity across the dangling
// predicates (reverse lookup via the store's byO index — same idiom as
// store.Adjacent1Reverse). Called from the quarantine list build, i.e.
// during validation and BEFORE quarantine moves any triple (D-46-04).
func countDanglingEdges(kg *store.Store, entity string) int {
	n := 0
	for _, pred := range danglingEdgePredicates {
		n += len(kg.Query("", pred, entity))
	}
	return n
}

// --- Namespace helpers ---

func sh(rel string) string  { return "http://www.w3.org/ns/shacl#" + rel }
func ns(rel string) string { return "http://www.w3.org/1999/02/22-rdf-syntax-ns#" + rel }
func pf(rel string) string { return "http://skillforge.local/ontology#" + rel }

// expandPrefixed converts a prefixed IRI like "skf:categoryName" to its full URI.
// It handles the common prefixes used in SHACL shapes files.
func expandPrefixed(value string) string {
	if strings.HasPrefix(value, "skf:") {
		return pf(strings.TrimPrefix(value, "skf:"))
	}
	if strings.HasPrefix(value, "rdf:") {
		return ns(strings.TrimPrefix(value, "rdf:"))
	}
	if strings.HasPrefix(value, "xsd:") {
		return "http://www.w3.org/2001/XMLSchema#" + strings.TrimPrefix(value, "xsd:")
	}
	if strings.HasPrefix(value, "sh:") {
		return sh(strings.TrimPrefix(value, "sh:"))
	}
	if strings.HasPrefix(value, "rdfs:") {
		return "http://www.w3.org/2000/01/rdf-schema#" + strings.TrimPrefix(value, "rdfs:")
	}
	return value
}

func subjectStr(s term.Subject) string {
	switch v := s.(type) {
	case term.URIRef:
		return v.Value()
	case term.BNode:
		return "_:" + v.Value()
	default:
		return s.String()
	}
}

func objectStr(o term.Term) string {
	switch v := o.(type) {
	case term.URIRef:
		return v.Value()
	case term.Literal:
		if vs, ok := v.Value().(string); ok {
			return `"` + vs + `"`
		}
		return fmt.Sprintf("%v", v.Value())
	case term.BNode:
		return "_:" + v.Value()
	default:
		return o.String()
	}
}

func parseInt(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}

func parseInteger(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

func parseFloat(s string) (float64, error) {
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	return v, err
}

func storeNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}
