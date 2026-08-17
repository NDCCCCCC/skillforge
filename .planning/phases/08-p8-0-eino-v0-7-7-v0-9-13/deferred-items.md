# Phase 8 Deferred Items

> Out-of-scope discoveries logged during execution (scope boundary: only auto-fix issues directly caused by the current task's changes).

## 2026-08-17 — Plan 08-03: contract 套件冷缓存首跑可能超 10s 包级 timeout

- **发现于:** 08-03 Task 1 全套验证（`go test ./integration/contract -tags=contract -count=1 -timeout 10s -v`）
- **现象:** 冷缓存首跑 FAIL（10.576s 超包级 timeout）——`TestContractWireFormat` 内 `buildSpiked`（contract_test.go:518，sync.Once 30s build timeout）首次编译 spiked 二进制在冷缓存下可达 ~10s；warm cache 下全套 6.306s 全绿。
- **归因:** 既有套件特性，非 08-03 引入（08-03 仅新增 0.03s 纯文件扫描测试，且未触碰 contract_test.go）。08-01 记录的绿跑即 warm cache。
- **处置:** 不修（08-03 plan 明令不改 contract_test.go 的 RunsUnder10s 聚合；timeout 调整属套件维护决策）。若后续 phase 出现 CI 冷跑超时，可考虑：预热 build cache、或将包级 timeout 提至 60s（buildSpiked 自身已有 30s 上界，包级 10s 与之矛盾）。
