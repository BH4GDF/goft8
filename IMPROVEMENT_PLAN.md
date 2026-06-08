# 项目改进计划

## 完成状态

本计划面向 6 人团队：1 名项目管理、2 名开发、1 名安全顾问、1 名性能测试、1 名功能测试。当前项目是 Go 模块 `github.com/bh4gdf/goft8`，覆盖 FT8 编码、解码、WAV I/O、CLI 工具与内部算法包。

P0-P5 首轮目标已完成并通过当前审计。最新验证：

- `go test ./...` 通过。
- `go test -race ./...` 通过。
- `go test -cover ./...` 通过：根包 `63.7%`；`cmd/decodewav` `75.0%`；`cmd/genwav` `78.6%`；`internal/decode` `39.2%`；`internal/dsp` `50.2%`；`internal/encode` `80.6%`；`internal/ldpc` `81.6%`；`internal/protocol` `71.4%`。
- 关键 benchmark 已记录：`BenchmarkDecodeWAVCap1` 约 `471-497 ms/op, 32-35 MB/op, 799-849 allocs/op`；`BenchmarkEncoderEncodeToBytes` 约 `1.83 MB/op, 3 allocs/op`；`BenchmarkEncoderEncodeMulti2_48k` 约 `2.43 MB/op, 13 allocs/op`；decode metrics 与 LDPC 微基准保持 `0 allocs/op`，`BenchmarkOSDDecodeOrder3Clean` 约 `1.29-1.42 ms/op`。

## 验收证据

### P0：文档与 API 行为一致

- `WithFreqRange` 注释与默认 `100..3000 Hz` 一致。
- `WithSampleRate` 注释与默认 `48000` 一致，仅支持 `12000`、`48000`。
- `WithBitDepth` 仅支持 `16`、`24`、`32`；`EncodeToBytes` 和 `WriteWAV` 对非法位深返回错误。
- 测试覆盖非法采样率、非法位深、默认编码配置、`WriteWAV` 输出参数校验和 PCM 输出匹配。

### P1：统一 WAV 读取与 CLI 输入处理

- `ReadWAVMono`、`ReadWAVMono12k`、`ReadWAVParams` 共享 `wav.go` reader。
- WAV chunk/header/data 使用 `io.ReadFull` 或等价完整读，未知 chunk 支持 padding 跳过。
- 测试覆盖截断文件、非 mono、非法 chunk、oversized chunk、PCM 16/24/32-bit、float32、48 kHz downsample。
- `cmd/decodewav` 使用共享 reader，错误路径返回非零退出码。

### P2：测试与 CI

- GitHub Actions 在 `.github/workflows/ci.yml` 执行 `go test ./...` 和 `go test -race ./...`。
- 已补强协议 packing、LDPC、DSP FFT/buffer、decode AP/downsample、PCM 转换、非法选项和 CLI smoke 测试。
- 测试仍按 Go 标准 `*_test.go` co-located 组织。

### P3：性能与内存优化

- 保留 encode/decode、decode micro、LDPC benchmark。
- 新增并记录 `BenchmarkEncoderEncodeMulti2_48k`。
- 解码分配相对初始基线已从约 `46 MB/op, 3608 allocs/op` 降到约 `32-35 MB/op, 799-849 allocs/op`。
- `EncodeMulti` 48 kHz 多消息分配已降到约 `2.43 MB/op, 13 allocs/op`。
- fixture decode 测试保持结果稳定。

### P4：安全与健壮性

- WAV fmt/data chunk 和输出 data size 有大小与对齐检查，畸形输入返回错误而不是 panic。
- 已新增 `FuzzParseMessage`、`FuzzReadWAVMono`、`internal/protocol.FuzzPack77`。
- `cmd/genwav` 正式命令和 ignore 工具均使用明确 stderr 错误和非零退出码；短文件、未对齐 PCM 不再运行时 panic。
- LDPC 内部排序栈按需扩容，避免固定 `NSTACK` 保护触发 panic。

### P5：功能边界澄清

- `DecodeFoxHound` API 注释明确该功能尚未实现且当前固定返回 `nil`。
- `foxhound_test.go` 固定 placeholder 行为，避免调用方把 `nil` 误解为已完成的 Fox/Hound 判定。

## 后续可选 Backlog

- 将 `internal/decode` 覆盖率继续提升到 50% 以上，重点覆盖 sync/subtract 边界。
- 为 fuzz 任务增加 CI 定时短跑，例如 `go test -run '^$' -fuzz=FuzzReadWAVMono -fuzztime=30s`。
- 增加 benchmark 结果归档脚本，便于对比不同 CPU 和 Go 版本。
- 评估 Fox/Hound 是否进入正式里程碑；若进入，应改造 API 为返回 `(results, error)` 或新增 v2 API。

## PR Gate

每个后续 PR 需要包含：

- 行为变更说明和风险说明。
- 运行过的验证命令，至少包含相关包测试；共享代码改动需包含 `go test ./...`。
- 性能敏感改动需包含 `go test -bench ... -benchmem` 对比。
- CLI、WAV 或公开 API 变更需同步 README/API 注释与错误路径测试。
