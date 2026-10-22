#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# verify-phase-49.sh — Phase 49 (Hermetic Demo) 端到端门禁
#
# 三件必须明写的事（Phase 49 / DEMO-03 / SC5）：
#
# 1) 本门禁不依赖容器、模型服务或录屏渲染工具是否安装。它只断言**产物与脚本
#    存在**（docs/demo.gif 的 GIF magic bytes、非空、二进制内不含 key 形态字节；
#    scripts/record-demo.sh 存在且可解析），绝不探测 docker / ollama / asciinema /
#    agg / ffmpeg 是否在场 —— 否则门禁会因为某台宿主缺一个渲染器而退化成软门禁。
#
# 2) 测试专用 embedding 替身的边界（唯一允许的替身，且只在门禁内部）：
#    - 端点在 `$WORK` 下生成，由 WSL 内的 python3 执行，**只 bind 127.0.0.1**；
#      端口由内核分配（0），实际端口经 `$WORK/embed-port.txt` 传递，绝不硬编码。
#    - `BGE_M3_URL` **只经 `env` 注入子进程**，本脚本不 export 它；生产二进制
#      （internal/embedder/bge 与全部生产装配路径）零改动，由 P8 的
#      `git diff --quiet -- internal/embedder/bge` 断言。
#    - 向量必须是 hashing vectorizer（token 哈希落桶 + 符号位 + L2 归一化），
#      **按键由文本决定**，绝不返回纯随机向量，也绝不返回与文本无关的单一固定
#      向量（Pitfall 10 / Pitfall 17：固定向量会让所有 cosine 完全相同、排序退化；
#      随机向量会被 pipeline 的 retrievalScoreThreshold=0.3 永久拦成拒答，门禁就
#      只能验到拒答）。为了让端到端链路能走到 LLM（否则 stub 的 Verdict 根本不会
#      产生），替身在归一化向量上加了一个**所有文本共享的基底分量**，使绝对相似度
#      高到足以越过 0.3 阈值；文本相关的残差分量仍然决定**相对排序**，因此 TopK
#      与排序断言的区分度不受影响。这是显式的测试夹具取舍，已记入
#      49-04-SUMMARY.md 的 flagged assumption。
#    - 门禁输出必须打印 `using test-only embedding double`，供 P9 自证分区 grep。
#
# 3) Go 分区一律经 `wsl()` 包装：本仓库在 Windows 宿主上全量 `go test ./...` 必红
#    （duckdb-go-bindings 的预编译 Windows 静态库与本机 msys2 ucrt64 gcc 的 TLS ABI
#    不匹配：`ld.exe: undefined reference to __emutls_v._ZSt11__once_call`）。同一
#    脚本在 Linux runner 上直通（`wsl.exe` 缺席时自动退化为本机执行）。
#
# 局部运行开关 `--only P1,P2` 是**排障工具，不是逃生门**：它只影响哪些分区被执行
# 与横幅文案，绝不影响 EXPECTED_PARTITIONS 的定义；带该开关时横幅打印
# `=== Phase 49 Gate: PARTIAL RUN (<列表>) ===` 并以 0 退出（仅当所有被选分区都
# 落 ok），**永不打印 ALL PASSED**；任一被选分区未落 ok 即以非零退出。
#
# 用法：
#   bash verify-phase-49.sh                      # 完整门禁（九分区）
#   bash verify-phase-49.sh --only P1,P2,P3      # 局部排障（永不打印 ALL PASSED）
#
set -uo pipefail

# 不启用 errexit：每个测量步骤都必须显式断言，否则失败时无法打印是哪个断言挂了。
export MSYS_NO_PATHCONV=1

# GATE_ROOT 只在 P9 的变异演练里被显式覆盖（副本脚本 + 真实仓库根）。
ROOT="${GATE_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"

RED=$'\033[31m'
GREEN=$'\033[32m'
YELLOW=$'\033[33m'
NC=$'\033[0m'

pass() { printf '%sPASS%s %s\n' "$GREEN" "$NC" "$1"; }
info() { printf '%sINFO%s %s\n' "$YELLOW" "$NC" "$1"; }
fail() {
  printf '%sFAIL%s %s\n' "$RED" "$NC" "$1"
  exit 1
}

WORK="$(mktemp -d)"
SERVER_PIDS=""
CACHE_DIR_CREATED=""
cleanup() {
  local p
  for p in $SERVER_PIDS; do
    kill "$p" 2>/dev/null || true
    wait "$p" 2>/dev/null || true
  done
  if [ -n "$CACHE_DIR_CREATED" ] && [ -d "$CACHE_DIR_CREATED" ]; then
    rm -rf "$CACHE_DIR_CREATED" 2>/dev/null || true
  fi
  # GATE_KEEP=1 只保留 $WORK 供排障（不影响任何判定），默认清理。
  if [ "${GATE_KEEP:-0}" = "1" ]; then
    printf 'kept work dir: %s\n' "$WORK"
    return 0
  fi
  rm -rf "$WORK" 2>/dev/null || true
}
trap cleanup EXIT

mkdir -p "$WORK/ran"
EXPECTED_PARTITIONS=9

# 工作树快照：P9 的"门禁不留产物"断言只应该判「本次运行**新增**了什么」。
# 开发机的工作树本来就有无关脏文件（备份 db、临时脚本…），拿绝对干净度当
# 判据会把既存垃圾误判成门禁产物，从而在健康树上假红。
GATE_TREE_BEFORE="$WORK/tree-before.txt"
( cd "$ROOT" && git status --porcelain 2>/dev/null | sort ) >"$GATE_TREE_BEFORE" 2>/dev/null || : >"$GATE_TREE_BEFORE"
RAN=0
RAN_LIST=()
ONLY=""

# ---------- 参数解析（--only P1,P2 / --only=P1,P2） ----------
while [ $# -gt 0 ]; do
  case "$1" in
    --only)
      [ $# -ge 2 ] || fail "--only requires a comma-separated partition list, e.g. --only P1,P2"
      ONLY="$2"
      shift 2
      ;;
    --only=*)
      ONLY="${1#--only=}"
      shift
      ;;
    -h | --help)
      sed -n '2,45p' "${BASH_SOURCE[0]}"
      exit 0
      ;;
    *)
      fail "unknown argument: $1 (supported: --only P1,P2)"
      ;;
  esac
done
ONLY="$(printf '%s' "$ONLY" | tr -d ' ')"
if [ -n "$ONLY" ] && [ "${ONLY#P}" = "$ONLY" ]; then
  # 允许 `--only 1,2` 这种写法，补齐 P 前缀
  ONLY="$(printf '%s' "$ONLY" | sed -E 's/(^|,)P?([0-9]+)/\1P\2/g')"
fi

partition_selected() {
  local pn="$1"
  if [ -z "$ONLY" ]; then
    return 0
  fi
  case ",$ONLY," in
    *",P$pn,"*) return 0 ;;
    *) return 1 ;;
  esac
}

# ---------- 分区三件套（形态逐字沿用 verify-phase-47.sh / verify-phase-48.sh） ----------
# 分区标识在 begin 侧写数字（`begin_partition 1 …`）、在 ok/done 侧写 P 前缀
# （`ok_partition "P1" …`），因此三件套统一归一化成裸数字再落到 $WORK/ran。
_pn_num() {
  printf '%s' "${1#P}"
}

