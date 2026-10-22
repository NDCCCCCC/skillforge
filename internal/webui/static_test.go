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

package webui

// 静态托管的测试三条腿：
//  1. 直接用 httptest 打 Handler（语义判据：fallback / 404 / 503 / 不列目录 /
//     路径安全 / 缓存头 / 并发与中断确定性）；
//  2. 真实 hertz 实例 + 真实 TCP 请求（路由顺序：挂载点不遮蔽 /api）；
//  3. cmd/server/main.go 的源码顺序锁与固定 distDir 锁（Pitfall 13 / D-15）。
//
// 所有用例都用 t.TempDir() 造 dist，不依赖仓库是否已 `npm run build`。

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const (
	indexHTML = "<!doctype html>\n<html><body><div id=\"root\">skillforge</div></body></html>\n"
	assetJS   = "console.log('skillforge asset');\n"

	// goModMarker 是仓库 go.mod 的第一行内容，作为“泄露了仓库文件”的探针。
	goModMarker = "module skillforge"
)

// newDist 造一个最小可用的 dist 目录并返回其路径。
func newDist(t *testing.T) string {
	t.Helper()
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte(indexHTML), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dist, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dist, "assets", "app.js"), []byte(assetJS), 0o644); err != nil {
		t.Fatalf("write assets/app.js: %v", err)
	}
	return dist
}

