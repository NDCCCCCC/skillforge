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

// Package webui 让后端（:8888）同源伺服 Vite 构建产物 web/dist，
// 复刻 Dockerfile.frontend:39 的 nginx 语义，让 hermetic quickstart 只开
// 一个口就能在浏览器里用（D-13）。
//
// nginx 基线（Dockerfile.frontend:39 逐字）：
//
//	location /    { try_files $uri /index.html; }
//	location /api { proxy_pass http://backend:8888; proxy_buffering off; ... }
//
// 三条硬语义：
//  1. 无扩展名且磁盘上不存在（例如 /dashboard）⇒ 回 index.html（SPA fallback）。
//  2. 有扩展名但不存在（例如 /assets/missing.js）⇒ 404，绝不回 index.html
//     （把缺失的 js 变成 HTML 会让浏览器报 Unexpected token '<'）。
//  3. /api 与 /api/ 前缀永不进静态处理：未实现的接口必须 404，不能伪装成前端路由。
//
// web/dist（或其 index.html）缺失时，所有非 /api 请求返回 503 + 可操作的
// 构建提示，不静默 404、不列目录、不托管仓库源码。
package webui

import (
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
)

// DistDirDefault 是相对仓库根的 Vite 构建产物目录。
// 依据：Vite 默认 outDir（web/vite.config.ts 未覆盖 build.outDir）与
// Dockerfile.frontend:23-29 的 `COPY --from=builder /web/dist`。
// D-15 要求配置体系一字不动，因此它是固定相对路径，不读环境变量。
const DistDirDefault = "web/dist"

// CatchAllPath 是 hertz 根 catch-all 路由的字面量。
// 它不是 "/*"：route/tree.go 的 checkPathValid 要求 wildcard 必须带名字，
// 注册 "/*" 会在启动期 panic（`wildcards must be named with a non-empty name`）。
//
// 本包实际用 NoRoute 挂载（见 Mount），catch-all 路由字面量仅作为等价
// catch-all 语义的文档锚点保留：NoRoute 只在“没有任何路由命中”时被调用，
// 因此已注册的 /api/* 与 /reports* 结构性地不可能落进静态 handler
// （这是比“先注册全部 API、最后注册 catch-all”更强的不变式，见 Pitfall 13）。
const CatchAllPath = "/*filepath"

// Handler 返回 web/dist 的 http.Handler，按 nginx 的 try_files 语义处理
// 每个请求（无扩展名 → SPA fallback；有扩展名 → 404；/api → 404）。
//
// 不做方法分派：GET-only 由挂载侧的注册方式决定。
func Handler(distDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(distDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "" {
			p = "/"
		}

		// /api 闸。这不是冗余：即便用 NoRoute 挂载，未实现的 /api/xxx 也
		// 不该被当成前端路由回 index.html——把“接口不存在”变成“前端路由”
		// 会让前端的 404 处理误以为是页面路由问题。
		//
		// 这道闸必须排在 dist 存在性检查**之前**（phase 49 独立验证发现）：
		// 否则前端尚未构建（web/dist 缺失）时，一个根本不存在的 /api/xxx
		// 会拿到“前端没构建”的 503，而它其实该是 404——接口不存在与前端
		// 是否构建无关。已注册的 /api/* 路由根本到不了这里（NoRoute 只在
		// 无路由命中时触发），所以这道闸只影响“未注册的 /api 路径”。
		if p == "/api" || strings.HasPrefix(p, "/api/") {
			http.NotFound(w, r)
			return
		}

		// 逐请求检查 index.html 而非启动期缓存：支持“先起后端、后构建前端”
		// 的 hermetic 开发流（构建完刷新页面即可，无需重启）。
		indexPath := filepath.Join(distDir, "index.html")
		if _, err := os.Stat(indexPath); err != nil {
			serveMissingDist(w)
			return
		}

		// 根锚定的清洗：/../x 被折叠成 /x，rel 里不可能残留 .. 段。
		rel := path.Clean("/" + p)
		fsPath := filepath.Join(distDir, filepath.FromSlash(rel))

		// Lstat 而非 Stat：符号链接按“文件不存在”处理（404 或 SPA fallback），
		// 拒绝通过 dist 内的符号链接逃逸到仓库其它目录。残余风险：
		// http.FileServer 之后会按路径重新打开文件（存在 TOCTOU 窗口），
		// 本包按“dist 是构建产物、非用户可写目录”接受该风险。
		info, err := os.Lstat(fsPath)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			if hasExtension(rel) {
				http.NotFound(w, r)
				return
			}
			serveIndex(w, indexPath)
			return
		}
		if info.IsDir() {
			// 目录一律回 index.html：http.FileServer 默认会列目录。
			serveIndex(w, indexPath)
			return
		}

		// 其余交给 http.FileServer（自带 MIME 表、Range、Content-Length）。
		fileServer.ServeHTTP(w, r)
	})
}