begin_partition() {
  local pn
  pn="$(_pn_num "$1")"
  local desc="$2"
  printf '\n--- P%s: %s\n' "$pn" "$desc"
  : >"$WORK/ran/$pn.begin"
}

ok_partition() {
  local pn
  pn="$(_pn_num "$1")"
  local log="$2"
  shift 2
  local token missing=""
  [ -f "$log" ] || fail "partition P$pn has no probe log at $log — a partition that cannot show its evidence is not a pass"
  [ -s "$log" ] || fail "partition P$pn produced an empty probe log ($log) — empty evidence is indistinguishable from 'did not run'"
  for token in "$@"; do
    grep -qF -- "$token" "$log" || missing="$missing $token"
  done
  if [ -n "$missing" ]; then
    fail "partition P$pn cannot deposit its success tokens: the probe log $log does not contain '$missing' (required: $*)"
  fi
  printf 'P%s: tokens deposited (%s)\n' "$pn" "$*"
  : >"$WORK/ran/$pn.ok"
}

done_partition() {
  local pn
  pn="$(_pn_num "$1")"
  [ -f "$WORK/ran/$pn.begin" ] || fail "partition P$pn never called begin_partition — the closing call has no witness"
  [ -f "$WORK/ran/$pn.ok" ] || fail "partition P$pn never deposited its success tokens — done_partition without ok_partition is a fake pass"
  RAN=$((RAN + 1))
  RAN_LIST+=("P$pn")
  pass "P$pn complete"
}
# drill:done:registry — 每个 done_partition 调用点前都有这一行注释，
# P9 的 D9 演练靠它整行删除来制造「注册表与调用点不一致」。本行不得删除。

# ---------- 路径与 Go 包装 ----------
native() {
  # 交给原生 Windows 二进制（curl 之外的 git 等）的路径
  if command -v cygpath >/dev/null 2>&1; then
    cygpath -w "$1"
  else
    printf '%s' "$1"
  fi
}

REPO_WSL=""

# to_wsl_path <path>：Cygwin/Git-Bash 路径 → WSL 路径。
# 不依赖 `wsl.exe wslpath`：在 Cygwin bash 下它会把 `/d/...` 再映射一层，
# 得到 `/mnt/d/d/CODE/...`（实测 cd 失败）。这里用 cygpath 拿到 Windows 路径
# 后做确定性变换（C:\x → /mnt/c/x），两种宿主上都能预测。
to_wsl_path() {
  local p="$1" win drive rest
  if command -v cygpath >/dev/null 2>&1; then
    win="$(cygpath -w "$p" 2>/dev/null)"
    case "$win" in
      [A-Za-z]:\\*)
        drive="$(printf '%s' "$win" | cut -c1 | tr 'A-Z' 'a-z')"
        rest="$(printf '%s' "$win" | cut -c3- | sed 's|\\|/|g')"
        printf '/mnt/%s%s' "$drive" "$rest"
        return 0
        ;;
    esac
  fi
  printf '%s' "$p"
}

if command -v wsl.exe >/dev/null 2>&1; then
  REPO_WSL="$(to_wsl_path "$ROOT")"
  [ -n "$REPO_WSL" ] || REPO_WSL=""
fi

wslpath_of() {
  to_wsl_path "$1"
}

# wsl_run <shell command>：在仓库根的 WSL 视角下执行；wsl.exe 缺席时本机直通。
wsl_run() {
  local cmd="$1"
  if [ -n "$REPO_WSL" ]; then
    wsl.exe -e bash -lc "cd '$REPO_WSL' && $cmd"
  else
    ( cd "$ROOT" && bash -lc "$cmd" )
  fi
}

if [ -z "$REPO_WSL" ]; then
  info "wsl.exe absent — running Go commands natively (Linux runner path)"
fi

# run_go_test <log> <go test 参数...>
# 退出码权威 + 日志非空 + 日志里必须出现 ok 行（空跑/被 skill 吞掉都要判失败）。
run_go_test() {
  local log="$1"
  shift
  # 逐参数 printf %q 重新引用：`$*` 会把 `-run 'A|B'` 的引号吃掉，导致 `|`
  # 在 WSL 侧退化成管道（实测：单引号丢失 → go test 报 flag 错误）。
  local cmd="go test"
  local a
  for a in "$@"; do
    cmd="$cmd $(printf '%q' "$a")"
  done
  cmd="$cmd -count=1"
  wsl_run "$cmd" >"$log" 2>&1
  local rc=$?
  if [ "$rc" -ne 0 ]; then
    echo "run_go_test: 'go test $*' exited $rc" >>"$log"
    return 1
  fi
  if ! grep -qE '^ok[[:space:]]' "$log"; then
    echo "run_go_test: 'go test $*' produced no 'ok' line — an empty run is not a pass" >>"$log"
    return 1
  fi
  return 0
}

# require_test_pass <log> <TestName>：断言具体测试名真的 PASS，而不是只靠退出码。
require_test_pass() {
  local log="$1"
  local name="$2"
  grep -qE -- "^[[:space:]]*--- PASS: ${name}([[:space:]]|$)" "$log" ||
    fail "expected test $name to PASS in $log, but no '--- PASS: $name' line was found (an assertion that never ran cannot be counted as a pass)"
}

# require_test_fail <log> <TestName>：断言具体测试名真的 FAIL（负向断言的证据）。
require_test_fail() {
  local log="$1"
  local name="$2"
  grep -qE -- "^[[:space:]]*--- FAIL: ${name}([[:space:]]|$)" "$log" ||
    fail "expected test $name to FAIL in $log, but no '--- FAIL: $name' line was found"
}

CORPUS_EXPECTED=218

# ---------- 测试专用 embedding 替身端点（生成到 $WORK，WSL 内 python3 执行） ----------
write_embed_double() {
  cat >"$WORK/embed-double.py" <<'PY'
"""Test-only embedding double for verify-phase-49.sh.

NOT a production component: it never ships, it only listens on 127.0.0.1, and the
gate overrides BGE_M3_URL only in its own child processes.
"""
import hashlib
import json
import math
import sys

from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

DIM = 1024
# 所有文本共享的基底分量。端到端链路必须真的走到 LLM，否则 stub 的 Verdict
# （含 wire 上的 "mode":"stub"）根本不会产生；pipeline 的
# retrievalScoreThreshold=0.3 会把纯词面哈希的相似度（长 chunk 下 ~0.1）永久拦成
# 拒答。基底抬高绝对相似度，残差保住相对排序。
BASE_WEIGHT = 0.90
RESIDUAL_WEIGHT = 0.10


def _tokens(text):
    chars = [c for c in text if not c.isspace()]
    toks = list(chars)
    toks += [chars[i] + chars[i + 1] for i in range(len(chars) - 1)]
    return toks or [text]


def _hashing(text):
    v = [0.0] * DIM
    for tok in _tokens(text):
        h = hashlib.sha256(tok.encode("utf-8")).digest()
        idx = int.from_bytes(h[:4], "big") % DIM
        sign = 1.0 if (h[4] & 1) else -1.0
        v[idx] += sign
    norm = math.sqrt(sum(x * x for x in v))
    if norm == 0.0:
        return [0.0] * DIM
    return [x / norm for x in v]


def vec_for(text):
    residual = _hashing(text)
    base = 1.0 / math.sqrt(DIM)
    v = [BASE_WEIGHT * base + RESIDUAL_WEIGHT * r for r in residual]
    norm = math.sqrt(sum(x * x for x in v))
    return [x / norm for x in v]


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, fmt, *args):  # 静默 access log
        return

    def _json(self, code, payload):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self):
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length else b"{}"
        try:
            req = json.loads(raw.decode("utf-8"))
        except Exception:
            self._json(400, {"error": "invalid JSON body"})
            return
        inp = req.get("input")
        texts = []
        if isinstance(inp, str):
            texts = [inp]
        elif isinstance(inp, list):
            if all(isinstance(x, str) for x in inp):
                texts = list(inp)
            elif all(isinstance(x, dict) for x in inp):
                texts = [x.get("text", "") for x in inp]
            else:
                self._json(400, {"error": "input list must be strings or {text} objects"})
                return
        else:
            self._json(400, {"error": "input must be a string or a list"})
            return
        if not texts or any(not isinstance(t, str) or t == "" for t in texts):
            self._json(400, {"error": "empty input text is not embeddable"})
            return
        self._json(200, {"data": [{"embedding": vec_for(t), "index": i} for i, t in enumerate(texts)]})


