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

package data

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// ErrDatasetNotFound is returned by DeleteDataset when no registry row matches
// the requested name (either an exact composite id or a logical dataset name).
var ErrDatasetNotFound = errors.New("data: dataset not found")

// DeleteDataset removes one or more committed datasets. The name argument is
// interpreted as:
//   - an exact registry name containing "__"  → delete that single period;
//   - a logical dataset name without "__"    → delete every period of that
//     dataset.
//
// The delete order is Detach (release Windows file locks) → os.Remove →
// registry delete → INSERT-only dataset_deletes audit row. os.ErrNotExist is
// tolerated so retries converge.
func (imp *Importer) DeleteDataset(ctx context.Context, name string) ([]string, error) {
	if err := imp.guardSiteSalt(); err != nil {
		return nil, err
	}
	rows, err := imp.seam.ListDatasets(ctx)
	if err != nil {
		return nil, fmtError("DeleteDataset", err)
	}

	var targets []store.DatasetRow
	if strings.Contains(name, "__") {
		for _, r := range rows {
			if r.Name == name {
				targets = append(targets, r)
				break
			}
		}
	} else {
		prefix := name + "__"
		for _, r := range rows {
			if strings.HasPrefix(r.Name, prefix) {
				targets = append(targets, r)
			}
		}
	}
	if len(targets) == 0 {
		return nil, ErrDatasetNotFound
	}

	now := time.Now().UTC()
	deleted := make([]string, 0, len(targets))
	for _, target := range targets {
		if err := imp.deleteOneDataset(ctx, target, now); err != nil {
			return deleted, fmtError("DeleteDataset", err)
		}
		deleted = append(deleted, target.Name)
	}
	return deleted, nil
}

func (imp *Importer) deleteOneDataset(ctx context.Context, row store.DatasetRow, now time.Time) error {
	if imp.lru != nil {
		if err := imp.lru.Detach(ctx, row.Name); err != nil {
			return fmt.Errorf("detach %s: %w", row.Name, err)
		}
	}
	if err := os.Remove(row.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", row.FilePath, err)
	}
	if err := imp.seam.DeleteDataset(ctx, row.Name); err != nil {
		return fmt.Errorf("delete registry row %s: %w", row.Name, err)
	}
	return imp.seam.PutDatasetDelete(ctx, store.DatasetDeleteRow{
		Name:      row.Name,
		Dataset:   row.Dataset,
		DeletedAt: now,
	})
}
