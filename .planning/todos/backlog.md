
## 2026-11-23 impl-53-01 执行中顺带发现的既有缺陷（未修，超 53-01 范围）

- **[server] STEP 7b 无 HTTP 响应**：upload.go 的 UploadIndex 失败分支写审计但不写 HTTP 响应——客户端拿到空体。修法：失败分支补 422 JSON。
- **[knowledge] 非 md 上传的 chunk 源是原始二进制**：PDF/DOCX 上传时 UploadIndex 的 md 分支判断外直接对原始字节分块嵌入——向量库嵌的是二进制字节。53-01 的落盘侧已让磁盘拷贝正确（parser Content），Milvus 侧要等 reseed 收敛；根治需 UploadIndex 对非 md 类型先走 parser 再分块（与 53-02 的统一分块器衔接）。
- **[server] upload.go:224 nilness 警告**：嵌套 if err != nil 为既有代码，golangci-lint nonnil 类提示。
