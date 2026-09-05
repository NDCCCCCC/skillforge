---
slug: windows-gcc-emutls-link
status: awaiting_human_verify
trigger: |
  PS D:\CODE\ClaudeCode\ai-agent> go run .\cmd\server\main.go
  link.exe: running gcc failed: exit status 1
  C:/msys64/ucrt64/bin/.../ld.exe: .../libicu_extension.a(ub_duckdb_icu_common.cpp.obj):
    (.rdata$.refptr.__emutls_v._ZSt11__once_call): undefined reference to `__emutls_v._ZSt11__once_call'
  C:/msys64/ucrt64/bin/.../ld.exe: .../libicu_extension.a(ub_duckdb_icu_common.cpp.obj):
    (.rdata$.refptr.__emutls_v._ZSt15__once_callable): undefined reference to `__emutls_v._ZSt15__once_callable'
  collect2.exe: error: ld returned 1 exit status
created: 2026-09-05
updated: 2026-09-05
goal: find_and_fix
---

# Debug Session: windows-gcc-emutls-link

Windows 原生 CGO 链接 go-duckdb 失败（MSYS2 ucrt64 gcc 16.1.0 emutls ABI 不匹配）。
目标：尝试找到 Windows 原生可用的真实修复；若不可行，按项目 blocked-with-notes 先例收敛为
"环境受限 + WSL 绕行"的最终结论，杜绝反复踩坑。

## Symptoms (gathered)

- expected: PowerShell 原生 `go run ./cmd/server/main.go` 构建并启动服务器
- actual: link.exe 调 gcc 失败；`duckdb-go-bindings/windows-amd64@v0.1.21` 的
  `libicu_extension.a` 中 `ub_duckdb_icu_common.cpp.obj` 引用
  `__emutls_v._ZSt11__once_call` / `__emutls_v._ZSt15__once_callable` 未定义
- errors: 逐字 trigger（见 frontmatter）；collect2 → ld returned 1
- timeline: 本机从未成功。首次确认于 Phase 14/15（2026-08-22/23），项目已两次记为
  deferred/env_issue（STATE.md "gcc linking (MSYS2 emutls ABI)"；D15-01-1 记录
  duckdb-go-bindings#105，internal/data + internal/duckdb 套件因此固定 WSL2 运行）
- reproduction: 任何把 go-duckdb 链进最终二进制的 Windows CGO 构建
  （`go run ./cmd/server/main.go`、`go build ./cmd/server`、`go build ./cmd/eval`）。
  纯 modernc sqlite / 非 CGO 包不受影响（go test 大部分包绿）
- verified_workaround: WSL2 Ubuntu-22.04 `/usr/local/go/bin/go`（go1.25.0）构建
  cmd/server exit 0 且服务器可运行（2026-09-02 本会话实证，:8888 服务正常）

## Environment facts

- Windows 11 Enterprise 10.0.26200；Go toolchain v0.0.1-go1.25.0.windows-amd64
- CC = C:\msys64\ucrt64\bin\gcc.exe（MSYS2 UCRT64，gcc 16.1.0，ld 2.4x）
- 依赖：github.com/marcboeker/go-duckdb（经 duckdb-go-bindings v0.1.21 windows-amd64
  静态库）；mattn/go-sqlite3 v1.14.50（可链接，非故障点）
- 链接行含 -lduckdb_static 及全部 extension 静态库 + -lstdc++ --static

## Prior art (project-internal, read-only evidence)

- D15-01-1（STATE.md 决策表）：emutls ABI 不匹配归因 + duckdb-go-bindings#105 引用；
  决议 = WSL2 跑 internal/data / internal/duckdb / cmd/server / cmd/eval
- v1.0 closeout deferred 表：`env_issue | gcc linking (MSYS2 emutls ABI) | Deferred`
- 本次 /gsd-debug 触发 = 用户 PowerShell 再度撞上同一错误（第 3 次记录在案）

## Current Focus

hypothesis: |
  （根因已证）duckdb-go-bindings v0.1.21 windows-amd64 静态库由 GCC 10.4.0 mingw
  （emutls 方言）构建，对象引用 __emutls_v._ZSt11__once_call / __emutls_v._ZSt15__once_callable /
  __emutls_get_address；MSYS2 ucrt64 gcc 16.1.0 以 --enable-tls（native TLS）构建其
  libstdc++/libgcc，不含任何 __emutls_v.* 定义 → 链接期未定义引用。
  修复 = 用 emutls 方言的 mingw-w64 GCC（winlibs 14.2.0 ucrt posix-seh，便携装 temp）
  作 CC/CXX 重链。