// do 对 handler 发一次请求并返回 recorder。
func do(h http.Handler, method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// newHertz 造一个 :0 端口的真实 hertz 实例（不 Spin）。
func newHertz(t *testing.T) *server.Hertz {
	t.Helper()
	return server.Default(server.WithHostPorts(":0"))
}

// newLiveHertz 造一个真正在监听、在跑的 hertz 实例，返回它与 base URL。
//
// 为什么不用 hertz 的 ut.PerformRequest 测挂载：ut 的 createUtRequestContext
// （pkg/common/ut/context.go:34）从不给 ctx.conn 赋值，而 RequestContext.GetWriter
// 直接返回 ctx.conn（pkg/app/context.go:329）。于是 adaptor 的
// httpResponseWriter.WriteHeader 会 `w.Malloc(...)` 一个 nil writer，
// 实测任何经 HertzHandler 的 handler 在该 harness 里都 panic
// `sigpanic: panicmem() resp/header.go:93`——包括与 NoRoute 无关的普通注册路由。
// 那是 harness 的传输层缺失，不是本包缺陷；要验证“同一实例上 API 与 SPA 共存”
// 就必须走真实连接。
func newLiveHertz(t *testing.T, register func(h *server.Hertz)) (*server.Hertz, string) {
	t.Helper()
	// Hertz 的 *server.Hertz 不导出 Listener 字段（v0.10.6），所以先自己
	// 预留一个空闲回环端口再交给 Hertz 绑定：:0 由 Hertz 内部分配后无处读回。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a loopback port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("release the reserved port: %v", err)
	}
	h := server.New(server.WithHostPorts(addr))
	register(h)
	go h.Spin()
	base := "http://" + addr
	// h.Spin() 是异步的：goroutine 返回时内核还没完成 listen，立刻 GET 会拿到
	// connection refused。等端口真正接受连接再放行后续断言。
	deadline := time.Now().Add(5 * time.Second)
	for {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = c.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("hertz did not start listening on %s within 5s: %v", addr, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() {
		_ = h.Shutdown(context.Background())
	})
	return h, base
}

// fetchLive 发一次真实 HTTP GET，返回状态码、Content-Type 与 body。
func fetchLive(t *testing.T, base, path string) (int, string, string) {
	t.Helper()
	resp, err := http.Get(base + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return resp.StatusCode, resp.Header.Get("Content-Type"), string(b)
}

// jsonAPIHandler 返回最小 JSON，模拟一个已注册的 /api 路由。
func jsonAPIHandler(_ context.Context, c *app.RequestContext) {
	c.JSON(consts.StatusOK, map[string]string{"ok": "chat"})
}

// ── Task 1：单端口端到端最小可跑三条 ─────────────────────────────

func TestStaticHandlerServesIndexAtRoot(t *testing.T) {
	h := Handler(newDist(t))
	rec := do(h, http.MethodGet, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != indexHTML {
		t.Fatalf("GET / body = %q, want index.html content %q", got, indexHTML)
	}
}

func TestStaticHandlerServesAsset(t *testing.T) {
	h := Handler(newDist(t))
	rec := do(h, http.MethodGet, "/assets/app.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /assets/app.js status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("GET /assets/app.js Content-Type = %q, want javascript", ct)
	}
	if got := rec.Body.String(); got != assetJS {
		t.Fatalf("GET /assets/app.js body = %q, want %q", got, assetJS)
	}
}

func TestRouteOrderCatchAllDoesNotShadowAPI(t *testing.T) {
	dist := newDist(t)
	_, base := newLiveHertz(t, func(h *server.Hertz) {
		h.GET("/api/chat", jsonAPIHandler)
		Mount(h, dist)
	})

	// 已注册的 /api/chat 必须是 JSON，绝不是 HTML。
	code, ct, body := fetchLive(t, base, "/api/chat")
	if code != http.StatusOK {
		t.Fatalf("GET /api/chat status = %d, want 200", code)
	}
	if strings.Contains(strings.ToLower(body), "<!doctype html") {
		t.Fatalf("GET /api/chat returned HTML (the static mount shadowed the registered API): %q", body)
	}
	if !strings.Contains(body, `"ok"`) {
		t.Fatalf("GET /api/chat body = %q, want the registered JSON handler", body)
	}
	if strings.Contains(ct, "text/html") {
		t.Fatalf("GET /api/chat Content-Type = %q, want JSON", ct)
	}

	// 同一实例上，未匹配的前端路由回 index.html。
	code, _, body = fetchLive(t, base, "/dashboard")
	if code != http.StatusOK {
		t.Fatalf("GET /dashboard status = %d, want 200 (SPA fallback)", code)
	}
	if body != indexHTML {
		t.Fatalf("GET /dashboard body = %q, want index.html", body)
	}

	// 未注册的 /api 子路径必须 404，不能伪装成前端路由。
	code, _, body = fetchLive(t, base, "/api/unknown")
	if code != http.StatusNotFound {
		t.Fatalf("GET /api/unknown status = %d, want 404", code)
	}
	if strings.Contains(body, indexHTML) {
		t.Fatal("GET /api/unknown returned index.html — an unimplemented API must not look like an SPA route")
	}
}

// ── Task 2：边界语义 ─────────────────────────────────────────────

func TestStaticHandlerSPAFallbackForExtensionlessPath(t *testing.T) {
	h := Handler(newDist(t))
	for _, p := range []string{"/dashboard", "/dashboard/settings", "/skills/simple-qa", "/v1.2/foo"} {
		rec := do(h, http.MethodGet, p)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200 (SPA fallback)", p, rec.Code)
		}
		if got := rec.Body.String(); got != indexHTML {
			t.Fatalf("GET %s body = %q, want index.html", p, got)
		}
	}
}

func TestStaticHandlerMissingAssetReturns404(t *testing.T) {
	h := Handler(newDist(t))
	rec := do(h, http.MethodGet, "/assets/missing.js")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /assets/missing.js status = %d, want 404 (an extension never falls back to index.html)", rec.Code)
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "<!doctype html") {
		t.Fatal("GET /assets/missing.js returned HTML — the browser would fail with Unexpected token '<'")
	}
}

func TestStaticHandlerMissingDistReturns503(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "no-such-dist")
	rec := do(Handler(absent), http.MethodGet, "/")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET / with missing dist status = %d, want 503", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "SkillForge: web/dist not found") {
		t.Fatalf("503 body does not name the missing directory: %q", body)
	}
	if !strings.Contains(body, "cd web && npm run build") {
		t.Fatalf("503 body does not carry the build command: %q", body)
	}
	if strings.Contains(body, absent) {
		t.Fatalf("503 body leaks the absolute dist path %q: %q", absent, body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Fatalf("503 Content-Type = %q, want text/plain", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Fatalf("503 Cache-Control = %q, want no-store (a cached 503 would outlive the build)", cc)
	}
}

func TestStaticHandlerAPIPrefixNeverFallsBackToHTML(t *testing.T) {
	h := Handler(newDist(t))
	for _, p := range []string{"/api", "/api/", "/api/unknown", "/api/knowledge"} {
		rec := do(h, http.MethodGet, p)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404", p, rec.Code)
		}
		if strings.Contains(strings.ToLower(rec.Body.String()), "<!doctype html") {
			t.Fatalf("GET %s returned HTML — an unimplemented API must 404", p)
		}
	}
}

