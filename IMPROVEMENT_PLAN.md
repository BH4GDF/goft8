# 项目改进计划

## 目标与当前证据

本计划面向 6 人团队：1 名项目管理、2 名开发、1 名安全顾问、1 名性能测试、1 名功能测试。当前项目是 Go 模块 `github.com/bh4gdf/goft8`，包含 FT8 编码、解码、WAV I/O、CLI 工具与内部算法包。基线验证结果：

- `go test ./...` 通过。
- `go test -race ./...` 通过。
- `go test -cover ./...`：根包覆盖率约 `63.3%`；`internal/decode` 约 `28.1%`，`internal/encode` 约 `80.6%`，`internal/dsp` 约 `38.7%`，`internal/protocol` 约 `39.6%`，`internal/ldpc` 新增 deterministic decoder tests 后约 `81.7%`。
- 代表性 benchmark：`BenchmarkEncoderEncode` 约 `6.65 ms/op, 7.4 MB/op`；`BenchmarkDecodeWAVCap1` 首轮优化后约 `486 ms/op, 40 MB/op`，二轮 decode/LDPC 分配优化后约 `475-492 ms/op, 31.5-32.8 MB/op, 810-857 allocs/op`（`-benchtime=5x -count=3`）。
- 新增微基准：`BenchmarkGetSpectrumBaselineSerial` 约 `5.01 ms/op, 24 B/op`；`BenchmarkOSDDecodeOrder3Clean` 约 `1.45 ms/op, 0 allocs/op`。
- `EncodeMulti` 二轮池化后：`BenchmarkEncoderEncodeMulti2_12k` 约 `1.75-1.85 ms/op, 0.64-0.69 MB/op`；`BenchmarkEncoderEncodeMulti2_48k` 约 `7.14-7.56 ms/op, 7.4 MB/op`（`-benchtime=50x -count=3`）。
- 编码池化三轮后：`BenchmarkEncoderEncodeToBytes` 约 `1.83 MB/op, 3 allocs/op`；`BenchmarkEncoderEncodeMulti2_48k` 约 `2.43 MB/op, 13 allocs/op`。
- 解码 metrics 优化后：新增 `BenchmarkComputeSymbolSpectra`、`BenchmarkComputeSoftMetrics`、`BenchmarkSync8d`、`BenchmarkHardSync`；`BenchmarkComputeSoftMetrics` 从 `132-151 us/op` 降到 `21.9-25.2 us/op`，保持 `0 allocs/op`（`-benchtime=500x -count=10`）。
- 当前状态：P0、P1、P2、P3、P4、P5 已完成首轮实现。

## 主要可改进项

### P0：修正文档与 API 行为不一致（已完成首轮）

- `decoder_options.go` 中 `WithFreqRange` 注释写默认 `200..3000 Hz`，实际默认是 `100..3000 Hz`。
- `encoder.go` 中 `WithSampleRate` 注释写 `12000` 为默认，实际 `NewEncoder()` 默认 `48000`。
- `WithSampleRate` 和 `WithBitDepth` 当前只记录配置，不显式拒绝非法值；`internal/encode.EncodeParams` 会把非 `48000` 的采样率按 12 kHz 符号长度处理，容易产生静默错误。

验收标准：修正文档；为非法采样率、位深增加测试；决定非法配置是回退默认还是返回错误，并在 README/API 注释中说明。

### P1：统一 WAV 读取与 CLI 输入处理（已完成首轮）

- `decoder.go`、`wav.go`、`cmd/decodewav/main.go` 存在重复 WAV 解析逻辑。
- CLI 中部分 `Read`、`Seek` 调用未完整检查返回值。
- 库函数 `ReadWAVMono12k` 只支持 12 kHz PCM16/float32，而 `WriteWAV` 与 CLI 覆盖 16/24/32-bit、12/48 kHz，能力边界不统一。

