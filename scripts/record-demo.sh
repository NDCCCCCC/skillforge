#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# scripts/record-demo.sh — 录制 Phase 49 的 demo 素材（docs/demo.gif）
#
# 录制的是**终端 CLI 演示**（D-17），不是浏览器 UI：
#   ① 起后端（无 API key ⇒ stub LLM，启动时打 WARN）
#   ② 打一轮问答，把 SSE 流里的 data-policy Verdict 与 citations 打成人类可读的几行
#   ③ 列出当前 Skill
#   ④ 做一次 Skill 切换（切完后**立即还原**，工作树不留任何改动）
#   ⑤ 停后端
#
# 硬约束：
#   * `unset GLM_API_KEY LOCAL_API_KEY` —— 空 key 反而会展示 stub 的启动 WARN，
#     那正是本 demo 要讲的事；脚本也从 .env 里**只**取 BGE_M3_URL 一行，
#     任何 *_API_KEY 的值都不会进录制环境（Pitfall 12）。
#   * 本脚本**不使用任何替身 embedding 端点**：素材要展示真实检索链路，
#     所以 BGE_M3_URL 必须指向一个真实可用的外部 embedding 端点；
#     不可达时明确失败并说明缺什么，绝不写占位 GIF 充数。
#   * 三条渲染路径（agg / svg-term-cli / python+Pillow+ffmpeg），全不可用时
#     以非零退出并打印 "no renderer available"（绝不产出占位帧）。
#   * 中间产物全部落在 mktemp -d 并在 trap 里清理；仓库里只留 docs/demo.gif。
#
# 用法：
#   bash scripts/record-demo.sh                 # 真实录制
#   DEMO_DRY_RUN=1 bash scripts/record-demo.sh  # 只打印将要执行的命令，不写文件
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# ---------- 安全约束：录制环境里不得存在任何 API key ----------
unset GLM_API_KEY || true
unset LOCAL_API_KEY || true
unset ANTHROPIC_API_KEY || true
unset OPENAI_API_KEY || true
# .env 只允许贡献 BGE_M3_URL 一行；任何 *_API_KEY 行一律不读。
if [ -f .env ]; then
  BGE_FROM_ENV="$(grep -E '^[[:space:]]*BGE_M3_URL=' .env | tail -1 | cut -d= -f2- | tr -d '"'"'"' ' || true)"
fi

# ---------- 环境 ----------
export RETRIEVER_BACKEND=embedded
export BGE_M3_URL="${BGE_M3_URL:-${BGE_FROM_ENV:-http://localhost:11434/v1/embeddings}}"
export SKILLFORGE_MCP_SERVICE_SKILL="${SKILLFORGE_MCP_SERVICE_SKILL:-simple-qa}"
# 注意：静态托管目录是常量 webui.DistDirDefault == "web/dist"
# （internal/webui/static_test.go 明确锁住"dist 路径不得由环境变量驱动"，D-15），
# 所以这里没有 WEB_DIST_DIR 可设。
# 演示要展示"无 key 也能跑"，因此默认把向量缓存放到临时目录，录完即弃。
DEMO_WORK="$(mktemp -d)"
# 首次索引要把 218 篇语料全部喂给真实 embedding 模型，慢的是这一步而不是服务本身。
# 允许把缓存指到一个可复用的目录（DEMO_CACHE_DIR），这样第二次录制不必再算一遍；
# 未设置时仍然落在 $DEMO_WORK/cache，录完即弃，保持 hermetic。
DEMO_CACHE_DIR="${DEMO_CACHE_DIR:-$DEMO_WORK/cache}"
mkdir -p "$DEMO_CACHE_DIR"
# 就绪等待上限：0.3s × 次数。首次冷索引 218 篇可能要几分钟，给足 20 分钟。
DEMO_READY_TRIES="${DEMO_READY_TRIES:-4000}"
# 演示**不写任何 Skill 文件**：cmd/server/main.go:459 与 :820 把 skills 目录
# 写死成 "skills/"（SKILLFORGE_SKILLS_DIR 只喂 ontology emitter，不驱动 loader），
# 所以"换 Skill 换业务领域"只能靠请求里的 skill 字段切。录制因此对工作树只读。
cleanup() {
  if [ -n "${SERVER_PID:-}" ]; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  if [ "${DEMO_KEEP_WORK:-0}" = "1" ]; then
    say "[record-demo] DEMO_KEEP_WORK=1: keeping $DEMO_WORK"
  else
    rm -rf "$DEMO_WORK"
  fi
}
trap cleanup EXIT