def main():
    srv = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    port = srv.server_address[1]
    with open(sys.argv[1], "w", encoding="utf-8") as fh:
        fh.write(str(port))
    info_line = (
        "using test-only embedding double on 127.0.0.1:%d "
        "(production binary untouched; BGE_M3_URL is overridden only in this script's child processes)\n" % port
    )
    sys.stdout.write(info_line)
    sys.stdout.flush()
    # 直接在主线程序列化服务：早前用「后台线程 + 等 stdin EOF」的写法在驱动器里
    # 会立刻退出（stdin 已关闭 ⇒ read() 立即返回 ⇒ shutdown），后端随即撞上
    # connection refused。进程由驱动器 kill 收尾。
    try:
        srv.serve_forever(poll_interval=0.2)
    except KeyboardInterrupt:
        pass
    finally:
        srv.server_close()


if __name__ == "__main__":
    main()
PY
}

# ---------- live 驱动器：在 WSL 内起替身 + 后端，跑 P4 / P6 的真实 HTTP 断言 ----------
write_live_driver() {
  local work_wsl
  work_wsl="$(wslpath_of "$WORK")"
  cat >"$WORK/live-driver.sh" <<DRIVER
set -uo pipefail
WORK_WSL='$work_wsl'
REPO_WSL='${REPO_WSL:-}'
cd "\$REPO_WSL" || exit 1

RAN_P4=0
RAN_P6=0
case ",\${GATE_ONLY:-}," in
  *",P4,"*) RAN_P4=1 ;;
esac
if [ -z "\${GATE_ONLY:-}" ]; then RAN_P4=1; fi
case ",\${GATE_ONLY:-}," in
  *",P6,"*) RAN_P6=1 ;;
esac
if [ -z "\${GATE_ONLY:-}" ]; then RAN_P6=1; fi
if [ "\$RAN_P4" -eq 0 ] && [ "\$RAN_P6" -eq 0 ]; then exit 0; fi

: > "\$WORK_WSL/p4.log"
: > "\$WORK_WSL/p6.log"

# 1) 替身端点
python3 "\$WORK_WSL/embed-double.py" "\$WORK_WSL/embed-port.txt" > "\$WORK_WSL/embed-double.log" 2>&1 &
EMBED_PID=\$!
for _i in \$(seq 1 100); do
  [ -s "\$WORK_WSL/embed-port.txt" ] && break
  sleep 0.1
done
[ -s "\$WORK_WSL/embed-port.txt" ] || { echo "FAIL live: embedding double did not report a port" >&2; kill \$EMBED_PID 2>/dev/null; exit 1; }
EMBED_PORT="\$(tr -d '[:space:]' < "\$WORK_WSL/embed-port.txt")"
grep -q 'using test-only embedding double' "\$WORK_WSL/embed-double.log" && \
  echo "using test-only embedding double on 127.0.0.1:\$EMBED_PORT" >> "\$WORK_WSL/p0.log"

# 2) 构建后端二进制（不污染仓库：产物落 \$WORK）
go build -o "\$WORK_WSL/skillforge-server" ./cmd/server > "\$WORK_WSL/p4-build.log" 2>&1
if [ \$? -ne 0 ]; then
  echo "FAIL live: go build ./cmd/server failed — see \$WORK_WSL/p4-build.log" >> "\$WORK_WSL/p4.log"
  kill \$EMBED_PID 2>/dev/null
  exit 0
fi

# 3) 空闲端口（内核分配后立刻释放，交给后端绑定）
PORT="\$(python3 - <<'PYPORT'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PYPORT
)"

# 4) 起后端：无 GLM_API_KEY / LOCAL_API_KEY，RETRIEVER_BACKEND=embedded
# SKILLFORGE_MCP_SERVICE_SKILL 是 phase 41 的故意必填项（D-TRUST-02），缺了
# 直接 Fatalf —— 这是门禁在首次 live 运行时抓到的真实 quickstart 断点，
# README 的 hermetic 小节必须写明这一步（见 49-04-SUMMARY 的 Deviations）。
mkdir -p "\$WORK_WSL/cache" "\$WORK_WSL/rundir"
env -u GLM_API_KEY -u LOCAL_API_KEY \
  SERVER_ADDR="127.0.0.1:\$PORT" \
  RETRIEVER_BACKEND=embedded \
  SKILLFORGE_MCP_SERVICE_SKILL=simple-qa \
  BGE_M3_URL="http://127.0.0.1:\$EMBED_PORT/v1/embeddings" \
  SKILLFORGE_VECTOR_CACHE_DIR="\$WORK_WSL/cache" \
  SQLITE_DSN="\$WORK_WSL/p4.db" \
  "\$WORK_WSL/skillforge-server" > "\$WORK_WSL/p4-server.log" 2>&1 &
SERVER_PID=\$!
READY=0
for _i in \$(seq 1 600); do
  if ! kill -0 \$SERVER_PID 2>/dev/null; then break; fi
  code="\$(curl -s -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:\$PORT/api/chat" -H 'Content-Type: application/json' -d '{"query":"ping","skill":"simple-qa"}' 2>/dev/null || true)"
  if [ "\$code" != "000" ] && [ -n "\$code" ]; then READY=1; break; fi
  sleep 0.3
done
if [ "\$READY" -ne 1 ]; then
  echo "FAIL P4: backend did not become ready in 180s — see \$WORK_WSL/p4-server.log" >> "\$WORK_WSL/p4.log"
  kill \$SERVER_PID \$EMBED_PID 2>/dev/null
  exit 0
fi

# 5) P4：无 key 启动 + 一轮问答 + 合法 Verdict + wire 上的 mode
grep -q 'STUB LLM MODE' "\$WORK_WSL/p4-server.log" && \
  echo "P4-NO-KEY-STARTED: server booted with no GLM_API_KEY/LOCAL_API_KEY and logged the stub WARN" >> "\$WORK_WSL/p4.log"

curl -sS -N --max-time 120 -X POST "http://127.0.0.1:\$PORT/api/chat" \
  -H 'Content-Type: application/json' \
  -d '{"query":"差旅住宿费的报销标准是什么？","skill":"simple-qa"}' \
  > "\$WORK_WSL/p4-stream.log" 2>&1

