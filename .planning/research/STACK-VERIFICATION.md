# STACK-VERIFICATION — v1.0 新增依赖全面调查报告

> **日期**: 2026-08-21 · **审计对象**: `.planning/research/STACK.md` 中 v1.0 milestone 的全部新增依赖与技术选型
> **验证方法**: go module proxy（goproxy.cn）权威版本查询 + 模块源码下载 grep（API/依赖闭包逐行核对）+ 官方 README（go-duckdb / cron / goldmark / bluemonday）+ DuckDB 官方文档（ATTACH 语句）+ 本机构建环境实测
> **结论先行**: 所有选型方向**全部成立**（DuckDB / cron / goldmark / bluemonday / stdlib csv+hmac+template），无一需要推翻；但存在 **1 个 CRITICAL 环境阻塞**（本机无 gcc → go-duckdb 本地编译不过）、**1 个模块路径错误**（bluemonday 路径写错，照抄会 `go get` 失败）、若干版本/事实修正与实现级风险需要带入后续 spec。

---

## 0. 总判定表

| # | 依赖 | STACK.md 声明 | 验证结果 | 判定 |
|---|------|--------------|---------|------|
| 1 | `github.com/marcboeker/go-duckdb/v2` | v2.4.0 line，静态链接 C++ 引擎 | 最新 **v2.4.3**（捆绑 DuckDB 引擎 **v1.4.1**）；维护权正在移交 **DuckDB 官方团队**（Marc Boeker + DuckDB team 联合，MIT 不变）；静态链接确认，二进制 +60–100MB；**CGO 必需，本机无 gcc = 本地构建阻塞** | ✅ 选型成立 / ⚠️ **CRITICAL 环境项 + 版本修正** |
| 2 | `github.com/robfig/cron/v3` | v3.0.1+（"released 2024"），WithLocation/CRON_TZ=/WithChain(Recover, SkipIfStillRunning)/Stop() context | 全部 API 源码逐行确认存在（option.go:11/34/41, chain.go:38/78, cron.go:323）；**v3.0.1 发布于 2020 年**（v3 线 2019-06），此后休眠但 feature-complete、零依赖 | ✅ 成立 / ⚠️ 年份修正 + 休眠风险已接受 |
| 3 | `github.com/yuin/goldmark` | v1.7+，CommonMark 0.31.2，GFM 扩展 | 最新 **v1.8.5**；CommonMark **0.31.2** 合规确认；README 明示 "Depends only on standard libraries"（**完全零依赖**，连 x/text 都不需要）；**默认不渲染 raw HTML/危险 URL**（`html.WithUnsafe()` 才开启）；另有 CJK 扩展（中文换行优化，加分项） | ✅ 全部确认 / 版本号上浮 |
| 4 | `bluemonday` | 路径 `github.com/microcosmohq/bluemonday`，v1.0+，UGCPolicy 允许 table | **路径错误**：正确路径 `github.com/microcosm-cc/bluemonday`（microcosmohq 在 goproxy.cn / proxy.golang.org 均不存在，已实测）；最新 **v1.0.27**；**UGCPolicy 源码 policies.go:217 确认调用 `AllowTables()`**；传递依赖 = `aymerick/douceur` + `golang.org/x/net v0.26.0` + `gorilla/css`（均纯 Go 小库）；**v1.0.0–v1.0.25 已被作者 retract**（"only latest is to be depended upon"）；BSD-3-Clause | ⚠️ **路径必须修正** / UGCPolicy 表格确认 |
| 5 | stdlib `encoding/csv` | 动态 schema，不引入 gocsv | 成立；**但中文金融环境 CSV 常为 GBK/GB18030 编码 + UTF-8 BOM**，stdlib csv 只吃 UTF-8——需解码层（见 §3.1） | ✅ 成立 / ⚠️ 编码风险必须设计 |
| 6 | stdlib `crypto/hmac`+`sha256` | L1 确定性脱敏哈希 | 成立；**可 join 性取决于哈希前规范化**（NFC/trim/全角空格/手机号 +86 形态），不规范化则同名不同形态 → 不同哈希 → join 断裂（见 §3.2） | ✅ 成立 / ⚠️ 规范化规则必须进 spec |
| 7 | stdlib `text/template` | KPI 值绑定 → markdown → HTML | 成立；goldmark 默认转义 raw HTML + bluemonday 兜底 = 双层防御确认；顺序 **绑定→渲染→消毒** 正确（bluemonday README 原话 "always run bluemonday after any other processing"） | ✅ 确认 |

