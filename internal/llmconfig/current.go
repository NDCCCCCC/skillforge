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

package llmconfig

// CurrentVendor 返回文件 LLM 配置的 vendor ID；无配置或读取失败返回 ""。
// 读失败按未知厂商处理（保守：调用方沿用默认请求字段）——与 resolver 的
// fail-open-to-stub 姿态一致，这里绝不需要 Fatal。
func CurrentVendor() string {
	cfg, present, err := Load()
	if err != nil || !present {
		return ""
	}
	return cfg.Vendor
}