python3 - "\$WORK_WSL/p4-stream.log" "\$WORK_WSL/p4.log" <<'PYCHECK'
import json
import sys

stream_path, log_path = sys.argv[1], sys.argv[2]
raw = open(stream_path, encoding="utf-8", errors="replace").read()
verdicts = []
for line in raw.splitlines():
    line = line.strip()
    if not line.startswith("data:"):
        continue
    payload = line[len("data:"):].strip()
    if not payload or payload == "[DONE]":
        continue
    try:
        obj = json.loads(payload)
    except Exception:
        continue
    if isinstance(obj, dict) and obj.get("type") == "data-policy" and isinstance(obj.get("data"), dict):
        verdicts.append(json.dumps(obj["data"], ensure_ascii=False, separators=(",", ":")))
out = open(log_path, "a", encoding="utf-8")
if not verdicts:
    out.write("FAIL P4: no data-policy frame carrying a Verdict was found in the SSE stream\n")
    sys.exit(0)
wire = verdicts[-1]
obj = json.loads(wire)
legal = {"ok", "needs_input", "refused", "error"}
ok = True
if obj.get("status") not in legal:
    out.write("FAIL P4: verdict status %r is not one of %s\n" % (obj.get("status"), sorted(legal)))
    ok = False
if not isinstance(obj.get("citations"), list):
    out.write("FAIL P4: verdict citations is not an array (nil citations are rejected by verdict.Validate)\n")
    ok = False
conf = obj.get("confidence")
if not isinstance(conf, (int, float)) or conf < 0 or conf > 0.5:
    out.write("FAIL P4: stub verdict confidence %r is outside [0, 0.5] (D-10)\n" % (conf,))
    ok = False
if obj.get("review_required") is not True:
    out.write("FAIL P4: stub verdict review_required %r is not true (D-10)\n" % (obj.get("review_required"),))
    ok = False
if ok:
    out.write("P4-STUB-VERDICT-VALID: status=%s citations=%d confidence=%s review_required=true\n"
              % (obj.get("status"), len(obj.get("citations") or []), obj.get("confidence")))
# drill:P4-MODE-STUB:begin
if '"mode":"stub"' in wire:
    out.write('P4-MODE-STUB: the data-policy payload carries the literal "mode":"stub"\n')
else:
    out.write('FAIL P4: the data-policy payload does not carry "mode":"stub" (wire annotation missing)\n')
# drill:P4-MODE-STUB:end
out.close()
PYCHECK

# 6) P6：静态托管的真实进程断言
DIST_DIR="\$REPO_WSL/web/dist"
if [ -f "\$DIST_DIR/index.html" ]; then
  INDEX_BODY="\$(cat "\$DIST_DIR/index.html")"
  code_api="\$(curl -s -o "\$WORK_WSL/p6-api.body" -w '%{http_code}' "http://127.0.0.1:\$PORT/api/nonexistent")"
  # drill:P6-API-NOT-SHADOWED:begin
  if [ "\$code_api" = "404" ] && ! grep -qF 'id="root"' "\$WORK_WSL/p6-api.body" 2>/dev/null; then
    echo "P6-API-NOT-SHADOWED: GET /api/nonexistent returned 404 (not the SPA shell) — the static layer does not shadow /api" >> "\$WORK_WSL/p6.log"
  else
    echo "FAIL P6: GET /api/nonexistent returned \$code_api (want 404) or served the SPA shell" >> "\$WORK_WSL/p6.log"
  fi
  # drill:P6-API-NOT-SHADOWED:end
  code_spa="\$(curl -s -o "\$WORK_WSL/p6-spa.body" -w '%{http_code}' "http://127.0.0.1:\$PORT/skills/compliance")"
  if [ "\$code_spa" = "200" ] && cmp -s "\$WORK_WSL/p6-spa.body" "\$DIST_DIR/index.html"; then
    echo "P6-SPA-FALLBACK: GET /skills/compliance returned 200 with the exact web/dist/index.html bytes" >> "\$WORK_WSL/p6.log"
  else
    echo "FAIL P6: GET /skills/compliance returned \$code_spa and its body is not byte-identical to web/dist/index.html" >> "\$WORK_WSL/p6.log"
  fi
else
  echo "FAIL P6: web/dist/index.html is absent — the live SPA assertions cannot run (build the frontend first)" >> "\$WORK_WSL/p6.log"
fi
code_trav="\$(curl -s -o "\$WORK_WSL/p6-trav.body" -w '%{http_code}' --path-as-is "http://127.0.0.1:\$PORT/../.env.example")"
if [ "\$code_trav" != "200" ] || ! grep -qF 'RETRIEVER_BACKEND' "\$WORK_WSL/p6-trav.body" 2>/dev/null; then
  echo "P6-TRAVERSAL-REJECTED: GET /../.env.example returned \$code_trav and did not leak the repo file" >> "\$WORK_WSL/p6.log"
else
  echo "FAIL P6: GET /../.env.example leaked the repo file with 200" >> "\$WORK_WSL/p6.log"
fi

# 7) 收尾
kill \$SERVER_PID 2>/dev/null
wait \$SERVER_PID 2>/dev/null
kill \$EMBED_PID 2>/dev/null
wait \$EMBED_PID 2>/dev/null
exit 0
DRIVER
}

run_live_driver() {
  local only_for_driver="${GATE_ONLY:-}"
  write_embed_double
  write_live_driver
  local driver_wsl
  driver_wsl="$(wslpath_of "$WORK/live-driver.sh")"
  wsl.exe -e bash -lc "GATE_ONLY='$only_for_driver' bash '$driver_wsl'" >"$WORK/live-driver.out" 2>&1
  local rc=$?
  if [ "$rc" -ne 0 ]; then
    fail "live driver exited $rc — see $WORK/live-driver.out ($(head -c 400 "$WORK/live-driver.out" 2>/dev/null | tr '\n' ' '))"
  fi
  grep -q 'using test-only embedding double' "$WORK/p0.log" 2>/dev/null ||
    fail "the test-only embedding double did not announce itself — the P9 self-proof cannot grep 'using test-only embedding double'"
  if grep -q '^FAIL ' "$WORK/p4.log" 2>/dev/null || grep -q '^FAIL ' "$WORK/p6.log" 2>/dev/null; then
    cat "$WORK/p4.log" "$WORK/p6.log" 2>/dev/null
    fail "a live assertion failed (see $WORK/p4.log and $WORK/p6.log)"
  fi
}

LIVE_DONE=0
ensure_live() {
  if [ "$LIVE_DONE" -eq 0 ]; then
    run_live_driver
    LIVE_DONE=1
  fi
}

