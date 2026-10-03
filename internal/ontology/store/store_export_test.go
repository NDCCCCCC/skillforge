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

package store

// Test-only export seam for the D-45-01 emitter conformance test
// (store_emitter_conformance_test.go, external package store_test).
//
// Why this file exists: the conformance test must import
// internal/ontology/emitter to render its live fixture, and the emitter
// package transitively imports store (emitter/upgrade.go → induction →
// store). An IN-PACKAGE test file cannot import emitter ("import cycle not
// allowed in test"). External test packages (store_test) MAY cycle through
// the package under test, and they see the package compiled with its
// in-package _test.go files — so these exported-for-test symbols give the
// external test exactly the unexported access the plan requires, while
// production builds (which never compile _test.go) see nothing.
//
// Production API surface is unchanged: every symbol here lives in a
// _test.go file.

// ConformanceNS is the ontology namespace const (ns).
const ConformanceNS = ns

// ConformanceRdfType is the rdf:type IRI const.
const ConformanceRdfType = rdfType

// ConformanceMetaPredicates returns the metaPredicates allowlist (D-45-01).
func ConformanceMetaPredicates() map[string]bool { return metaPredicates }

// ConformanceSchemaSubjectSet derives the schema-individual subject set —
// the SAME derivation QueryScoped Path B uses (D-45-02).
func ConformanceSchemaSubjectSet(s *Store) map[string]bool { return s.schemaSubjectSet() }

// ConformanceAll returns the store's full parsed triple list.
func ConformanceAll(s *Store) []Triple { return s.all }
