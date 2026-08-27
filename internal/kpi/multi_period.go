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
	"sort"
)

// ResolvePeriods returns the sorted intersection of reqPeriods and the periods
// registered for dataset in the datasets registry. Every period in the result
// exists in the registry (i.e. has been imported). If a requested period is
// missing, returns a *Refusal with ReasonDatasetNotAvailable.
func ResolvePeriods(ctx context.Context, meta DatasetMetaSource, dataset string, reqPeriods []string) ([]string, *Refusal) {
	if len(reqPeriods) == 0 {
		return nil, nil
	}
	available, err := meta.Periods(ctx, dataset)
	if err != nil {
		return nil, &Refusal{ReasonCode: ReasonDatasetNotAvailable, HumanExplanation: "数据集 " + dataset + " 注册表读取失败"}
	}
	availSet := make(map[string]struct{}, len(available))
	for _, p := range available {
		availSet[p] = struct{}{}
	}
	var result []string
	for _, p := range reqPeriods {
		if _, ok := availSet[p]; ok {
			result = append(result, p)
		} else {
			return nil, &Refusal{
				ReasonCode:       ReasonDatasetNotAvailable,
				HumanExplanation: "数据集 " + dataset + " 在周期 " + p + " 无数据",
			}
		}
	}
	sort.Strings(result)
	return result, nil
}
