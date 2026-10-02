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

package induction

import (
	"errors"
	"time"
)

// ProposalKind enumerates the three schema element kinds the LLM may propose.
type ProposalKind string

const (
	KindClass            ProposalKind = "Class"
	KindObjectProperty   ProposalKind = "ObjectProperty"
	KindDatatypeProperty ProposalKind = "DatatypeProperty"
)

// ProposalStatus is the human-approval lifecycle state (ONT-04: nothing
// enters the TBox without an explicit approve).
type ProposalStatus string

const (
	StatusPending  ProposalStatus = "pending"
	StatusApproved ProposalStatus = "approved"
	StatusRejected ProposalStatus = "rejected"
)

// Proposal is one LLM-proposed schema element awaiting human approval.
type Proposal struct {
	ID              string         `json:"id"`
	Kind            ProposalKind   `json:"kind"`
	DescriptionZH   string         `json:"description_zh"`
	DescriptionEN   string         `json:"description_en"`
	LocalName       string         `json:"local_name"`       // PascalCase, backend-generated (D-Induct-01)
	ProposedIRI     string         `json:"proposed_iri"`     // {ns}{LocalName}
	ProposedTriples []string       `json:"proposed_triples"` // Turtle lines to append on approve
	Status          ProposalStatus `json:"status"`
	SourceText      string         `json:"source_text"`
	TargetSkill     string         `json:"target_skill,omitempty"`
	SourceChunkID   string         `json:"source_chunk_id,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	DecidedAt       *time.Time     `json:"decided_at,omitempty"`
}

// Validate enforces the required fields before a proposal can be persisted.
func (p Proposal) Validate() error {
	if p.ID == "" {
		return errors.New("induction: proposal ID is empty")
	}
	switch p.Kind {
	case KindClass, KindObjectProperty, KindDatatypeProperty:
	default:
		return errors.New("induction: proposal kind must be Class, ObjectProperty, or DatatypeProperty")
	}
	if p.DescriptionZH == "" || p.DescriptionEN == "" {
		return errors.New("induction: bilingual description is required (D-Induct-01)")
	}
	if p.LocalName == "" {
		return errors.New("induction: local name is empty")
	}
	if p.SourceText == "" {
		return errors.New("induction: source text is empty")
	}
	return nil
}