func TestStaticHandlerRejectsTraversalAndSymlink(t *testing.T) {
	h := Handler(newDist(t))
	for _, p := range []string{
		"/../go.mod",
		"/..%2f..%2fgo.mod",
		"/assets/../../go.mod",
		"/%2e%2e/%2e%2e/go.mod",
	} {
		rec := do(h, http.MethodGet, p)
		body := rec.Body.String()
		if strings.Contains(body, goModMarker) {
			t.Fatalf("GET %s leaked the repository go.mod", p)
		}
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 404 or 200", p, rec.Code)
		}
		if rec.Code == http.StatusOK && body != indexHTML {
			t.Fatalf("GET %s returned 200 with non-index content — traversal escaped dist", p)
		}
	}

	// dist 内指向仓库 go.mod 的符号链接必须按“不存在”处理。
	link := filepath.Join(newDist(t), "link.js")
	if err := os.Symlink(filepath.Join("..", "go.mod"), link); err != nil {
		t.Skipf("symlink unsupported on this platform (%v) — run under WSL to exercise this case", err)
	}
	rec := do(h, http.MethodGet, "/link.js")
	if strings.Contains(rec.Body.String(), goModMarker) {
		t.Fatal("a symlink inside dist served the repository go.mod — the Lstat symlink rejection is broken")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /link.js status = %d, want 404", rec.Code)
	}
}

func TestStaticHandlerNeverListsDirectories(t *testing.T) {
	h := Handler(newDist(t))
	for _, p := range []string{"/assets", "/assets/"} {
		rec := do(h, http.MethodGet, p)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", p, rec.Code)
		}
		if got := rec.Body.String(); got != indexHTML {
			t.Fatalf("GET %s body = %q, want index.html (never a directory listing)", p, got)
		}
		if strings.Contains(rec.Body.String(), "Index of /") {
			t.Fatalf("GET %s produced a directory listing", p)
		}
	}
}

func TestStaticHandlerIndexIsNoCache(t *testing.T) {
	h := Handler(newDist(t))
	rec := do(h, http.MethodGet, "/")
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Fatalf("GET / Cache-Control = %q, want no-cache (a stale shell references deleted hash assets)", cc)
	}
}

// ── Task 3：路由顺序锁 / 并发不变量 / 启动日志 / 固定 distDir ──────

// mainGoSourceFrom reads cmd/server/main.go for the source-order locks.
func mainGoSourceFrom(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "cmd", "server", "main.go"))
	if err != nil {
		t.Fatalf("read cmd/server/main.go: %v", err)
	}
	return string(raw)
}