DRY="${DEMO_DRY_RUN:-0}"
say() { printf '%s\n' "$*"; }
run() {
  if [ "$DRY" = "1" ]; then
    say "+ $*"
    return 0
  fi
  "$@"
}

# 解释器探测：Linux/WSL 只有 python3，Windows Git Bash 只有 python。
# 录制环境必须能跑 python —— 端口探测、SSE 解析、cast 渲染都依赖它。
PY=""
for cand in python3 python; do
  if command -v "$cand" >/dev/null 2>&1; then
    PY="$cand"
    break
  fi
done
if [ "$DRY" != "1" ] && [ -z "$PY" ]; then
  say "[record-demo] no python interpreter found (need python3 or python on PATH)"
  exit 1
fi
# 导出给演示脚本与渲染器子进程（它们是独立的 bash 进程）。
export PY

# ---------- 1) 真实 embedding 端点必须可达 ----------
if [ "$DRY" != "1" ]; then
  say "== checking the real embedding endpoint: $BGE_M3_URL"
  if ! curl -sS --max-time 10 -o /dev/null -w '' "$BGE_M3_URL" 2>/dev/null &&
     ! curl -sS --max-time 20 -X POST "$BGE_M3_URL" \
       -H 'Content-Type: application/json' \
       -d '{"input":["warmup"]}' -o /dev/null 2>/dev/null; then
    say "[record-demo] no real embedding endpoint at $BGE_M3_URL"
    say "[record-demo] need an external embedding endpoint (ollama pull bge-m3 then BGE_M3_URL=http://localhost:11434/v1/embeddings, or any OpenAI-compatible /v1/embeddings endpoint); this script never uses a stand-in"
    exit 1
  fi
fi

# ---------- 2) 端口与后端 ----------
PORT="$("$PY" - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)"
BASE="http://127.0.0.1:$PORT"

say "== booting the backend on $BASE (RETRIEVER_BACKEND=embedded, no API key)"
if [ "$DRY" != "1" ]; then
  # 预编译再启动：`go run` 会把真正的服务进程藏在编译器子进程后面，
  # 录制结束后无法可靠地停掉它（stop 会只杀掉 go run，孤儿进程占着端口）。
  say "== building the backend binary"
  ( cd "$REPO_ROOT" && go build -o "$DEMO_WORK/skillforge-server" ./cmd/server ) ||
    { say "[record-demo] go build ./cmd/server failed"; exit 1; }

  # 演示用的 skills 目录：只放 simple-qa 一个 Skill。

  env -u GLM_API_KEY -u LOCAL_API_KEY \
    SERVER_ADDR="127.0.0.1:$PORT" \
    RETRIEVER_BACKEND=embedded \
    SKILLFORGE_MCP_SERVICE_SKILL="$SKILLFORGE_MCP_SERVICE_SKILL" \
    BGE_M3_URL="$BGE_M3_URL" \
    SKILLFORGE_VECTOR_CACHE_DIR="$DEMO_CACHE_DIR" \
    SQLITE_DSN="$DEMO_WORK/demo.db" \
    "$DEMO_WORK/skillforge-server" >"$DEMO_WORK/server.log" 2>&1 &
  SERVER_PID=$!

  READY=0
  for _ in $(seq 1 "$DEMO_READY_TRIES"); do
    if ! kill -0 "$SERVER_PID" 2>/dev/null; then
      break
    fi
    code="$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/chat" \
      -H 'Content-Type: application/json' -d '{"query":"ping","skill":"simple-qa"}' 2>/dev/null || true)"
    if [ "$code" != "000" ] && [ -n "$code" ]; then
      READY=1
      break
    fi
    sleep 0.3
  done
  if [ "$READY" -ne 1 ]; then
    say "[record-demo] the backend did not become ready — see $DEMO_WORK/server.log"
    tail -20 "$DEMO_WORK/server.log" || true
    exit 1
  fi
  grep -m1 'STUB LLM MODE' "$DEMO_WORK/server.log" | sed 's/^/== /' || true