验收标准：提取共享 WAV reader；使用 `io.ReadFull` 或等价完整读；增加截断文件、非 mono、非法 chunk、24/32-bit、48 kHz 的测试。

### P2：补强测试与 CI（已完成首轮）

- 端到端测试已覆盖部分捕获文件，但 `internal/dsp`、`internal/encode`、`internal/ldpc`、`internal/protocol` 缺少包级覆盖。
- 已新增 GitHub Actions CI，执行 `go test ./...` 和 `go test -race ./...`。

验收标准：新增 GitHub Actions 或等价 CI，执行 `go test ./...`、`go test -race ./...`；为协议 packing、PCM 转换、非法选项、CLI smoke path 增加测试。

### P3：性能与内存优化（已完成首轮）

- 解码基线约 `46 MB/op`、`3608 allocs/op`，适合优先做分配热点治理。
- `EncodeMulti` 的缓冲池固定回收 `NTXSamples` 长度，48 kHz 场景会反复分配大缓冲，影响多消息编码性能。

验收标准：保留当前 benchmark；增加 `EncodeMulti` 48 kHz benchmark；首轮目标为解码分配降低 15% 以上，编码多消息分配降低 25% 以上，且解码结果不变。（首轮已完成）

### P4：安全与健壮性（已完成首轮）

- 对 RIFF/WAV chunk size 做上限保护，避免异常文件导致大内存分配。
- 对消息解析、WAV 解析增加 fuzz 测试。
- `cmd/genwav/*.go` 的 ignore 工具中存在 `panic`，应改为明确错误输出，避免复制到正式命令时保留坏习惯。

验收标准：新增 fuzz 入口；恶意/畸形输入不 panic、不超大分配；CLI 错误路径返回非零退出码和清晰错误。（首轮已完成）

### P5：功能边界澄清（已完成首轮）

- `foxhound.go` 的 `DecodeFoxHound` 当前无条件返回 `nil`，调用方难以区分“无解码结果”和“功能未实现”。

验收标准：将 API 调整为返回错误，或在文档中明确未实现状态；增加测试固定当前行为，避免误用。（首轮已完成）

## 角色分工

### 项目管理

- 维护改进 backlog、优先级和每周验收清单。
- 将 P0/P1/P2 设为第一阶段交付，P3/P4/P5 按风险和资源排入第二阶段。
- 每个 PR 必须包含变更说明、验证命令、风险说明。

### 开发 1

- 负责 P0 与 P1：修正文档/API 行为、统一 WAV reader、补齐错误处理。
- 输出对应单元测试和 README/API 注释更新。

### 开发 2

- 负责 P2 与 P5：补内部包测试、CI 配置、Fox/Hound API 边界澄清。
- 与功能测试协作维护端到端 fixture 预期结果。

### 安全顾问

- 评审 WAV/message 输入边界、chunk size 限制、fuzz 覆盖范围。
- 制定畸形文件测试集，确认所有错误路径不 panic、不泄露资源。

### 性能测试

- 固化 benchmark 命令和基线表。
- 针对解码分配、`EncodeMulti` 48 kHz、多 worker 解码建立性能报告，避免优化引入行为回归。

### 功能测试

- 建立手工与自动化验收用例：正常 WAV 解码、生成 WAV 再解码、非法参数、非法文件、CLI 输出格式。
- 每次功能变更确认 README 示例仍可运行。

## 建议排期

- 第 1 周：完成 P0、CI 初版、WAV/配置测试用例设计。
- 第 2 周：完成共享 WAV reader、CLI 错误处理、畸形输入测试。
- 第 3 周：完成内部包测试补强、Fox/Hound 边界调整、README 更新。
- 第 4 周：完成性能热点优化、fuzz 集成、最终回归报告。

## 完成定义

- 所有 P0/P1/P2 项完成并合入。
- P0-P5 均完成首轮实现。
- `go test ./...`、`go test -race ./...`、关键 benchmark 均有记录。
- 文档、测试、CLI 行为与公开 API 保持一致。