// TestRouteOrderAPIRoutesRegisteredBeforeCatchAll is the mechanical form of
// Pitfall 13: webui.Mount must run after every API route is registered and
// before h.Spin().
//
// 注意 main.go 本身不写 h.GET("/api/chat", …)：全部 API 路由由
// internal/server/server.go 的 Register 在 server.New 内部注册
// （internal/server/server.go:579 的 `h.POST("/api/chat", deps.chatHandler)`），
// main.go 只负责构造 h、打印路由清单、再挂静态层。所以顺序不变式是两段：
// server.go 确实注册了 chat 路由，且 main.go 的挂载排在 server.New 之后、Spin 之前。
func TestRouteOrderAPIRoutesRegisteredBeforeCatchAll(t *testing.T) {
	// (1) API 路由仍然注册在 server.go 里（没被谁搬到别处或删掉）。
	routerSrc, err := os.ReadFile(filepath.Join("..", "..", "internal", "server", "server.go"))
	if err != nil {
		t.Fatalf("read internal/server/server.go: %v", err)
	}
	if !strings.Contains(string(routerSrc), `h.POST("/api/chat", deps.chatHandler)`) {
		t.Fatal(`internal/server/server.go no longer registers h.POST("/api/chat", deps.chatHandler) — update this order lock alongside the mount`)
	}
	for _, anchor := range []string{`h.GET("/api/sessions"`, `h.GET("/api/eval/reports"`, `h.DELETE("/api/import/dataset/:name"`} {
		if !strings.Contains(string(routerSrc), anchor) {
			t.Fatalf("internal/server/server.go no longer registers %s — update this order lock alongside the mount", anchor)
		}
	}

	// (2) main.go 里挂载排在 server.New 之后、h.Spin() 之前。
	//
	// 匹配必须锚定“代码行”而不是任意出现：`strings.Index` 的首次匹配会命中
	// 挂载点上方那段解释性注释里的 “h.Spin()” 字样（main.go:1281 的
	// “把挂载放在全部路由之后、h.Spin() 之前”），那不是调用点。函数体内的
	// 顶层语句都是 Tab 缩进，而注释行是 “\t// …”，所以用前置换行 + Tab 锚定
	// 既能只命中真实调用，又不依赖注释措辞。
	src := mainGoSourceFrom(t)
	mainIdx := strings.Index(src, "func main() {")
	if mainIdx < 0 {
		t.Fatal("cmd/server/main.go no longer contains `func main() {` — update this order lock")
	}
	mainSrc := src[mainIdx:]
	indexOfInMain := func(needle string) int {
		i := strings.Index(mainSrc, needle)
		if i < 0 {
			t.Fatalf("cmd/server/main.go's route registration block changed (missing %q) — update this order lock alongside the mount", needle)
		}
		return i
	}

	mount := indexOfInMain("\n\twebui.Mount(")
	newServer := indexOfInMain("\n\th := server.New(")
	if newServer > mount {
		t.Fatalf("route order broken: webui.Mount (offset %d) must come AFTER `h := server.New(` (offset %d) — the Hertz instance and every API route registration must already exist", mount, newServer)
	}
	if spin := indexOfInMain("\n\th.Spin()"); mount >= spin {
		t.Fatalf("route order broken: webui.Mount (offset %d in main) must come BEFORE h.Spin() (offset %d) — mounting after Spin is dead code", mount, spin)
	}
}

// TestStaticHandlerConcurrentRequestsAreDeterministic is the landing point of
// the DEMO-02 concurrency probe: 200 goroutines mixing four path classes must
// only ever see 200/404/503 — never a 500 or a panic — and every 200 body must
// match its Content-Length. Runs well inside 30s.
func TestStaticHandlerConcurrentRequestsAreDeterministic(t *testing.T) {
	srv := httptest.NewServer(Handler(newDist(t)))
	defer srv.Close()

	paths := []string{"/", "/assets/app.js", "/dashboard", "/assets/missing.js"}
	const n = 200

	var (
		mu   sync.Mutex
		errs []string
		wg   sync.WaitGroup
	)
	wg.Add(n)
	for i := 0; i < n; i++ {
		p := paths[i%len(paths)]
		go func(p string) {
			defer wg.Done()
			resp, err := http.Get(srv.URL + p)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("GET %s: transport error %v", p, err))
				mu.Unlock()
				return
			}
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("GET %s: read body: %v", p, readErr))
				mu.Unlock()
				return
			}
			switch resp.StatusCode {
			case http.StatusOK:
				if cl := resp.Header.Get("Content-Length"); cl != "" {
					want, convErr := strconv.Atoi(cl)
					if convErr != nil || want != len(body) {
						mu.Lock()
						errs = append(errs, fmt.Sprintf("GET %s: body %d bytes but Content-Length %q", p, len(body), cl))
						mu.Unlock()
					}
				}
				if strings.HasSuffix(p, ".js") && bytes.HasPrefix(bytes.ToLower(body), []byte("<!doctype html")) {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("GET %s: a 200 .js response starts with HTML", p))
					mu.Unlock()
				}
			case http.StatusNotFound, http.StatusServiceUnavailable:
				// 合法结果。
			default:
				mu.Lock()
				errs = append(errs, fmt.Sprintf("GET %s: status %d, want 200/404/503", p, resp.StatusCode))
				mu.Unlock()
			}
		}(p)
	}
	wg.Wait()
	for _, e := range errs {
		t.Error(e)
	}
}