fi

# ---------- 3) 演示命令序列 ----------
# 演示**不写任何 Skill 文件**：cmd/server/main.go:459 / :820 把 skills 目录写死成
# "skills/"（SKILLFORGE_SKILLS_DIR 只喂 ontology emitter，不驱动 loader），所以
# 想演示"换 Skill 就换业务领域"只能靠请求里的 skill 字段切。好处是录制过程对
# 工作树完全只读，也不存在"录完忘了还原"的风险。
DEMO_SCRIPT="$DEMO_WORK/demo.sh"
cat >"$DEMO_SCRIPT" <<'DEMO'
set -u
BASE="$1"
PY="${PY:-python3}"

# 一次问答的渲染器：从 SSE 里挑 data-policy 帧，把 Verdict 打成人类可读。
SHOW_VERDICT='
import json, sys
for line in sys.stdin:
    line = line.strip()
    if not line.startswith("data:"):
        continue
    body = line[5:].strip()
    if not body or body == "[DONE]":
        continue
    try:
        obj = json.loads(body)
    except Exception:
        continue
    if obj.get("type") != "data-policy":
        continue
    v = obj.get("data") or {}
    print("")
    print("  --- Verdict (data-policy frame) ---")
    print("  status          : %s" % v.get("status"))
    print("  mode            : %s   <- stub 模式如实标注" % v.get("mode", "(omitted)"))
    print("  confidence      : %s" % v.get("confidence"))
    print("  review_required : %s" % v.get("review_required"))
    cites = v.get("citations") or []
    print("  citations       : %d" % len(cites))
    for c in cites:
        print("     - [%s] tier=%s" % (c.get("chunk_id"), c.get("tier", "?")))
    ans = (v.get("answer") or "").strip()
    if ans:
        print("  answer          : %s" % ans[:200])
'

ask() {
  # $1 = skill，$2 = 问题
  printf '\$ curl -sS -N -X POST %s/api/chat -d %s\n' "$BASE" "$(printf '{"query":"%s","skill":"%s"}' "$2" "$1")"
  curl -sS -N -X POST "$BASE/api/chat" \
    -H 'Content-Type: application/json' \
    -d "$(printf '{"query":"%s","skill":"%s"}' "$2" "$1")" \
    2>/dev/null | "$PY" -c "$SHOW_VERDICT"
  sleep 1.2
}

echo "# 1) 没有 API key、没有 Docker、没有外部向量库，直接跑起来的问答"
ask simple-qa "出差住宿费超标了怎么处理？"

echo ""
echo "\$ curl -sS $BASE/api/skills   # 全部可插拔的 Skill（reads 决定各自能吃哪一层语料）"
curl -sS "$BASE/api/skills" 2>/dev/null | "$PY" -c '
import json, sys
raw = sys.stdin.read()
try:
    data = json.loads(raw)
except Exception:
    print(raw[:400]); raise SystemExit(0)
items = data.get("skills") if isinstance(data, dict) else data
if not isinstance(items, list):
    print(str(data)[:400]); raise SystemExit(0)
for s in items:
    if not isinstance(s, dict):
        print("  - %s" % s)
        continue
    m = s.get("manifest") or {}
    reads = m.get("reads") or []
    print("  - %-24s reads=%s" % (s.get("name") or s.get("id") or "?",
                                 ",".join(reads) or "-"))
'
sleep 1.5

echo ""
echo "# 2) 同一个二进制、同一条命令，只把 skill 换成 sql-guides —— 知识域整个换掉"
ask sql-guides "怎么给保单查询写一条分页 SQL？"

echo ""
echo "# 换的是 Skill 文件里的 reads，不是代码：一个内核适配任意业务领域。"
sleep 1.0
DEMO

if [ "$DRY" = "1" ]; then
  say "+ bash $DEMO_SCRIPT $BASE"
  say "+ asciinema rec --overwrite -c <demo> (cast -> \$DEMO_WORK/demo.cast)"
fi

# ---------- 4) 录制 ----------
CAST="$DEMO_WORK/demo.cast"
say ""
say "== recording the terminal session to a cast"
if [ "$DRY" = "1" ]; then
  say "+ (asciinema rec)"