// hasExtension 判定 rel 是否带扩展名。SPA fallback 只看这一点，不看 query
// string、不看 Accept 头。`/v1.2/foo` 这类“点号在中间段”的路径 ext 为空，
// 也回 index.html——这是刻意对齐 nginx 的 try_files，而非“含点即静态”。
func hasExtension(rel string) bool {
	return filepath.Ext(rel) != ""
}

// serveIndex 输出 SPA 外壳。Cache-Control: no-cache 是必需的：外壳必须
// 每次协商，否则换版本后旧 shell 引用已删除的 hash 资源会白屏。
// /assets/* 不额外加头，交给 http.FileServer（Vite assets 带内容 hash，可长缓存）。
//
// 刻意不用 http.ServeFile：它走 net/http 的 serveContent，会对 writer 做
// 大块 Malloc/Seek 假设；而本 handler 被 hertz 的 adaptor 包了一层
// httpResponseWriter，在 NoRoute（engine.go:832 的 serveError 分支）上下文里
// rc.GetWriter() 尚未分配可 Malloc 的缓冲，实测直接 panic
// `sigpanic: panicmem() header.go:93 Malloc`。index.html 是纯文本小文件，
// 自己读进来一次性写出即可，也顺带省掉 serveContent 的目录/symlink 复查。
func serveIndex(w http.ResponseWriter, indexPath string) error {
	b, err := os.ReadFile(indexPath)
	if err != nil {
		// index.html 刚被 Stat 确认存在，这里失败只能是并发删除/权限变化。
		http.Error(w, "index.html unreadable", http.StatusInternalServerError)
		return err
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(http.StatusOK)
	_, werr := w.Write(b)
	return werr
}

// serveMissingDist 在 web/dist 缺失时给出 503 + 可操作提示。
// body 逐字含构建指令，但绝不回显绝对路径、os.Getwd() 或 env，避免 503 页
// 泄露部署路径。Cache-Control: no-store 避免浏览器缓存 503 页导致构建完
// 仍看到旧提示。
func serveMissingDist(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusServiceUnavailable)
	//nolint:errcheck // 客户端可能已断开，写失败无可恢复动作
	w.Write([]byte("SkillForge: web/dist not found\n" +
		"the backend API (/api/*) is still served; the web UI needs a build first.\n" +
		"cd web && npm run build\n" +
		"(dev mode: cd web && npm run dev -> http://localhost:5173)\n"))
}

// Mount 把 web/dist 挂到 hertz 的 NoRoute 上——即“没有任何路由命中时”的
// 兜底处理。这结构性保证已注册的 /api/* 与 /reports* 不会被静态 handler
// 吞掉：NoRoute 只在路由树查找失败后被调用，是比“先注册全部 API 路由、
// 最后注册 catch-all”（Pitfall 13）更强的机械不变式。
//
// 仍然必须在全部 /api/* 与 /reports* 路由注册之后调用，保持意图显式。
//
// 只注册 GET（不用 h.Any）：把 POST/PUT 吞进静态 handler 对 SPA 没有意义，
// 只会扩大攻击面。不给 /api 路径加任何写缓冲或 writer 包装——Go 侧 SSE 直出
// 等价于 nginx 的 proxy_buffering off。
func Mount(h *server.Hertz, distDir string) {
	if _, err := os.Stat(filepath.Join(distDir, "index.html")); err != nil {
		log.Printf("[WARN] webui: %s missing — non-/api requests will answer 503 until cd web && npm run build runs", distDir)
	} else {
		log.Printf("[OK] webui: static web UI mounted from %s (SPA fallback; NoRoute)", distDir)
	}
	h.NoRoute(adaptor.HertzHandler(Handler(distDir)))
}