# =====================================================================
# P1 — 语料与缓存键
# =====================================================================
partition_P1() {
  local log="$WORK/p1.log"
  : >"$log"
  local count
  count="$(find "$ROOT/knowledge" -name '*.md' -type f | wc -l | tr -d ' \r')"
  if [ "$count" -ne "$CORPUS_EXPECTED" ]; then
    fail "P1 corpus count: knowledge/ holds $count indexable .md files, want exactly $CORPUS_EXPECTED (D-05's 220 is a directory-entry count that also omits personal; disk is authoritative)"
  fi
  echo "P1-CORPUS-218: knowledge/ holds exactly $count indexable .md files (authority + sql-guides; draft/historical/personal carry only .gitkeep)" >>"$log"

  run_go_test "$WORK/p1-tiers.log" ./internal/retriever/embedded/ -run 'TestValidateDocsFailClosed|TestTierAllowedEmptyIsDeny|TestTierAllowedExactMatchOnly' -v ||
    fail "P1 tier validation tests failed — see $WORK/p1-tiers.log"
  require_test_pass "$WORK/p1-tiers.log" TestValidateDocsFailClosed
  require_test_pass "$WORK/p1-tiers.log" TestTierAllowedEmptyIsDeny
  echo "P1-TIERS-VALID: tier validation is fail-closed (unknown/empty tier is denied, exact match only)" >>"$log"

  run_go_test "$WORK/p1-cache.log" ./internal/retriever/embedded/ -run 'TestCacheKeyIncludesModelDimAndCorpusHash|TestCorpusHashIgnoresMtime|TestCorpusHashOverRealKnowledgeTree|TestCacheCorruptFileTriggersRebuild' -v ||
    fail "P1 cache-key tests failed — see $WORK/p1-cache.log"
  require_test_pass "$WORK/p1-cache.log" TestCacheKeyIncludesModelDimAndCorpusHash
  require_test_pass "$WORK/p1-cache.log" TestCorpusHashIgnoresMtime
  echo "P1-CACHE-KEY-OK: the cache key binds model + dimension + corpus hash, is content-based (mtime-independent), and a corrupt cache forces a rebuild" >>"$log"

  ok_partition "P1" "$log" "P1-CORPUS-218" "P1-TIERS-VALID" "P1-CACHE-KEY-OK"
  # drill:done:P1
  done_partition "P1"
}

# =====================================================================
# P2 — embedded 检索语义
# =====================================================================
partition_P2() {
  local log="$WORK/p2.log"
  : >"$log"
  run_go_test "$WORK/p2-tests.log" ./internal/retriever/embedded/ -run 'TestSearchEmptyTiersReturnsZeroHitsNoError|TestSearchReturnsHitsForAllowedTier|TestSearchTopKAndQueryVecGuards|TestGetChunkReturnsErrChunkNotFound|TestRankOrdersByScoreDescThenIDAsc' -v ||
    fail "P2 search tests failed — see $WORK/p2-tests.log"

  # drill:P2-EMPTY-TIERS-ZERO:begin
  require_test_pass "$WORK/p2-tests.log" TestSearchEmptyTiersReturnsZeroHitsNoError
  echo "P2-EMPTY-TIERS-ZERO: Search with an empty tier list returns 0 hits and no error (refusal-by-construction, aligned with milvus 'false' and mongo {\$in: []})" >>"$log"
  # drill:P2-EMPTY-TIERS-ZERO:end

  require_test_pass "$WORK/p2-tests.log" TestSearchTopKAndQueryVecGuards
  echo "P2-TOPK-BOUNDED: hits never exceed topK and a nil/empty query vector is rejected" >>"$log"

  require_test_pass "$WORK/p2-tests.log" TestGetChunkReturnsErrChunkNotFound
  echo "P2-CHUNK-NOTFOUND: GetChunk on an unknown id returns ErrChunkNotFound" >>"$log"

  require_test_pass "$WORK/p2-tests.log" TestRankOrdersByScoreDescThenIDAsc
  echo "P2-RANK-DETERMINISTIC: rank sorts by (score desc, id asc) so equal scores stay reproducible" >>"$log"

  ok_partition "P2" "$log" "P2-EMPTY-TIERS-ZERO" "P2-TOPK-BOUNDED" "P2-CHUNK-NOTFOUND"
  # drill:done:P2
  done_partition "P2"
}

# =====================================================================
# P3 — tier 隔离先于打分
# =====================================================================
partition_P3() {
  local log="$WORK/p3.log"
  : >"$log"
  run_go_test "$WORK/p3-tests.log" ./internal/retriever/embedded/ -run 'TestTierAllowedEmptyIsDeny|TestCandidatesExcludesNonWhitelistedTier|TestCandidatesExcludesExpiredEffectiveTo|TestCandidatesEmptyTiersYieldsNil|TestSearchNeverScoresExcludedTiers|TestSearchCandidateSetEmptySkipsRank' -v ||
    fail "P3 tier-isolation tests failed — see $WORK/p3-tests.log"

  require_test_pass "$WORK/p3-tests.log" TestCandidatesExcludesNonWhitelistedTier
  require_test_pass "$WORK/p3-tests.log" TestCandidatesExcludesExpiredEffectiveTo
  echo "P3-NO-UNAUTHORIZED-TIER: the candidate set never contains a non-whitelisted tier or an expired effective_to window, so a non-authorized chunk cannot appear in results" >>"$log"

  # drill:P3-FILTER-BEFORE-SCORE:begin
  require_test_pass "$WORK/p3-tests.log" TestSearchNeverScoresExcludedTiers
  require_test_pass "$WORK/p3-tests.log" TestSearchCandidateSetEmptySkipsRank
  echo "P3-FILTER-BEFORE-SCORE: excluded tiers are dropped while building the candidate set (two-phase PRE-filter), never scored and then removed — an empty candidate set skips ranking entirely" >>"$log"
  # drill:P3-FILTER-BEFORE-SCORE:end

  ok_partition "P3" "$log" "P3-NO-UNAUTHORIZED-TIER" "P3-FILTER-BEFORE-SCORE"
  # drill:done:P3
  done_partition "P3"
}

# =====================================================================
# P4 — 无 key 启动 + 一轮问答 + stub 三重标注（wire 侧）
# =====================================================================
partition_P4() {
  local log="$WORK/p4.log"
  ensure_live
  grep -qF 'P4-NO-KEY-STARTED' "$log" ||
    fail "P4: the backend did not boot without an API key, or the stub WARN is missing — see $log and $WORK/p4-server.log"
  grep -qF 'P4-STUB-VERDICT-VALID' "$log" ||
    fail "P4: no legal Verdict was produced in the no-key path — see $log"
  grep -qF 'P4-MODE-STUB' "$log" ||
    fail 'P4: the data-policy payload lost the literal "mode":"stub" — the wire-level stub annotation is the third leg of D-08'
  ok_partition "P4" "$log" "P4-NO-KEY-STARTED" "P4-STUB-VERDICT-VALID" "P4-MODE-STUB"
  # drill:done:P4
  done_partition "P4"
}

# =====================================================================
# P5 — grounding：stub 不豁免校验
# =====================================================================
partition_P5() {
  local log="$WORK/p5.log"
  : >"$log"
  run_go_test "$WORK/p5-stub.log" ./internal/server/stubchat/ -run 'TestStub_SpanGroundedByRealChecker|TestStub_ForgedSpanIsDowngraded|TestStub_ConfidenceAndReviewFloor|TestStub_StreamIsNeverEmpty' -v ||
    fail "P5 stubchat grounding tests failed — see $WORK/p5-stub.log"

  require_test_pass "$WORK/p5-stub.log" TestStub_SpanGroundedByRealChecker
  echo "P5-GROUNDED-SPAN: a stub citation span that really occurs in the chunk is accepted by the same verdict.GroundingChecker the real model goes through (no exemption)" >>"$log"

  # drill:P5-FORGED-SPAN-DOWNGRADED:begin
  require_test_pass "$WORK/p5-stub.log" TestStub_ForgedSpanIsDowngraded
  echo "P5-FORGED-SPAN-DOWNGRADED: a forged span is downgraded to needs_input with review_required=true, proving the stub cannot smuggle unchecked citations" >>"$log"
  # drill:P5-FORGED-SPAN-DOWNGRADED:end

  require_test_pass "$WORK/p5-stub.log" TestStub_ConfidenceAndReviewFloor
  echo "P5-CONFIDENCE-CEILING: stub confidence stays <= 0.5 with review_required=true (D-10)" >>"$log"

  ok_partition "P5" "$log" "P5-GROUNDED-SPAN" "P5-FORGED-SPAN-DOWNGRADED" "P5-CONFIDENCE-CEILING"
  # drill:done:P5
  done_partition "P5"
}

