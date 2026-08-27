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

package kpi

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// stubMetaSource implements DatasetMetaSource for hermetic testing.
type stubMetaSource struct {
	periodsFn func(ctx context.Context, dataset string) ([]string, error)
}

func (s *stubMetaSource) LatestDatasetVersion(ctx context.Context, dataset string) (DatasetVersion, error) {
	return DatasetVersion{}, nil
}

func (s *stubMetaSource) Periods(ctx context.Context, dataset string) ([]string, error) {
	return s.periodsFn(ctx, dataset)
}

func TestResolvePeriods_AllExist(t *testing.T) {
	meta := &stubMetaSource{
		periodsFn: func(ctx context.Context, dataset string) ([]string, error) {
			return []string{"2026-01", "2026-02", "2026-03", "2026-04", "2026-05"}, nil
		},
	}
	result, ref := ResolvePeriods(context.Background(), meta, "loans", []string{"2026-02", "2026-04", "2026-01"})
	assert.Nil(t, ref)
	assert.Equal(t, []string{"2026-01", "2026-02", "2026-04"}, result)
}

func TestResolvePeriods_MissingOne(t *testing.T) {
	meta := &stubMetaSource{
		periodsFn: func(ctx context.Context, dataset string) ([]string, error) {
			return []string{"2026-01", "2026-02", "2026-03"}, nil
		},
	}
	result, ref := ResolvePeriods(context.Background(), meta, "loans", []string{"2026-01", "2026-05", "2026-03"})
	assert.Nil(t, result)
	assert.NotNil(t, ref)
	assert.Equal(t, ReasonDatasetNotAvailable, ref.ReasonCode)
	assert.Contains(t, ref.HumanExplanation, "loans")
}

func TestResolvePeriods_Empty(t *testing.T) {
	meta := &stubMetaSource{
		periodsFn: func(ctx context.Context, dataset string) ([]string, error) {
			return []string{"2026-01", "2026-02"}, nil
		},
	}
	result, ref := ResolvePeriods(context.Background(), meta, "loans", nil)
	assert.Nil(t, ref)
	assert.Nil(t, result)

	result, ref = ResolvePeriods(context.Background(), meta, "loans", []string{})
	assert.Nil(t, ref)
	assert.Nil(t, result)
}

func TestResolvePeriods_Unsorted(t *testing.T) {
	meta := &stubMetaSource{
		periodsFn: func(ctx context.Context, dataset string) ([]string, error) {
			return []string{"2026-01", "2026-02", "2026-03", "2026-04", "2026-05", "2026-06", "2026-07", "2026-08"}, nil
		},
	}
	result, ref := ResolvePeriods(context.Background(), meta, "loans", []string{"2026-07", "2026-03", "2026-05"})
	assert.Nil(t, ref)
	assert.Equal(t, []string{"2026-03", "2026-05", "2026-07"}, result)
}

func TestResolvePeriods_PeriodsError(t *testing.T) {
	meta := &stubMetaSource{
		periodsFn: func(ctx context.Context, dataset string) ([]string, error) {
			return nil, errors.New("registry unavailable")
		},
	}
	result, ref := ResolvePeriods(context.Background(), meta, "loans", []string{"2026-01"})
	assert.Nil(t, result)
	assert.NotNil(t, ref)
	assert.Equal(t, ReasonDatasetNotAvailable, ref.ReasonCode)
}