else
  if ! command -v asciinema >/dev/null 2>&1; then
    say "[record-demo] asciinema is not installed (pip install asciinema)"
    exit 1
  fi
  # 演示命令序列本身要进画面，所以用 bash -c 包一层并回显。
  # 注意：asciinema 2.x 没有 --cols/--rows（那是 agg 的选项），终端尺寸通过
  # COLUMNS/LINES 传给被录制的命令。
  asciinema rec --overwrite \
    -c "COLUMNS=100 LINES=30 bash '$DEMO_SCRIPT' '$BASE'" \
    "$CAST" >/dev/null 2>&1
  [ -s "$CAST" ] || { say "[record-demo] the asciinema cast is empty"; exit 1; }
fi

# ---------- 5) 渲染 ----------
GIF="$REPO_ROOT/docs/demo.gif"
say "== rendering the cast to docs/demo.gif"
if [ "$DRY" = "1" ]; then
  say "+ (renderer path A: agg | path B: svg-term-cli + rsvg/cairosvg | path C: python + Pillow + ffmpeg)"
else
  mkdir -p "$REPO_ROOT/docs"
  RENDERED=0

  # 路径 A（D-18 原形）：agg
  if [ "$RENDERED" -eq 0 ] && command -v agg >/dev/null 2>&1; then
    if agg "$CAST" "$GIF" >/dev/null 2>&1 && [ -s "$GIF" ]; then
      RENDERED=1
      say "  renderer: agg"
    fi
  fi

  # 路径 B：asciinema → svg-term-cli → (rsvg-convert | cairosvg) → ffmpeg
  if [ "$RENDERED" -eq 0 ]; then
    SVG="$DEMO_WORK/demo.svg"
    if npx --yes svg-term-cli --in "$CAST" --out "$SVG" >/dev/null 2>&1 && [ -s "$SVG" ]; then
      PNG="$DEMO_WORK/frame.png"
      if command -v rsvg-convert >/dev/null 2>&1; then
        rsvg-convert -o "$PNG" "$SVG" 2>/dev/null || true
      elif "$PY" -c "import cairosvg" >/dev/null 2&1; then
        "$PY" -m cairosvg "$SVG" -o "$PNG" >/dev/null 2>&1 || true
      fi
      if [ -s "$PNG" ] && command -v ffmpeg >/dev/null 2>&1; then
        ffmpeg -y -loop 0 -i "$PNG" -t 6 -vf "fps=8,scale=800:-1:flags=lanczos,split[a][b];[a]palettegen[p];[b][p]paletteuse" "$GIF" >/dev/null 2>&1 || true
        if [ -s "$GIF" ]; then
          RENDERED=1
          say "  renderer: svg-term-cli + ffmpeg"
        fi
      fi
    fi
  fi

  # 路径 C：直接解析 cast（真实输出 + 真实时间轴），Pillow 渲染 PNG 序列，ffmpeg 合成。
  # 渲染器内嵌在脚本里：它必须随仓库一起走，不能依赖 .planning/tmp 下的临时文件。
  if [ "$RENDERED" -eq 0 ]; then
    if "$PY" -c "import PIL" >/dev/null 2>&1 && command -v ffmpeg >/dev/null 2>&1; then
      RENDERER="$DEMO_WORK/cast-to-gif.py"
      cat >"$RENDERER" <<'CAST2GIF_PY'
#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Render an asciinema v2 cast to an animated GIF.