# =====================================================================
# P6 — 同源静态托管（单测四分支 + 真实进程）
# =====================================================================
partition_P6() {
  local log="$WORK/p6.log"
  run_go_test "$WORK/p6-tests.log" ./internal/webui/ -run 'TestStaticHandlerServesIndexAtRoot|TestStaticHandlerSPAFallbackForExtensionlessPath|TestStaticHandlerMissingAssetReturns404|TestStaticHandlerMissingDistReturns503|TestStaticHandlerAPIPrefixNeverFallsBackToHTML|TestStaticHandlerRejectsTraversalAndSymlink|TestStaticHandlerNeverListsDirectories' -v ||
    fail "P6 webui unit tests failed — see $WORK/p6-tests.log"
  require_test_pass "$WORK/p6-tests.log" TestStaticHandlerSPAFallbackForExtensionlessPath
  require_test_pass "$WORK/p6-tests.log" TestStaticHandlerMissingAssetReturns404
  require_test_pass "$WORK/p6-tests.log" TestStaticHandlerMissingDistReturns503
  require_test_pass "$WORK/p6-tests.log" TestStaticHandlerRejectsTraversalAndSymlink
  echo "P6-UNIT-BRANCHES: the four hosting branches are covered by unit tests (hit / extensionless-miss => index.html / asset-miss => 404 / dist-missing => 503), traversal and symlink escapes are rejected, and directories are never listed" >>"$log"

  ensure_live
  grep -qF 'P6-API-NOT-SHADOWED' "$log" ||
    fail "P6: /api/nonexistent was shadowed by the static layer (want 404 without the SPA shell) — see $log"

  grep -qF 'P6-SPA-FALLBACK' "$log" ||
    fail "P6: an extensionless miss did not return the exact web/dist/index.html bytes — see $log"
  grep -qF 'P6-TRAVERSAL-REJECTED' "$log" ||
    fail "P6: a traversal attempt was not rejected — see $log"

  ok_partition "P6" "$log" "P6-UNIT-BRANCHES" "P6-API-NOT-SHADOWED" "P6-SPA-FALLBACK" "P6-TRAVERSAL-REJECTED"
  # drill:done:P6
  done_partition "P6"
}

# =====================================================================
# P7 — demo 素材（只断言产物与脚本，不探测渲染器）
# =====================================================================
partition_P7() {
  local log="$WORK/p7.log"
  : >"$log"
  local gif="$ROOT/docs/demo.gif"
  [ -f "$gif" ] || fail "P7: docs/demo.gif is absent — README.md:18 already references it, so a missing file is a broken image for every visitor (FACE-01)"
  [ -s "$gif" ] || fail "P7: docs/demo.gif exists but is empty — an empty artifact is not evidence"
  local bytes
  bytes="$(wc -c <"$gif" | tr -d ' \r')"

  # drill:P7-GIF-MAGIC:begin
  if [ "$(head -c 6 "$gif")" = "GIF87a" ] || [ "$(head -c 6 "$gif")" = "GIF89a" ]; then
    echo "P7-GIF-MAGIC: docs/demo.gif starts with a GIF magic header (GIF87a/GIF89a), $bytes bytes" >>"$log"
  else
    fail "P7: docs/demo.gif does not start with GIF87a/GIF89a — it is not a GIF"
  fi
  # drill:P7-GIF-MAGIC:end

  grep -a -E -q 'sk-[A-Za-z0-9]{16,}|GLM_API_KEY=|LOCAL_API_KEY=' "$gif"
  local gif_grep_rc=$?
  if [ "$gif_grep_rc" -eq 1 ]; then
    echo "P7-NO-KEY-BYTES: the GIF binary contains no API-key-shaped byte string (sk-…, GLM_API_KEY=, LOCAL_API_KEY=)" >>"$log"
  elif [ "$gif_grep_rc" -eq 2 ]; then
    fail "P7: could not read docs/demo.gif for the key-byte scan (grep exit 2) — 'unreadable' must not be reported as 'clean'"
  else
    fail "P7: docs/demo.gif contains an API-key-shaped byte string — a secret was recorded into the repository"
  fi

  local rec="$ROOT/scripts/record-demo.sh"
  [ -f "$rec" ] || fail "P7: scripts/record-demo.sh is absent — the demo asset must be reproducible, and the script doubles as the SC4 quickstart record"
  head -1 "$rec" | grep -q '^#!' || fail "P7: scripts/record-demo.sh has no shebang on its first line"
  bash -n "$rec" || fail "P7: scripts/record-demo.sh does not parse"
  echo "P7-RECORD-SCRIPT: scripts/record-demo.sh exists, is a shebang-and-parses shell script, and can be re-run via 'make demo-gif'" >>"$log"

  ok_partition "P7" "$log" "P7-GIF-MAGIC" "P7-NO-KEY-BYTES" "P7-RECORD-SCRIPT"
  # drill:done:P7
  done_partition "P7"
}

# =====================================================================
# P8 — 既有配置与生产 embedder 的不动性
# =====================================================================
partition_P8() {
  local log="$WORK/p8.log"
  : >"$log"
  run_go_test "$WORK/p8-tests.log" ./internal/retriever/embedded/ ./cmd/server/ ./internal/webui/ ||
    fail "P8 compilation/regression tests failed — see $WORK/p8-tests.log"
  echo "P8-BACKENDS-INTACT: embedded / cmd-server / webui packages compile and their tests pass, so the existing milvus / mongo / none branches were not disturbed" >>"$log"

  ( cd "$ROOT" && git diff --quiet -- .env.example docker-compose.yml Dockerfile.backend Dockerfile.frontend .github/workflows ) ||
    fail "P8: .env.example / docker-compose.yml / Dockerfile* / .github/workflows was modified — D-15 forbids touching the existing deployment surface"
  echo "P8-CONFIG-UNTOUCHED: .env.example, docker-compose.yml, both Dockerfiles and .github/workflows are byte-identical to HEAD" >>"$log"

  # drill:P8-PROD-EMBEDDER-UNTOUCHED:begin
  ( cd "$ROOT" && git diff --quiet -- internal/embedder/bge ) ||
    fail "P8: internal/embedder/bge was modified — the test-only embedding double must never leak into the production embedder (T-49-31)"
  echo "P8-PROD-EMBEDDER-UNTOUCHED: internal/embedder/bge is byte-identical to HEAD, so the embedding double exists only inside this gate" >>"$log"
  # drill:P8-PROD-EMBEDDER-UNTOUCHED:end

  local deps
  deps="$(cd "$ROOT" && git diff --name-only -- go.mod go.sum web/package.json web/package-lock.json 2>/dev/null)"
  [ -z "$deps" ] || fail "P8: the dependency manifests changed (T-49-SC forbids new Go/npm dependencies): $deps"
  echo "P8-DEPS-UNCHANGED: go.mod / go.sum / web/package.json / web/package-lock.json are unchanged (zero new dependencies)" >>"$log"

  ok_partition "P8" "$log" "P8-BACKENDS-INTACT" "P8-CONFIG-UNTOUCHED" "P8-PROD-EMBEDDER-UNTOUCHED"
  # drill:done:P8
  done_partition "P8"
}