// TestStaticHandlerSurvivesIndexRemoval is the interruption variant: while
// requests are in flight index.html disappears. The handler must answer 503 or
// a complete 200 body — never a truncated 200.
func TestStaticHandlerSurvivesIndexRemoval(t *testing.T) {
	dist := newDist(t)
	indexPath := filepath.Join(dist, "index.html")
	saved, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	srv := httptest.NewServer(Handler(dist))
	defer srv.Close()

	var (
		mu   sync.Mutex
		errs []string
		wg   sync.WaitGroup
	)
	const n = 120
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			resp, err := http.Get(srv.URL + "/")
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("GET / during index removal: %v", err))
				mu.Unlock()
				return
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			switch resp.StatusCode {
			case http.StatusServiceUnavailable:
			case http.StatusOK:
				if len(body) != len(saved) {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("GET / got 200 with %d bytes, want the full %d-byte shell", len(body), len(saved)))
					mu.Unlock()
				}
			default:
				mu.Lock()
				errs = append(errs, fmt.Sprintf("GET / during index removal: status %d, want 200 or 503", resp.StatusCode))
				mu.Unlock()
			}
		}()
	}
	if err := os.Remove(indexPath); err != nil {
		t.Fatalf("remove index.html: %v", err)
	}
	wg.Wait()
	if err := os.WriteFile(indexPath, saved, 0o644); err != nil {
		t.Fatalf("restore index.html: %v", err)
	}
	for _, e := range errs {
		t.Error(e)
	}
}

// TestMountLogsMissingDistWarning locks the startup posture: an unbuilt dist
// must WARN with the build command; a built one must report [OK].
func TestMountLogsMissingDistWarning(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	Mount(newHertz(t), filepath.Join(t.TempDir(), "absent"))
	if out := buf.String(); !strings.Contains(out, "[WARN]") || !strings.Contains(out, "npm run build") {
		t.Fatalf("Mount on a missing dist logged %q, want [WARN] plus the build command", out)
	}

	buf.Reset()
	Mount(newHertz(t), newDist(t))
	if out := buf.String(); !strings.Contains(out, "[OK]") {
		t.Fatalf("Mount on a built dist logged %q, want [OK]", out)
	}
}

// TestMountUsesFixedDistDir locks D-15: the mount uses the fixed relative
// distDir constant, and no new env read was introduced after server.New.
func TestMountUsesFixedDistDir(t *testing.T) {
	src := mainGoSourceFrom(t)
	if !strings.Contains(src, "webui.Mount(h, webui.DistDirDefault)") {
		t.Fatal("cmd/server/main.go no longer mounts with the fixed webui.DistDirDefault — the dist path must stay a constant, not env-driven (D-15)")
	}
	if DistDirDefault != "web/dist" {
		t.Fatalf("DistDirDefault = %q, want \"web/dist\"", DistDirDefault)
	}
	newServer := strings.Index(src, "h := server.New(")
	mount := strings.Index(src, "webui.Mount(")
	if newServer < 0 || mount < 0 {
		t.Fatal("cmd/server/main.go no longer contains both server.New and webui.Mount")
	}
	if newServer > mount {
		t.Fatal("webui.Mount must come after `h := server.New(` — the Hertz instance does not exist before it")
	}
	// 只扫挂载语句本身所在的这一行：mount 之后仍有 embedder 等无关装配代码
	// 读环境变量，那是既有配置体系，与本 plan 的挂载无关（D-15 只冻结挂载参数）。
	mountLine := src[mount:]
	if nl := strings.IndexByte(mountLine, '\n'); nl >= 0 {
		mountLine = mountLine[:nl]
	}
	for _, envRead := range []string{"envOr(", "os.Getenv(", "envOrInt("} {
		if strings.Contains(mountLine, envRead) {
			t.Fatalf("the webui.Mount call reads config (%s) — D-15 forbids new configuration: %q", envRead, mountLine)
		}
	}
}

// TestCatchAllPathIsNamed guards the hertz constraint behind the constant:
// checkPathValid rejects an unnamed `/*` at registration time.
func TestCatchAllPathIsNamed(t *testing.T) {
	if strings.HasPrefix(CatchAllPath, "/*") && strings.Count(CatchAllPath, "*") != 1 {
		t.Fatalf("CatchAllPath = %q, want the single named root wildcard form /*<name>", CatchAllPath)
	}
	if CatchAllPath == "/*" {
		t.Fatalf("CatchAllPath = %q — hertz requires the root wildcard to be named or registration panics", CatchAllPath)
	}
}