---

## 1. CRITICAL：本机构建环境阻塞（go-duckdb / CGO）

**事实链**（全部实测）：
- go-duckdb v2 通过 CGO 静态链接预编译 DuckDB C++ 库（Windows amd64 有预编译 .a，捆绑扩展 ICU/JSON/Parquet/Autocomplete）
- Windows 官方构建方式 = 安装 **MSYS2** 后 `pacman -S mingw-w64-ucrt-x86_64-gcc`，再把 `C:\msys64\ucrt64\bin` 加入 PATH（go-duckdb README 原文）
- **本机实测：`gcc: command not found`**（CGO_ENABLED=1, CC=gcc 均为默认，但无编译器）
- 后果：go.mod 一旦引入 go-duckdb，**本机任何全仓 `go build`/`go vet ./...` 都会失败**（不只是 duckdb 包），开发流中断

**修复选项**（推荐 A）：
| 选项 | 做法 | 代价 |
|------|------|------|
| **A（推荐）** | 安装 MSYS2 + mingw-w64-ucrt-x86_64-gcc，PATH 注入 | 一次性 ~15 分钟安装；与 Docker Linux 构建（镜像自带 gcc）无冲突 |
| B | v1.0 期间所有构建走 Docker（dev container），本机只跑前端/编辑 | 日常 go test 迭代慢一个数量级，伤开发体验 |
| C | duckdb 隔离成独立 module / build tag | module 拆分破坏单仓 hermetic 叙事，不值得 |

**决策请求**: 选项 A 需要用户在本机执行安装（`! winget install MSYS2.MSYS2` 或官网安装器 + pacman 命令）。这是 v1.0 Phase 14 第一个 spike 的前置条件。

---

## 2. 逐依赖详录

### 2.1 go-duckdb/v2（选型确认，版本上浮）

- **版本**: v2.4.3（goproxy.cn 版本线实测）。v2.4.2 起捆绑 **DuckDB v1.4.1** 引擎
- **维护重大利好**: README 顶部公告——驱动维护权由 Marc Boeker + DuckDB 团队联合**移交至 DuckDB 官方**（"fully transfer the maintenance and day-to-day work of the driver to the DuckDB team"），MIT 不变，且 DuckDB 已宣布 LTS（v1.4 LTS + v1.5 双线）。**官方接手 = 长期维护风险显著下降**，比 STACK.md 撰写时的判断更强
- **`ATTACH '...' AS ds_xxx (READ_ONLY)` 语法**: DuckDB 官方文档确认合法（`ACCESS_MODE: READ_ONLY`）
- **⚠️ 附加会话不持久**: "attachment definitions are not persisted between sessions"——每个新连接必须重新 ATTACH。**解法已在驱动 README 里**: `duckdb.NewConnector(dsn, func(execer driver.ExecerContext) error { /* boot queries: ATTACH + SET */ })` 回调即为此设计，KPI 引擎必须用 `NewConnector` 而非裸 `sql.Open`
- **事务语义**: 单事务只能写一个 attached db（我们导入写 ds_*、KPI 只读，天然合规）
- **⚠️ 连接生命周期**: 临时对象随连接存活；连接池缓存 idle conn 会让临时表"活得比预期久"→ `db.SetMaxIdleConns(0)`
- **⚠️ WAL 同步**: 持久库必须正确 `Close()`（connector + db），否则 WAL 未刷盘 → **导入数据丢失风险**。导入管线的 shutdown hook 是硬要求
- **⚠️ JSON 类型扫描**: v2 破坏性变更——DuckDB JSON 列不能扫进 string/[]byte，需 SQL 侧 `::VARCHAR`。若 KPI 结果以 JSON 存储，注意 cast
- **二进制体积**: 静态链接后 Go 二进制从几 MB 涨到 **~60–100MB**（pkg.go.dev 及社区实测口径）。hermetic 单二进制叙事仍成立，Docker 镜像相应变大，可接受；日后可用 `-tags=duckdb_use_lib` 动态链接换体积
- **Arrow 重依赖默认关闭**（`-tags=duckdb_arrow` 才启用）——默认依赖面干净 ✓
- **可选彩蛋**: `ATTACH ... (ENCRYPTION_KEY ...)` 支持 AES-256-GCM 落盘加密（官方注明暂未达 NIST 合规）——v1.0 不做，记录为 v1.1 选项

