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

package server

// 261117-mpr — 面板「拉取模型列表」必须用「页面上刚填的那个 key」。
//
// 现状根因：GET /api/admin/llm/models 的凭据只能来自 api_key_env（白名单）
// 或已存文件配置 —— 明文 key 按 F-02 的决策**故意不进 query**（GET URL 会
// 落进 nginx access log、浏览器历史、Referer 头）。后果：用户在面板里填了
// API Key、点「拉取模型列表」，服务端根本收不到那把 key，只能发
// "Bearer anonymous" ⇒ 真实网关 401，面板显示一条让人以为是 key 错的红字
// （2026-11-17 用户截图：「拉取模型列表」报 HTTP 401 令牌已过期）。
//
// 修法：把候选连接放进**请求体**（POST），体不进 URL，F-02 担心的三个泄漏
// 面全都不成立；同时复用 resolveTestTarget 的凭据解析顺序，于是
//   「面板刚填的明文 key」> 「api_key_env 白名单」> 「已存配置 / env preset」
// 与原测试接口完全一致 —— 用户在页面怎么填，拉列表就用什么。
//
// GET 保留（旧客户端与脚本在用），但语义仍是「只用服务端持有的凭据」。

// llmModelsProbeRequest 是 POST /api/admin/llm/models 的请求体。
//
// 字段与 llmTestRequest 对齐，但多一个 models_url（目录里 opencode 之类
// 给了专门的列表端点）。APIKey 在这里是允许的：请求体不落 URL 日志。
type llmModelsProbeRequest struct {
	Vendor    string `json:"vendor"`
	Plan      string `json:"plan"`
	BaseURL   string `json:"base_url"`
	ModelsURL string `json:"models_url"`
	Model     string `json:"model"`
	APIKey    string `json:"api_key"`
	APIKeyEnv string `json:"api_key_env"`
}