next_action: 等待用户在真实 PowerShell 工作流按 Resolution.fix 配方确认（human-verify）；
  通过后由用户决定是否把配方写入 Makefile/文档（本次调试不落仓库）
test: Windows PowerShell `go build ./cmd/server` exit 0（修复判定标准）
expecting: H1 成立 → 链接成功；若仍报 __emutls_v 或其它符号缺失 → 假设证伪，回
  investigation_loop（备选：MSVCRT 变体 / GCC 12 / niXman；预算剩 4 次链接尝试）
reasoning_checkpoint: |
  hypothesis: duckdb-go-bindings windows-amd64@v0.1.21 静态库（GCC 10.4.0/emutls）与
    MSYS2 ucrt64 gcc16（native TLS）TLS 方言失配；emutls 方言工具链可解。
  confirming_evidence:
    - "MSYS2 libstdc++.a：两符号仅 native-TLS 形态（.tls$$ 节），__emutls_v.* 零定义"
    - "duckdb libicu_extension.a：U __emutls_get_address + U __emutls_v._ZSt1[15]__once_call；
      .comment 指纹 GCC 10.4.0 (GNU)；波及 pg_query/re2/icu/tpch 4 库"
    - "winlibs 14.2.0 libstdc++.a 定义该两 __emutls_v.* 符号；探针 thread_local+call_once
      编译+链接+运行 exit 0，nm 显示 .refptr.__emutls_v._ZSt11__once_call 已解析"
  falsification_test: "winlibs CC 下 go build ./cmd/server 仍报 __emutls_v 未定义引用
    （或其它证明 ABI 另有失配的符号缺失）→ 假设证伪"
  fix_rationale: "根因是 TLS 方言的工具链失配（非代码/依赖/链接顺序），换同方言工具链
    直接消除符号缺口"
  blind_spots: "winlibs 14.2 与 10.4 其余 libstdc++ 符号集差异（向后兼容，低风险）；
    __emutls_get_address 在 libgcc_eh.a 而非 libgcc.a，依赖 gcc 链接 spec（探针已证可解析）；
    duckdb 对象的 CRT 口味未声明（未发现 msvcrt 专属标记，UCRT 风险低）"
test: Windows PowerShell `go build ./cmd/server` exit 0（修复判定标准）
expecting: H1 成立 → 链接成功；H1/H2/H3 全败 → 收敛为"Windows 原生不可行 +
  WSL 绕行为正式结论"写入 Resolution，不再反复排查
reasoning_checkpoint: |
  项目纪律约束：(a) no-falsify——任何"修复"必须真实链接成功才算；
  (b) 预算 ≤5 次链接尝试（每次 DuckDB 静态链接需数分钟）；
  (c) 工具链安装仅允许装到独立目录（如 winlibs 解压版）并只经 CC/CGO 环境变量生效，
  不改系统 PATH 持久设置、不改 go.mod、不动 eval/frozen 锁与 .planning 文档；
  (d) 若找到可用修复：报告所需工具链 + 环境变量配方，等用户批准后再考虑写入
  Makefile/文档；(e) 若无修复：按 14-08 先例写 blocked-with-notes 终局结论。

## Evidence

- timestamp: 2026-09-05T00:00Z — 用户 PowerShell `go run ./cmd/server/main.go` 复现
  同一 emutls 未定义引用（libicu_extension.a）；与 D15-01-1 / duckdb-go-bindings#105 记录一致
- timestamp: 2026-09-02 — WSL2 构建对照组：`go build -o /tmp/... ./cmd/server/main.go`
  exit 0；同日以 skillforge-server-dev-new 运行 :8888 服务正常 → 故障严格隔离在
  Windows 侧工具链，代码无嫌疑
- timestamp: 2026-09-05 (Phase A-1) — MSYS2 ucrt64 gcc 16.1.0 configure 含 `--enable-tls`
  （mingw native TLS），thread model posix；`nm --defined-only libstdc++.a` 证明
  `_ZSt11__once_call`/`_ZSt15__once_callable` 仅有 native-TLS 定义（`.tls$$` 节），
  **零** `__emutls_v.*` 定义 → 机制确认：emutls 对象 + native-TLS libstdc++ = ABI 失配；
  与符号新旧、链接顺序无关
- timestamp: 2026-09-05 (Phase A-2) — duckdb-go-bindings windows-amd64@v0.1.21 静态库由
  **GCC 10.4.0 (GNU)** 构建（.comment strings）；libicu_extension.a 未定义引用 =
  `__emutls_get_address` + `__emutls_v._ZSt11__once_call` + `__emutls_v._ZSt15__once_callable`
- timestamp: 2026-09-05 (Phase A-3) — emutls 波及面：libduckdb_pg_query.a / libduckdb_re2.a /
  libicu_extension.a / libtpch_extension.a 共 4 个库 → 无法靠剔除单个 extension 绕开