### 2.2 robfig/cron/v3（选型确认，事实修正）

- **版本线**: 仅 `v3.0.0-rc1` / `v3.0.0`（2019-06）/ `v3.0.1`（2020）。**STACK.md "released 2024" 错误**，v3.0.1 距今 6 年无新版本 = 休眠
- **休眠风险定性**: feature-complete、纯 Go、**零依赖**、被海量项目部署（事实标准）。我们只用 2 个场景（页面重渲染 + TTL 清扫），API 面极小且会用接口包装调度器 → **接受休眠**；活跃备选 `go-co-op/gocron/v2`（v2.22.0，活跃）留作替换路径
- **API 源码逐行确认**（v3.0.1 tarball grep）:
  - `WithLocation(loc)` — option.go:11 ✓
  - `WithChain(wrappers...)` — option.go:34 ✓
  - `Recover(logger)` — chain.go:38 ✓；`SkipIfStillRunning(logger)` — chain.go:78 ✓
  - `CRON_TZ=` 前缀解析 — parser.go:95 ✓（`TZ=` legacy 也支持）
  - `(c *Cron) Stop() context.Context` — cron.go:323 ✓（优雅关停：ctx 在运行中 job 完成后 cancel）
- **关键行为**: v3 **默认不 recover panic**——`WithChain(Recover(logger))` 必须显式配置（我们的设计已含此条 ✓）

### 2.3 goldmark（全部声明确认）

- 最新 **v1.8.5**；CommonMark **0.31.2**（最新规范）合规；MIT
- **零依赖**: README 明示 "Depends only on standard libraries"——比 STACK.md 声明的 "zero transitive deps beyond x/text" 还干净（**连 x/text 都不需要**）
- **安全默认**: "By default, goldmark does not render raw HTML or potentially-dangerous URLs"——不开启 `html.WithUnsafe()` 时 raw HTML 全部转义。即 **bluemonday 之前已有第一层防御**，双层防线成立
- GFM 扩展（Table/Strikethrough/Linkify/TaskList）内置；**CJK 扩展**（`WithEastAsianLineBreaks`/`WithEscapedSpace`）对中文月报排版有直接价值，建议 v1.0 启用
- 性能对标 cmark（C 参考实现），`go test --fuzz` 测试过

### 2.4 bluemonday（路径修正后选型确认）

- **⚠️ 正确模块路径: `github.com/microcosm-cc/bluemonday`**（STACK.md 的 `microcosmohq` 不存在——两个代理实测 404，源 README go-import 元数据确认 microcosm-cc）
- 最新 **v1.0.27**；BSD-3-Clause（Apache-2.0 兼容）；3.7k stars 活跃维护
- **⚠️ 版本策略**: 作者已 retract `[v1.0.0, v1.0.25]`——"only latest is to be depended upon"。**必须显式 pin v1.0.27**（我们的 frozen-lock 规则本来就要求 pin，天然兼容），升级时直接跳最新
- **UGCPolicy 含表格**: 源码 policies.go:217 `p.AllowTables()` 确认；`AllowTables()`/`AllowLists()`/`AllowImages()` 辅助方法存在。STACK.md 建议的手工 `AllowElements("table",...)` 补充**非必需**（无害可留作显式声明）
- **并发模型**: policy 构建**非**goroutine 安全，构建后 `Sanitize` 并发安全——STACK.md 的 sync.Once 构建模式正确 ✓
- **依赖闭包**: `aymerick/douceur`（CSS 解析）+ `golang.org/x/net v0.26.0` + `gorilla/css v1.0.1`（间接）。**STACK.md 若有"零传递依赖"印象需修正**——3 个纯 Go 小库，无 CGO，风险可忽略；x/net 我们已有 v0.21.0（indirect），会被抬到 v0.26.0，无痛
- **顺序铁律**: README 原话 "You should always run bluemonday **after** any other processing" → 管线定死 `text/template 绑定 → goldmark 渲染 → bluemonday 消毒`

### 2.5 stdlib csv（选型确认 + §3.1 编码设计要求）