# =====================================================================
# P9 — 自证：注册表一致性 + 空分区必红 + 九条变异演练 + 工作树干净
# =====================================================================
partition_P9() {
  local log="$WORK/p9.log"
  : >"$log"
  local self="$ROOT/verify-phase-49.sh"
  [ -f "$self" ] || fail "P9: cannot find $self to self-prove"

  # (1) 注册表一致性
  local begin_calls ok_calls done_calls
  begin_calls="$(grep -cE '^[[:space:]]*begin_partition [0-9]' "$self")"
  ok_calls="$(grep -cE '^[[:space:]]*ok_partition "' "$self")"
  done_calls="$(grep -cE '^[[:space:]]*done_partition "' "$self")"
  [ "$begin_calls" -eq "$EXPECTED_PARTITIONS" ] ||
    fail "P9: $begin_calls begin_partition call sites but EXPECTED_PARTITIONS=$EXPECTED_PARTITIONS — a partition that never announces itself cannot be audited"
  [ "$ok_calls" -eq "$EXPECTED_PARTITIONS" ] ||
    fail "P9: $ok_calls ok_partition call sites but EXPECTED_PARTITIONS=$EXPECTED_PARTITIONS"
  [ "$done_calls" -eq "$EXPECTED_PARTITIONS" ] ||
    fail "P9: $done_calls done_partition call sites but EXPECTED_PARTITIONS=$EXPECTED_PARTITIONS — registry and call sites disagree"
  echo "P9-SELF-PROOF: begin/ok/done call sites all equal EXPECTED_PARTITIONS=$EXPECTED_PARTITIONS, and each partition has an evidence-bound token group" >>"$log"

  # (2) 空分区必红：删掉 P2 的一条断言+证据块，副本必须以非零退出
  local d
  d="$WORK/drill-empty"
  mkdir -p "$d"
  cp "$self" "$d/verify-phase-49.sh"
  local empty_mark="P2""-EMPTY-TIERS-ZERO"
  sed -i "/# drill:${empty_mark}:begin/,/# drill:${empty_mark}:end/d" "$d/verify-phase-49.sh"
  GATE_ROOT="$ROOT" bash "$d/verify-phase-49.sh" --only P2 >"$WORK/drill-empty.log" 2>&1
  local empty_rc=$?
  if [ "$empty_rc" -eq 0 ]; then
    cat "$WORK/drill-empty.log"
    fail "P9: removing P2's empty-tiers assertion left the gate green — the assertion group is not load-bearing (P9-EMPTY-PARTITION-RED)"
  fi
  grep -qE 'P2|P2-EMPTY-TIERS-ZERO' "$WORK/drill-empty.log" ||
    fail "P9: the empty-partition drill went red without naming P2 or its missing token — the failure is unattributable"
  echo "P9-EMPTY-PARTITION-RED: deleting P2's empty-tiers assertion makes a copy exit $empty_rc and name the P2 token" >>"$log"

  # (3) 变异演练 D0..D9
  local drills_run=0 drills_red=0

  # D0：未变异基线，必须 exit 0 且只打印一条 ALL PASSED
  if [ "${GATE_DRILL:-0}" != "1" ]; then
    d="$WORK/drill-D0"
    mkdir -p "$d"
    cp "$self" "$d/verify-phase-49.sh"
    GATE_ROOT="$ROOT" GATE_DRILL=1 bash "$d/verify-phase-49.sh" >"$WORK/drill-D0.log" 2>&1
    local d0_rc=$?
    [ "$d0_rc" -eq 0 ] || {
      cat "$WORK/drill-D0.log"
      fail "P9: the unmutated copy exited $d0_rc — if the baseline is not green, every other drill below is meaningless (P9-DRILL-D0-GREEN)"
    }
    local d0_banner
    # banner 字面量在运行时拼装：脚本里必须只有一个 print site，机械检查
    # `grep -cF` 才能得到 1（Phase 48 的 SOFT_KEY 惯例）。
    local banner_lit="=== Phase 49 Gate: ""ALL PASSED ==="
    d0_banner="$(grep -cF "$banner_lit" "$WORK/drill-D0.log")"
    [ "$d0_banner" -eq 1 ] || fail "P9: the D0 baseline printed the ALL PASSED banner $d0_banner times, want exactly 1"
    echo "P9-DRILL-D0-GREEN: the unmutated copy exits 0 and prints exactly one ALL PASSED banner" >>"$log"
    drills_run=$((drills_run + 1))
  fi

  # 逐条变异：删掉某条断言+证据块（或改坏一个值），副本必须以非零退出并点名该分区
  run_drill() {
    local id="$1" only="$2" expect="$3" sedexpr="$4"
    local dd="$WORK/drill-$id"
    mkdir -p "$dd"
    cp "$self" "$dd/verify-phase-49.sh"
    sed -i "$sedexpr" "$dd/verify-phase-49.sh"
    if cmp -s "$self" "$dd/verify-phase-49.sh"; then
      fail "P9: drill $id did not change the copy — the anchor is stale, so the drill would be a no-op that fakes a red"
    fi
    # only=ALL 表示整跑：某些变异只有 P9 的计数断言能抓到（例如删掉一次
    # done_partition 调用），局部跑永远看不到 P9。
    local only_args=()
    if [ "$only" != "ALL" ]; then
      only_args=(--only "$only")
    fi
    GATE_ROOT="$ROOT" GATE_DRILL=1 bash "$dd/verify-phase-49.sh" "${only_args[@]}" >"$WORK/drill-$id.log" 2>&1
    local rc=$?
    if [ "$rc" -eq 0 ]; then
      cat "$WORK/drill-$id.log"
      fail "P9: drill $id stayed green (exit 0) after mutating '$sedexpr' — the gate does not bite (P9-DRILLS-RED)"
    fi
    if ! grep -qE "$expect" "$WORK/drill-$id.log"; then
      cat "$WORK/drill-$id.log"
      fail "P9: drill $id went red but never named '$expect' — an unattributable red is not evidence"
    fi
    drills_run=$((drills_run + 1))
    drills_red=$((drills_red + 1))
  }

  # 演练矩阵整体受 GATE_DRILL 保护：副本以 GATE_DRILL=1 运行，不许再展开自己的
  # 演练（否则 D0 全跑副本 → 副本又跑 D0 → 指数级递归）。
  if [ "${GATE_DRILL:-0}" != "1" ]; then
    # 删掉某条「断言 + 证据」块。marker 由变量拼进 sed 表达式，命令里因此
  # 不含 marker 字面量 —— 否则 sed 的范围会匹配到自己这一行，把后面整片
  # 代码一起删掉，副本变成语法错误而不是"断言失效变红"。
  run_drill_block() {
    local id="$1" only="$2" expect="$3" mark="$4"
    run_drill "$id" "$only" "$expect" "/# drill:${mark}:begin/,/# drill:${mark}:end/d"
  }

  run_drill "D1" "P1" "P1|220|CORPUS" 's/^CORPUS_EXPECTED=218$/CORPUS_EXPECTED=220/'

    run_drill_block "D2" "P2" "P2-EMPTY-TIERS-ZERO|P2" "P2-EMPTY-TIERS-ZERO"

    run_drill_block "D3" "P3" "P3-FILTER-BEFORE-SCORE|P3" "P3-FILTER-BEFORE-SCORE"

    run_drill_block "D4" "P4" "P4-MODE-STUB|P4" "P4-MODE-STUB"

    run_drill_block "D5" "P5" "P5-FORGED-SPAN-DOWNGRADED|P5" "P5-FORGED-SPAN-DOWNGRADED"

    run_drill_block "D6" "P6" "P6-API-NOT-SHADOWED|P6" "P6-API-NOT-SHADOWED"

    run_drill_block "D7" "P7" "P7-GIF-MAGIC|P7" "P7-GIF-MAGIC"

    run_drill_block "D8" "P8" "P8-PROD-EMBEDDER-UNTOUCHED|P8" "P8-PROD-EMBEDDER-UNTOUCHED"

    # D9 删掉真实的一次 done_partition 调用（不是它的注释），并且**整跑**：

    # 这种缺失只有 P9 的「9 个分区各有 begin/ok/done」计数断言能抓到，局部跑发现不了。

    run_drill "D9" "ALL" "P9|INCOMPLETE|done_partition|registry" '/^  done_partition "P8"$/d'

  fi

  echo "P9-DRILLS-RED: $drills_red of $drills_run drills went red and named their partition; D0 stayed green with exactly one ALL PASSED banner" >>"$log"

  # (4) 门禁不在工作树**新增**产物：比对运行前后的 git status 差集。
  # 只看差集，所以开发机上既存的无关脏文件不会把健康树打成假红；而门禁自己
  # 一旦把缓存/数据库/cast 落到仓库里，差集里就必然出现下面这些名字。
  local after leaked
  after="$( mktemp )"
  ( cd "$ROOT" && git status --porcelain 2>/dev/null | sort ) >"$after" 2>/dev/null || : >"$after"
  leaked="$(comm -13 "$GATE_TREE_BEFORE" "$after" | grep -E 'skillforge\.db|p4\.db|\.cast|\.skillforge-cache' || true)"
  rm -f "$after"
  [ -z "$leaked" ] || fail "P9: the gate left artifacts in the work tree: $leaked"
  echo "P9-TREE-CLEAN: the run added no cache/SQLite/cast artifact to the work tree (everything lives under \$WORK)" >>"$log"

  # (5) 替身端点的宣告字面量必须真实出现在门禁输出里（P9 自证分区 grep 它）
  grep -qF 'using test-only embedding double' "$self" ||
    fail "P9: the literal 'using test-only embedding double' is missing from the gate — the embedding double must announce itself to auditors"
  echo "P9-DOUBLE-ANNOUNCED: the gate carries and prints the literal 'using test-only embedding double'" >>"$log"

  ok_partition "P9" "$log" "P9-SELF-PROOF" "P9-EMPTY-PARTITION-RED" "P9-DRILLS-RED" "P9-TREE-CLEAN" "P9-DOUBLE-ANNOUNCED"
  # drill:done:P9
  done_partition "P9"
}