- timestamp: 2026-09-05 (Phase A-4) — duckdb 对象要求 winpthreads（pthread_mutex_* /
  pthread_once / pthread_cond_timedwait / pthread_self）→ 修复工具链必须是 posix 线程模型
- timestamp: 2026-09-05 (Phase A-5) — libduckdb_static.a 未定义引用中无 msvcrt 专属标记
  （_iob/__getmainargs/_controlfp 等 0 命中）→ UCRT runtime 工具链可行
- timestamp: 2026-09-05 (Research) — 上游 duckdb/duckdb-go-bindings#105 仍 OPEN（无官方修复）；
  GCC mingw native TLS 经 gcc bugzilla 80881 于 ~GCC 15 落地 → **GCC ≤14 的 mingw 构建仍是
  emutls**；候选 = winlibs GCC 14.2.0 posix-seh UCRT 便携版（brechtsanders/winlibs_mingw）
- timestamp: 2026-09-05 (Link line) — windows-amd64@v0.1.21 cgo_static.go LDFLAGS 全文：
  `-lduckdb_static <各 extension> -lws2_32 -lwsock32 -lrstrtmgr -lstdc++ -lm --static
  -L${SRCDIR}/`；-lstdc++ 在 duckdb 库**之后** → 归档顺序健康（再证非顺序问题）；
  未显式 -lwinpthread → 依赖 gcc posix 线程 spec 自动追加
- timestamp: 2026-09-05 (Env) — 本机网络：api.github.com 不可达（000），sourceforge.net
  可达（200）；bash 沙箱默认阻断出网，下载需在沙箱外执行。winlibs 非LLVM UCRT zip
  已从 SourceForge 后台下载至 temp/emutls-tc
- timestamp: 2026-09-06 (Download) — winlibs x86_64 posix-seh gcc-14.2.0 mingw-w64ucrt-r2.zip
  经 ghfast.top 加速器 16 路并行 range 下载完成，244502971 字节，sha256 =
  d41933cef13113018418d7b596319c2ed59a567395bbb87afe27a171e111d553 与 SourceForge 官方
  .sha256 一致（第三方通道完整性已证）；tar 解压到 temp/emutls-tc/mingw64
- timestamp: 2026-09-06 (Pre-check PASS) — winlibs gcc 14.2.0 `x86_64-ucrt-posix-seh`：
  thread model posix；**libstdc++.a 定义 `__emutls_v._ZSt11__once_call` +
  `__emutls_v._ZSt15__once_callable`（正是缺失的两符号）**；libwinpthread.a 有
  pthread_mutex_lock/pthread_once；thread_local + std::call_once 探针 编译+链接+运行
  exit 0，nm 显示 `.refptr.__emutls_v._ZSt11__once_call` 已解析、`__emutls_get_address`
  为 T → 工具链与 duckdb 对象的 emutls ABI 完全匹配
- timestamp: 2026-09-06 (Note) — `__emutls_get_address` 不在 libgcc.a 而在
  libgcc_eh.a/libgcc_s.a；gcc 默认链接 spec 能解析（探针已证），不阻塞
- timestamp: 2026-09-06 (Link attempt 1, PASS) — winlibs CC/CXX/PATH 注入，
  `go build -o temp/.../skillforge-server-win.exe ./cmd/server` **exit 0**，
  161,988,417 B；DLL 依赖仅系统库；`__emutls_v._ZSt11__once_call` 在终产物中已定义、
  `__emutls_get_address` T → H1 成立，根因修复确认
- timestamp: 2026-09-06 (Runtime verify) — 冒烟运行：进程启动、Go runtime 初始化、
  走到应用级配置校验（BGE_M3_URL 未设，log.Fatalf 优雅退出）→ 无 TLS/加载崩溃
- timestamp: 2026-09-06 (Link attempt 2 + runtime, PASS) —
  `go test ./internal/duckdb/`（4 个 :memory: 用例，含 50 并发读者）全 PASS 0.904s；
  Windows 原生首次跑通该包（此前 WSL-only）
- timestamp: 2026-09-06 (Exact-form verify) — `go build ./cmd/server`（用户同形命令）
  exit 0；临时产物已清理，仓库无改动（files_changed: []）

## Eliminated

- hypothesis: 代码级回归（Phase 20 改动引入）——排除：错误发生在链接期且仅 Windows
  工具链触发；WSL 同一 HEAD 构建成功；MSYS2 gcc 16.1.0 自 2026-08 起未变
- hypothesis: go.mod 依赖漂移——排除：duckdb-go-bindings v0.1.21 pin 与 Phase 14/15
  记录一致，无升级记录
