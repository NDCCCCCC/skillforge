#!/usr/bin/env bash
# verify-phase-53.sh — Phase 53 (摄取链路完整性) phase gate.
# ING-01 上传落盘可重建 / ING-02 单一分块器 / ING-03 doc_id 生命周期.
# 零 waiver：任何 SKIP/WAIVED 都不得维持表面绿灯（CI 裸跑不读本地 waiver）。
# 骨架沿 verify-phase-52.sh（fail-fast 分区）。
set -euo pipefail
cd "$(dirname "$0")"

pass=0; fail=0
ok()   { echo "--- PASS: $1"; pass=$((pass+1)); }
bad()  { echo "--- FAIL: $1"; fail=$((fail+1)); }

echo "=== P1: CGO build + vet ==="
CGO_ENABLED=1 go build ./...
CGO_ENABLED=1 go vet ./...
ok "P1 build+vet (CGO_ENABLED=1)"

echo "=== P2: 包级套件 ==="
go test ./internal/knowledge/ ./internal/server/ ./internal/retriever/... ./internal/kb/ -count=1
ok "P2 knowledge+server+retriever+kb 全绿"

echo "=== P3: 具名门（Phase 53 验收测试组）==="
# P3-a ING-02: 跨路径等价性（53-02 常驻门）
go test ./internal/knowledge/ -run '^TestUploadIndexCrossPathEquivalence$' -count=1 \
  && ok "P3-a 跨路径等价性: upload 与目录 chunk 序列逐字节一致" \
  || bad "P3-a 跨路径等价性"
# P3-b ING-01: 重启蒸发回归（53-04 核心：落盘形状经 reseed 存活）
go test ./internal/knowledge/ -run '^TestUploadPersistReseedSurvival$' -count=1 \
  && ok "P3-b 重启蒸发回归: 上传文档经 reseed 后 chunk/窗口不丢" \
  || bad "P3-b 重启蒸发回归"
# P3-c ING-03: upsert 重传（hash 未变 PK 保持）
go test ./internal/knowledge/ -run '^TestUploadIndexUpsertReuploadKeepsUnchangedPKs$' -count=1 \
  && ok "P3-c upsert 重传: 未变 chunk PK 保持/变化 chunk 新 PK" \
  || bad "P3-c upsert 重传"
# P3-d ING-03: DELETE 端点（rows+file+audit, traversal/404/跨 slug 守卫）
go test ./internal/server/ -run '^TestDocDelete' -count=1 \
  && ok "P3-d DELETE 端点: 语料文件与行同删, 守卫齐备" \
  || bad "P3-d DELETE 端点"
# P3-e ING-01: 落盘 fail-closed（persist 失败 → 422 且零 Insert）
go test ./internal/server/ -run '^TestUploadPersistFailureRejectedBeforeInsert$' -count=1 \
  && ok "P3-e 落盘 fail-closed: Insert 前落盘, 失败零 Insert" \
  || bad "P3-e 落盘 fail-closed"
# P3-f ING-01: 落盘形状（provenance frontmatter + 窗口透传）
go test ./internal/server/ -run '^TestPersistUploadWritesKnowledgeFile$' -count=1 \
  && ok "P3-f 落盘形状: provenance frontmatter + 窗口透传" \
  || bad "P3-f 落盘形状"

echo "=== P4: 全量冒烟（非确定性证据，仅冒烟门槛）==="
go test ./internal/... -count=1
ok "P4 ./internal/... 全量"

echo ""
echo "=== verify-phase-53 汇总: PASS=$pass FAIL=$fail ==="
if [ "$fail" -ne 0 ]; then
  echo "GATE: RED — Phase 53 未通过（零 waiver 纪律：不得用 SKIP 掩盖）"
  exit 1
fi
echo "GATE: GREEN — Phase 53 摄取链路完整性收口"