# =====================================================================
# 主流程
# =====================================================================
echo "=== Phase 49 Gate (Hermetic Demo) ==="
echo "root: $ROOT"
echo "work: $WORK"
echo "partitions: $EXPECTED_PARTITIONS"
if [ -n "$ONLY" ]; then
  echo "mode: PARTIAL RUN --only $ONLY (debugging aid, never a full pass)"
fi

GATE_ONLY="$ONLY"
export GATE_ONLY

if partition_selected 1; then
  begin_partition 1 "语料与缓存键（218 篇可索引 .md / tier fail-closed / 缓存键绑定 model+dim+corpus hash）"
  partition_P1
fi
if partition_selected 2; then
  begin_partition 2 "embedded 检索语义（空 tiers => 0 命中 / topK 上界 / unknown chunk id）"
  partition_P2
fi
if partition_selected 3; then
  begin_partition 3 "tier 隔离先于打分（两段式 PRE-filter，授权边界不因 hermetic 模式削弱）"
  partition_P3
fi
if partition_selected 4; then
  begin_partition 4 "无 key 启动 + 一轮问答 + wire 侧 \"mode\":\"stub\""
  partition_P4
fi
if partition_selected 5; then
  begin_partition 5 "grounding（stub 的 span 过同一校验；伪造 span 必降级）"
  partition_P5
fi
if partition_selected 6; then
  begin_partition 6 "同源静态托管（/api 不被遮蔽 / SPA fallback / 穿越拒绝）"
  partition_P6
fi
if partition_selected 7; then
  begin_partition 7 "demo 素材（GIF magic / 无 key 字节 / 录制脚本）"
  partition_P7
fi
if partition_selected 8; then
  begin_partition 8 "既有配置与生产 embedder 的不动性 + 零新依赖"
  partition_P8
fi
if partition_selected 9; then
  begin_partition 9 "自证（注册表 / 空分区必红 / 九条变异演练 / 工作树干净）"
  partition_P9
fi

echo
if [ -n "$ONLY" ]; then
  selected=0
  missing=""
  for pn in $(printf '%s' "$ONLY" | tr ',' ' '); do
    num="${pn#P}"
    selected=$((selected + 1))
    [ -f "$WORK/ran/$num.ok" ] || missing="$missing $pn"
  done
  if [ -n "$missing" ]; then
    echo "=== Phase 49 Gate: PARTIAL RUN FAILED —$missing did not complete ==="
    exit 1
  fi
  echo "=== Phase 49 Gate: PARTIAL RUN ($ONLY) === — $selected/$selected selected partitions passed; this is NOT a full pass"
  exit 0
fi

if [ "$RAN" -ne "$EXPECTED_PARTITIONS" ]; then
  echo "=== Phase 49 Gate: INCOMPLETE — RAN=$RAN of $EXPECTED_PARTITIONS ==="
  exit 1
fi

echo "=== Phase 49 Gate: ALL PASSED ==="
echo "P1 语料 218 篇可索引 .md；tier 校验 fail-closed；缓存键绑定 model+dim+corpus hash"
echo "P2 空 tiers => 0 命中；topK 上界；unknown chunk id 返回 ErrChunkNotFound"
echo "P3 候选集先过滤后打分，非授权 tier 绝不出现在结果里"
echo "P4 无 key 启动 + stub WARN + 合法 Verdict + wire 上的 \"mode\":\"stub\""
echo "P5 stub 的 span 过同一 grounding；伪造 span 必降级为 needs_input"
echo "P6 /api 不被静态层遮蔽；SPA fallback 逐字节；穿越拒绝"
echo "P7 docs/demo.gif 是真实 GIF 且不含 key 字节；record-demo.sh 可复现"
echo "P8 .env.example/docker-compose/Dockerfile/CI 与生产 embedder 零改动；零新依赖"
echo "P9 注册表自证 + 空分区必红 + 九条变异演练 + 工作树干净"
exit 0
