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

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Config 是本机 LLM provider 配置（~/.skillforge/llm.json）的内存形态。
type Config struct {
	Vendor    string `json:"vendor"`                  // 厂商 ID（catalog.go 的 Vendor.ID）
	Plan      string `json:"plan"`                    // 套餐 ID
	BaseURL   string `json:"base_url,omitempty"`      // custom 时必填；非空则覆盖 plan 的 BaseURL
	Model     string `json:"model"`                   // 模型名
	APIKey    string `json:"api_key,omitempty"`       // 明文 key；与 APIKeyEnv 二选一（T-52-01：文件 0600 兜底）
	APIKeyEnv string `json:"api_key_env,omitempty"`   // 环境变量名；非空则优先于 APIKey（D-04：不落盘明文 key 的间接引用）
	UpdatedAt string `json:"updated_at"`              // RFC3339 时间戳
}

// envConfigPath 覆盖默认配置路径（测试与多实例场景用）。
const envConfigPath = "SKILLFORGE_LLM_CONFIG"

// ConfigPath 返回配置文件路径：SKILLFORGE_LLM_CONFIG 优先，否则
// ~/.skillforge/llm.json。
func ConfigPath() string {
	if p := os.Getenv(envConfigPath); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// 极端场景（无 HOME）；返回相对路径让后续写报出可读错误，
		// 而不是在这里 panic。
		return filepath.Join(".skillforge", "llm.json")
	}
	return filepath.Join(home, ".skillforge", "llm.json")
}

// Load 读取配置。文件不存在时返回 (Config{}, false, nil)——对首次启动
// 这是正常状态而非错误。
func Load() (Config, bool, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, false, nil
		}
		return Config{}, false, fmt.Errorf("llmconfig: 读取 %s 失败: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, false, fmt.Errorf("llmconfig: %s 不是合法 JSON: %w", path, err)
	}
	if err := c.validate(); err != nil {
		return Config{}, false, fmt.Errorf("llmconfig: %s 内容不合法: %w", path, err)
	}
	return c, true, nil
}

// Validate 是 validate 的导出别名（Phase 52 admin 面板在保存前调用，
// 与 Save 内部的校验保持同一套规则）。
func (c Config) Validate() error { return c.validate() }

// validate 校验字段基础合法性：vendor/model 非空；custom 厂商必须给
// base_url（目录里 custom 的 Plan.BaseURL 是空串，没有覆盖来源）。
func (c Config) validate() error {
	if c.Vendor == "" {
		return errors.New("vendor 为空")
	}
	if c.Model == "" {
		return errors.New("model 为空")
	}
	// key 二选一：明文或 env 间接引用都为空 = 必然 401 的死配置。
	if c.APIKey == "" && c.APIKeyEnv == "" {
		return errors.New("api_key / api_key_env 必须二选一")
	}
	if c.Vendor == "custom" && c.BaseURL == "" {
		return errors.New("custom 厂商必须提供 base_url")
	}
	if _, ok := FindPlan(c.Vendor, c.Plan); !ok && c.Vendor != "custom" {
		return errors.New("unknown vendor/plan（custom 除外）；base_url 可覆盖目录，但 vendor/plan 必须真实存在")
	}
	return nil
}

// ResolveKey 解析出实际可用的 API key：APIKeyEnv 非空 ⇒ 读环境变量
// （变量存在但值为空也报错——静默降级成无 key 请求只会换来 401 迷惑
// 用户）；否则返回明文 APIKey（可能为空，由调用方决定是否容忍）。
func ResolveKey(c Config) (string, error) {
	if c.APIKeyEnv != "" {
		v := os.Getenv(c.APIKeyEnv)
		if v == "" {
			return "", fmt.Errorf("llmconfig: 环境变量 %s 未设置或为空", c.APIKeyEnv)
		}
		return v, nil
	}
	return c.APIKey, nil
}

// Save 持久化配置：mkdirall 0700 → 写同目录 tmp 文件 → chmod 0600 →
// os.Rename 原子替换。同一目录保证 rename 是同文件系统原子操作；先
// chmod 再 rename 保证目标文件从不以宽权限短暂存在（T-52-01）。补齐
// UpdatedAt，调用方无需自己填。
func Save(c Config) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("llmconfig: 拒绝保存不合法配置: %w", err)
	}
	c.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	path := ConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("llmconfig: 创建目录 %s 失败: %w", dir, err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("llmconfig: 序列化配置失败: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".llm.json-*")
	if err != nil {
		return fmt.Errorf("llmconfig: 创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename 成功后 remove 返回 not-exist，无害

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("llmconfig: 写入临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("llmconfig: 关闭临时文件失败: %w", err)
	}
	if runtime.GOOS != "windows" { // Windows 的 chmod 是 no-op，仅在 POSIX 上收紧权限
		if err := os.Chmod(tmpName, 0o600); err != nil {
			return fmt.Errorf("llmconfig: 收紧文件权限失败: %w", err)
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("llmconfig: 原子替换 %s 失败: %w", path, err)
	}
	return nil
}

// Mask 把 key 脱敏成可展示形态：len<=8 ⇒ "****"；否则 "…"+末 4 位。
// GET 配置端点永远返回 Mask 后的值，绝不回明文（T-52-01）。
func Mask(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return "…" + key[len(key)-4:]
}