Pure stdlib + Pillow + ffmpeg. It replays the cast on a tiny terminal
emulator, renders one PNG per frame, and lets ffmpeg turn the PNG
sequence into a GIF. Embedded in scripts/record-demo.sh on purpose: the
recording must be reproducible from the repository alone.
"""
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unicodedata

from PIL import Image, ImageDraw, ImageFont

BG = (13, 17, 23)
FG = (201, 209, 217)
ACCENT = (88, 166, 255)

# 等宽字体：ASCII 的终端网格靠它对齐。
FONT_CANDIDATES = [
    "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
    "/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf",
    "/usr/share/fonts/TTF/DejaVuSansMono.ttf",
    "C:/Windows/Fonts/consola.ttf",
    "/mnt/c/Windows/Fonts/consola.ttf",
]

# CJK 字体：DejaVu/Liberation 没有汉字字形，中文会渲染成方框（□□□）。
# 演示的内容是中文语料，所以必须单独准备一个含汉字的字体，按双宽字符渲染。
CJK_FONT_CANDIDATES = [
    "/mnt/c/Windows/Fonts/msyh.ttc",       # Microsoft YaHei (WSL 访问宿主字体)
    "/mnt/c/Windows/Fonts/msyhl.ttc",
    "C:/Windows/Fonts/msyh.ttc",
    "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
    "/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
    "/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
    "/usr/share/fonts/truetype/arphic/uming.ttc",
]


def load_font(size=14):
    for path in FONT_CANDIDATES:
        if os.path.exists(path):
            try:
                return ImageFont.truetype(path, size)
            except Exception:
                pass
    return ImageFont.load_default()


def load_cjk_font(size=14):
    """返回一个含汉字字形的字体；找不到就返回 None（退化成方框，但至少不崩）。"""
    for path in CJK_FONT_CANDIDATES:
        if os.path.exists(path):
            try:
                return ImageFont.truetype(path, size)
            except Exception:
                pass
    return None


def char_cells(ch):
    """终端里一个字符占几列：CJK 全角占 2 列，其余占 1 列。"""
    if unicodedata.east_asian_width(ch) in ("W", "F"):
        return 2
    return 1


def parse_cast(path):
    events = []
    cols, rows = 100, 30
    with open(path, "r", encoding="utf-8", errors="replace") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            try:
                obj = json.loads(line)
            except Exception:
                continue
            if isinstance(obj, dict) and "version" in obj:
                cols = int(obj.get("width") or cols)
                rows = int(obj.get("height") or rows)
                continue
            if isinstance(obj, list) and len(obj) >= 3:
                t, kind, data = obj[0], obj[1], obj[2]
                if kind != "o":
                    continue
                try:
                    events.append((float(t), str(data)))
                except Exception:
                    continue
    events.sort(key=lambda e: e[0])
    return cols, rows, events


class Screen:
    def __init__(self, cols, rows):
        self.cols = cols
        self.rows = rows
        self.lines = [""]
        self.row = 0
        self.col = 0

    def _ensure(self, row):
        while len(self.lines) <= row:
            self.lines.append("")

    def feed(self, text):
        for ch in text:
            if ch == "\n":
                self.row += 1
                self.col = 0
                self._ensure(self.row)
            elif ch == "\r":
                self.col = 0
            elif ch == "\b":
                self.col = max(0, self.col - 1)
            elif ch == "\t":
                self.col = min(self.cols - 1, (self.col // 8 + 1) * 8)
            else:
                self._ensure(self.row)
                line = self.lines[self.row]
                if len(line) < self.col:
                    line = line + " " * (self.col - len(line))
                self.lines[self.row] = line[: self.col] + ch + line[self.col + 1 :]
                self.col += 1
                if self.col >= self.cols:
                    self.col = 0
                    self.row += 1
                    self._ensure(self.row)

    def visible(self):
        out = list(self.lines)
        while len(out) < self.rows:
            out.append("")
        return out[-self.rows :]


def render(out_gif, cast_path, fps=8, max_seconds=60.0):
    cols, rows, events = parse_cast(cast_path)
    if not events:
        raise SystemExit("cast contains no output events - refusing to render an empty animation")

    font = load_font()
    cjk = load_cjk_font()
    try:
        bbox = font.getbbox("M")
        cell_w = max(1, bbox[2] - bbox[0] + 1)
        cell_h = max(1, int((bbox[3] - bbox[1]) * 1.35))
    except Exception:
        cell_w, cell_h = 8, 16

    margin = 8
    width = cell_w * cols + margin * 2
    height = cell_h * rows + margin * 2

    total = events[-1][0]
    if total <= 0:
        total = 1.0

    screen = Screen(cols, rows)
    work = tempfile.mkdtemp(prefix="cast2gif-")
    frames_dir = os.path.join(work, "frames")
    os.makedirs(frames_dir, exist_ok=True)
    try:
        # 帧采样必须**覆盖到最后一个事件**，否则最后一屏内容会丢；
        # 采样必须**从第一个事件开始**，否则第 0 帧是空白 —— GIF 的缩略图
        # 就是第 0 帧，空白首帧在仓库预览里等于一张黑图。
        # 曾经的 bug：cast 只有 0.14s，int(0.14*8)==0 → 只出 1 帧、采样点 t=0
        # 还没喂任何事件 → 整段 GIF 全黑。
        first_t = events[0][0]
        last_t = events[-1][0]
        span = max(last_t - first_t, 1.0 / fps)
        frame_count = max(2, int(span * fps) + 1)
        hold_frames = max(1, int(fps * 1.5))
        states = []
        idx = 0
        for i in range(frame_count):
            t = first_t + span * i / float(frame_count - 1)
            while idx < len(events) and events[idx][0] <= t:
                screen.feed(events[idx][1])
                idx += 1
            states.append(list(screen.visible()))
        # 结尾停留：重复最后一屏若干帧，GIF 不会一闪而过。
        final_state = states[-1]
        for _ in range(hold_frames):
            states.append(list(final_state))

        for i, lines in enumerate(states):
            img = Image.new("RGB", (width, height), BG)
            draw = ImageDraw.Draw(img)
            draw.rectangle([0, 0, width - 1, height - 1], outline=(48, 54, 61))
            for r, line in enumerate(lines):
                y = margin + r * cell_h
                col = 0
                for ch in line:
                    if ch == " ":
                        col += 1
                        continue
                    cells = char_cells(ch)
                    color = ACCENT if ch in "$#" else FG
                    use = cjk if (cells == 2 and cjk is not None) else font
                    draw.text((margin + col * cell_w, y), ch, font=use, fill=color)
                    col += cells
            img.save(os.path.join(frames_dir, "f%05d.png" % i))

        ffmpeg = shutil.which("ffmpeg")
        if not ffmpeg:
            raise SystemExit("ffmpeg is required to assemble the GIF")
        cmd = [
            ffmpeg, "-y", "-loglevel", "error",
            "-framerate", str(fps),
            "-i", os.path.join(frames_dir, "f%05d.png"),
            "-filter_complex",
            "split[a][b];[a]palettegen=max_colors=128[p];[b][p]paletteuse=dither=bayer",
            out_gif,
        ]
        subprocess.run(cmd, check=True)
    finally:
        shutil.rmtree(work, ignore_errors=True)

    with open(out_gif, "rb") as fh:
        magic = fh.read(6)
    if magic not in (b"GIF87a", b"GIF89a"):
        raise SystemExit("rendered file is not a GIF (magic=%r)" % magic)
    return os.path.getsize(out_gif)


def main(argv):
    if len(argv) < 3:
        raise SystemExit("usage: cast-to-gif.py <input.cast> <output.gif> [--fps N] [--max-seconds N]")
    cast_path, out_gif = argv[1], argv[2]
    fps = 8
    max_seconds = 60.0
    i = 3
    while i < len(argv):
        if argv[i] == "--fps":
            fps = int(argv[i + 1]); i += 2
        elif argv[i] == "--max-seconds":
            max_seconds = float(argv[i + 1]); i += 2
        else:
            i += 1
    size = render(out_gif, cast_path, fps=fps, max_seconds=max_seconds)
    print("wrote %s (%d bytes)" % (out_gif, size))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
CAST2GIF_PY
      if "$PY" "$RENDERER" "$CAST" "$GIF" >/dev/null 2>&1 && [ -s "$GIF" ]; then
        RENDERED=1
        say "  renderer: python cast renderer + ffmpeg"
      fi
    fi
  fi

  if [ "$RENDERED" -ne 0 ] && [ -s "$GIF" ]; then
    MAGIC="$(head -c 6 "$GIF")"
    case "$MAGIC" in
      GIF87a | GIF89a) ;;
      *)
        say "[record-demo] the rendered file does not start with a GIF magic header (got '$MAGIC')"
        exit 1
        ;;
    esac
    BYTES="$(wc -c <"$GIF" | tr -d ' \r')"
    say "wrote docs/demo.gif ($BYTES bytes)"
  else
    say "[record-demo] no renderer available: need agg, or (asciinema + svg-term-cli + rsvg/cairosvg), or (python3 + Pillow)"
    exit 1
  fi
fi