- 动态 schema（列名运行时发现）不需要 gocsv 的 struct-tag 绑定——拒绝 gocsv 的理由成立
- `encoding/csv` 只接受 UTF-8——见 §3.1 GBK 风险

### 2.6 既有 go.mod 排查（无新增风险）

- `go.mongodb.org/mongo-driver/v2 v2.8.0` 是 `internal/retriever/mongo` 的**合法直接依赖**（Mongo retriever 存在于代码中），非僵尸依赖 ✓
- `golang.org/x/text v0.22.0` 已是直接依赖 → **GBK 解码零新增依赖** ✓
- `golang.org/x/net v0.21.0`（indirect）→ bluemonday 需抬升至 v0.26.0 ✓ 无痛
- 尚未引入 cron / goldmark / bluemonday / duckdb（符合预期——v1.0 才加）

---

### 2.7 后续决策：sqlite 驱动 CGO 化（modernc.org/sqlite → mattn/go-sqlite3）

**2026-08-21 用户决策**：既定引入 CGO（go-duckdb，§1），OLTP sqlite 驱动同步换成 CGO 版本换取性能。

- **版本**: `github.com/mattn/go-sqlite3` **v1.14.50**（goproxy.cn 实测最新；捆绑 SQLite amalgamation **3.53.4**——比 modernc v1.56.0 跟踪更直接）
- **动机**: 审计 INSERT 路径 ~1.5-2× 性能 + 链接真正的 SQLite 消除 C→Go 转译（modernc）的行为怪癖；CGO 前置已由 go-duckdb 引入，**边际成本为零**（同一 MSYS2 gcc、同一 Dockerfile 翻转）
- **迁移点（带入 Phase spec）**:
  1. driver name `sqlite`（modernc）→ `sqlite3`（mattn）；`internal/store/sqlite` import 与 `sql.Open` 参数替换
  2. DSN pragma 语法差异：modernc `?_pragma=busy_timeout(5000)` → mattn `?_busy_timeout=5000&_journal_mode=WAL`（schema.go 与 store DSN 逐处核对）
  3. **Dockerfile.backend `CGO_ENABLED=0` → `1`**：build 阶段加 build-base（alpine: gcc + musl-dev），运行镜像保留 libc——该改动本来就被 go-duckdb 要求，两项合并一次完成
  4. Windows 本机：同一 MSYS2 gcc 前置（与 go-duckdb 共用，无新增）
  5. 无 gcc 机器上全量构建进一步不可用——与 go-duckdb 同口径，已被接受

## 3. 实现级风险（必须带入 Phase spec，非选型问题）

### 3.1 中文 CSV 编码（GBK/GB18030 + BOM）— 导入管线硬前提
金融企业系统导出 CSV 常见三种形态：**GBK/GB18030 编码**（老核心系统默认）、**UTF-8 with BOM**（Excel 导出）、纯 UTF-8。stdlib csv 遇 GBK 直接报错或乱码。
**设计要求**: 导入读取原始字节 → ① 剥离 UTF-8 BOM（`\xEF\xBB\xBF`，不剥则首列列名带 BOM 导致 schema 匹配失败）→ ② `utf8.Valid` 检测 → ③ 非 UTF-8 则用 `x/text/encoding/simplifiedchinese.GB18030.NewDecoder()` 解码（**GB18030 是 GBK 超集，一个解码器通吃**）→ ④ 再喂 csv.Reader。**零新增依赖**（x/text v0.22.0 已在）。

### 3.2 L1 HMAC 哈希前规范化 — 可 join 性的生命线
确定性哈希的目的是同名实体可 join/去重。若不做规范化，「张三 / `张三␣`（尾空格）/ `＋8613800000000` / `138-0000-0000`」会产出不同哈希 → join 断裂、去重失效。
**设计要求**（进脱敏 spec，哈希前依次应用）: NFC Unicode 规范化（`x/text/unicode/norm`）→ 去首尾空白（含全角空格 `\u3000`）→ 字段特定规则（证件号大写去空格；手机号去 `+86`/`86` 前缀与分隔符）。**站点盐 + 数据集盐 + 字段名域分离**的设计维持不变。