- hypothesis: H2 libstdc++ 版本差 / `-static-libstdc++` 可解——排除：MSYS2 libstdc++.a
  **有** 这两个符号但为 native-TLS 形态；任何静态/动态 flag 都无法凭空产生
  `__emutls_v.*` 控制变量定义
- hypothesis: H3 `-fuse-ld=lld` 可解——排除：符号在全部输入（duckdb 对象 + MSYS2
  libgcc/libstdc++）中均无定义，换链接器无济于事
- hypothesis: 链接顺序问题（libstdc++.a 归档单遍扫描）——排除：同上，符号根本不存在，
  与归档扫描顺序无关

## Resolution

root_cause: |
  TLS 方言（TLS dialect）工具链失配，而非符号新旧/链接顺序/依赖问题：
  duckdb-go-bindings windows-amd64@v0.1.21 静态库由 mingw-w64 **GCC 10.4.0**（.comment
  指纹 "GCC: (GNU) 10.4.0"，emutls 时代）构建，其对象（pg_query/re2/icu/tpch 4 个 .a）
  以 **emutls**（仿真线程局部存储）实现 C++ thread_local，引用
  `__emutls_v._ZSt11__once_call` / `__emutls_v._ZSt15__once_callable` /
  `__emutls_get_address`。而本机 MSYS2 ucrt64 gcc 16.1.0 以 `--enable-tls`（mingw
  **native TLS**，经 gcc bugzilla 80881 于 ~GCC 15 落地）构建其 libstdc++：两个符号
  只有 native-TLS 形态（`.tls$$` 节），`__emutls_v.*` 定义数为 0 → 链接期必然
  undefined reference。GCC 15+ 的 mingw 工具链都会踩中；与 Go/代码/duckdb 版本无关。
fix: |
  用 emutls 方言的 mingw-w64 GCC 作 CGO 工具链（GCC ≤14；选用 winlibs GCC 14.2.0
  posix-seh UCRT，便携解压至 temp，仅经 CC/CXX/PATH 单命令注入，不改系统 PATH/go.mod）：
  工具链: C:\Users\CPIC\AppData\Local\Temp\emutls-tc\mingw64
    winlibs-x86_64-posix-seh-gcc-14.2.0-mingw-w64ucrt-12.0.0-r2（非 LLVM standalone）
    下载: sourceforge.net/projects/winlibs-mingw/files/14.2.0posix-19.1.1-12.0.0-ucrt-r2/
    244502971 B，sha256 d41933cef13113018418d7b596319c2ed59a567395bbb87afe27a171e111d553
  PowerShell 配方（每次构建前设置，会话级）:
    $env:CC   = "C:\Users\CPIC\AppData\Local\Temp\emutls-tc\mingw64\bin\gcc.exe"
    $env:CXX  = "C:\Users\CPIC\AppData\Local\Temp\emutls-tc\mingw64\bin\g++.exe"
    $env:PATH = "C:\Users\CPIC\AppData\Local\Temp\emutls-tc\mingw64\bin;" + $env:PATH
    go build .\cmd\server
  注: cmd/eval、internal/data、internal/duckdb 等一切 CGO 构建同样适用该配方；
      温度提示——若日后升级 duckdb-go-bindings 至以 native-TLS/MSVC 工具链构建的版本
      （上游 #105 仍 OPEN），方可解除对该工具链的依赖。
verification: |
  全部通过（2026-09-06，Windows 11 原生，非 WSL）：
  (1) `go build ./cmd/server` exit 0（与用户复现命令完全同形），产物 161,988,417 B，
      仅依赖系统 DLL（KERNEL32/UCRT api-ms-*/WS2_32/RstrtMgr），无 winpthread 等随行 DLL
  (2) nm 终产物：`.data$__emutls_v._ZSt11__once_call` 已定义 + refptr 已解析 +
      `__emutls_get_address`（T）→ 原缺失符号全部闭合
  (3) 运行冒烟：exe 正常启动执行至应用级配置校验（BGE_M3_URL 未设 → 优雅退出），
      无 TLS/加载崩溃
  (4) `go test ./internal/duckdb/ -run 'TestDuckDBBootQueries|TestDuckDBCloseFlushesWAL|
      TestDuckDBAttachDatasetRejectsInvalidID|TestDuckDBConcurrentReadNoBlockWrite'
      -count=1` 全 PASS（0.904s）——含 50 并发读者+写者用例（真线程反复走
      __emutls_get_address）；该包此前被迫 WSL-only，现已 Windows 原生可跑
  (5) 链接预算：实际消耗 2 次完整链接（server 构建 + duckdb 测试二进制），第 3 次
      为缓存命中；≤5 限额内
files_changed: []