### 3.3 模板注入顺序 — 已被双层防御覆盖，格式化仍需约束
管线 `绑定(text/template, 数据只作参数、绝不拼进模板源) → goldmark(默认转义 raw HTML) → bluemonday(UGCPolicy 兜底)`。**KPI 数值必须服务端格式化**（`%d`/`%.2f`/千分位），不允许原始字符串进表格单元格——否则 `1.23 | </td>` 类值可破坏 markdown 表格结构（无 XSS，但破版）。

### 3.4 DuckDB hermetic 陷阱 — 关闭扩展自动加载
go-duckdb 默认 `autoinstall/autload known extensions = 启用`。查询若触碰未捆绑扩展会**尝试网络下载** → 破坏 hermetic 部署 + 内网环境报错。捆绑集（ICU/JSON/Parquet/Autocomplete）覆盖 v1.0 全部需求。
**设计要求**: NewConnector boot queries 里固定 `SET autoinstall_known_extensions=false; SET autoload_known_extensions=false;`（与重 ATTACH 同一回调）。

### 3.5 Windows 文件锁 — TTL 清理必须先 DETACH
DuckDB 实例持有 attached 文件句柄；**Windows 上文件被打开时 `os.Remove` 直接失败**。TTL 清扫流程必须 `DETACH ds_xxx` → 再删文件（且 DETACH 不能作用于 default db——数据集永远以别名 attach，default 留 `:memory:`）。cron 的 TTL job 与导入 job 并发时，DETACH 需与 KPI 查询协调（读已开连接的查询结束后再 DETACH，或 TTL 只删"已过期且无活跃引用"的数据集，靠引用计数/时间窗判定）。

---

## 4. STACK.md 修正清单（本次已应用 ✅ / 待后续）

| # | 位置 | 原文 | 修正 | 状态 |
|---|------|------|------|------|
| 1 | L29/L459/L792/L876 | `github.com/microcosmohq/bluemonday` | `github.com/microcosm-cc/bluemonday` | ✅ 已改 |
| 2 | L378 | v3.0.1 (released 2024) | v3.0.1（2020 年发布；v3 线 2019-06 起，此后休眠但 feature-complete 零依赖） | ✅ 已改 |
| 3 | L448 | goldmark "zero transitive deps beyond golang.org/x/text" | goldmark 完全零依赖；bluemonday 传递依赖 = douceur + gorilla/css + x/net v0.26.0 | ✅ 已改 |
| 4 | L12/L26/L155/L873 | v2.4.0 line | v2.4.3（捆绑 DuckDB v1.4.1；CGO 必需 → Windows 开发机需 MSYS2 gcc，见 STACK-VERIFICATION.md） | ✅ 已改 |
| 5 | L15/L29/L444/L459 | goldmark v1.7+ / bluemonday v1.0+ | goldmark v1.8.5 / bluemonday v1.0.27（v1.0.25 及以下被 retract，必须 pin） | ✅ 已改 |
| 6 | 全文 | — | 顶部加指向本报告的审计注记 | ✅ 已加 |

---

## 5. 来源

- **go-duckdb**: 官方 README（GitHub, 2026-08 快照——含 DuckDB 团队接管公告/Windows MSYS2 指引/连接生命周期/Arrow opt-in/JSON 破坏性变更）；goproxy.cn 版本线（v2.4.3 最新）
- **DuckDB ATTACH 语法**: duckdb.org/docs/stable/sql/statements/attach（READ_ONLY 合法、会话不持久、单事务单写库、ENCRYPTION_KEY 选项及 NIST 注记）
- **robfig/cron**: 官方 README（v3 发布 2019-06、panic 不默认 recover）+ v3.0.1 源码 tarball grep（option.go/chain.go/cron.go/parser.go 行号证据）
- **goldmark**: 官方 README（CommonMark 0.31.2、"Depends only on standard libraries"、默认不渲染 raw HTML、CJK 扩展、GFM 构成）；goproxy.cn（v1.8.5 最新）
- **bluemonday**: 官方 README（UGCPolicy 描述、AllowTables 助手、"always run after"、BSD-3）+ v1.0.27 模块源码（policies.go:217 `AllowTables()`）+ go.mod（依赖闭包、retract 声明）；goproxy.cn 路径验证（microcosm-cc 存在 / microcosmohq 404）
- **go-duckdb 二进制体积**: pkg.go.dev 及 GitHub issue #116 社区口径（~60–100MB）
- **本机环境**: `gcc: command not found` 实测；go.mod / internal 源码 grep（mongo-driver 用途、x/text 现状）
