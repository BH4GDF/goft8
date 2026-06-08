# FFTW 分支 main 同步执行计划

本文档是给 AI agent 执行的同步任务包。目标是在 `fftw` 分支逐步吸收 `main` 的质量、测试、CLI、WAV 和性能更新，同时保留 FFTW3 CGO FFT 与 MSHV C++ LDPC/OSD 解码路径。

## 全局约束

- 执行分支：`fftw`。
- 共同祖先：`47209a36c06c9784a8e0f2a4fbc5fe9791478a17`。
- `main` 基线：`b969a6b94db87e793d6118b9da1b8c03b0e26fff`。
- `fftw` 基线：`66f9d6d0944aa355bfe71161b8a6e4c28a7998dc`。
- 必须保留：`internal/dsp/fft_fftw.go`、`internal/ldpc/decode_cgo.go`、`internal/ldpc/mshv_decode.cpp`、`internal/ldpc/mshv_decode.h`。
- 不允许在同步任务里直接用 `main` 的纯 Go `internal/dsp/fft_gonum.go` 替换 FFTW 路径。
- 每个任务只做一个主题；失败时停止在当前任务，不继续扩大范围。

## 前置环境

AI agent 开始任一任务前执行：

```bash
git status --short --branch
git log --oneline --left-right --cherry-pick fftw...main
pkg-config --exists fftw3
go test ./...
```

通过条件：

- 工作树没有无关改动，或已明确识别并避开用户改动。
- `pkg-config --exists fftw3` 成功；否则先安装 FFTW3 开发库，不能用纯 Go fallback 绕过。
- 基线 `go test ./...` 成功。

## 质量与性能守门

功能质量必须满足：

- `testdata/ft8_cap*.wav` fixture decode 消息集合不减少；频率、DT、SNR 只允许在已有测试容差内变化。
- `DecodeWAV`、`ReadWAVMono12k`、`cmd/decodewav` 对同一 WAV 的采样率处理一致。
- WAV malformed input 必须返回 error，不能 panic，不能触发明显超大分配。
- CLI 新增输出不能破坏默认人类可读输出；JSON 输出必须是稳定 schema。

性能质量必须满足：

- Decode 性能敏感任务必须记录修改前后：
  `go test -run '^$' -bench 'BenchmarkDecodeWAVCap1' -benchmem .`
- Encoder 性能敏感任务必须记录修改前后：
  `go test -run '^$' -bench 'BenchmarkEncoder|BenchmarkEncodeMulti' -benchmem .`
- LDPC/DSP 性能敏感任务必须记录修改前后：
  `go test -run '^$' -bench . -benchmem ./internal/ldpc ./internal/dsp`
- 没有明确算法收益时，不接受超过 10% 的稳定回退；decode allocs/op 和 bytes/op 不应高于任务前基线。
- FFTW plan pool、shared buffer、goroutine 或 CGO 共享状态变化后必须运行：
  `go test -race ./...`

可靠性前提：

- CI 必须在 Ubuntu runner 安装 `libfftw3-dev` 后运行测试。
- fuzz workflow 只做 smoke，不替代 malformed WAV 单元测试。
- CGO 边界不能持有 Go pointer 到 C 长期保存；FFTW malloc/free 和 plan destroy 必须配对。
- 如果 FFTW 与纯 Go 数值结果存在差异，只能用明确容差处理，不能放宽到掩盖解码错误。

## 可行性评估

总体可行，但不适合一次性 merge。

- 配置、agent、skill、MCP、CI：高可行性，已可直接落地。
- 协议测试、Fox/Hound placeholder、API option 测试：高可行性，冲突少，适合第一批。
- WAV hardening 与 `cmd/decodewav` JSON：中高可行性，需要先统一 reader，风险主要在 PCM16 decoder scaling。
- Encoder pooling：中高可行性，风险在 waveform 等价和 buffer 生命周期。
- Decode allocation 优化：中等可行性，冲突集中且影响核心质量，需要 fixture、race、benchmark 三重验证。
- LDPC 覆盖与 benchmark：中等可行性，必须围绕 MSHV CGO decode 改测试，不能直接套纯 Go 假设。
- optional pure-Go fallback/build tags：另立设计任务，不纳入本同步计划。

## 执行状态

本计划已作为 AI 可执行任务包在 `fftw` 分支落地。执行策略是先整体引入 main 的质量、测试、CLI、WAV、性能和配置更新，再恢复并适配 `fftw` 专属 native 后端，最后用测试、fuzz、race、benchmark 和后端 guard 做合流审计。

- T0 配置同步：完成。已增加 `.agents/`、`.codex/skills/goft8-improvement/`、`.codex/skills/goft8-fftw-sync/`、`.mcp.json`、`.codex/mcp.toml`、GitHub CI/fuzz workflow、`AGENTS.md` 和本计划。
- T1 低风险测试移植：完成。已移植 API、protocol、Fox/Hound placeholder、fuzz 和内部包测试。
- T2 WAV hardening 与共享 reader：完成。库和 CLI 使用共享 WAV reader，覆盖 malformed input、PCM16/24/32、float32、chunk padding、48 kHz downsample。
- T3 decodewav flags 与 JSON 输出：完成。默认文本输出保持可读，`-json` 输出已通过 fixture smoke 验证。
- T4 Encoder pooling 与输出等价：完成。保留波形/编码语义，新增 encoder 和 EncodeMulti benchmark/test 覆盖。
- T5 Decode workspace 与 soft metrics 优化：完成。围绕 FFTW spectrogram 适配 main 的 allocation 优化，没有降低 depth、candidate 或 AP 行为。
- T6 LDPC 覆盖与 benchmark：完成。`DecodeLDPC` 仍走 MSHV CGO，纯 Go BP/OSD 逻辑仅作为分支内保留实现和覆盖参考。
- T7 CI、fuzz、race 完整化：完成。CI/fuzz workflow 在 Ubuntu runner 安装 `libfftw3-dev` 后执行。
- T8 最终合流审计：完成。`internal/dsp/fft_gonum.go` 未引入；FFTW/MSHV 后端文件仍存在并被调用。

## 最终验收证据

功能和可靠性命令已通过：

```bash
pkg-config --exists fftw3
go test ./...
go test ./cmd/decodewav
go run ./cmd/decodewav testdata/ft8_cap1.wav
go run ./cmd/decodewav -json testdata/ft8_cap1.wav
go test -run 'TestReadWAV|TestWriteWAV|TestDecodeCaptures|TestDecodeWAV' .
go test ./internal/decode ./internal/dsp ./internal/ldpc ./internal/protocol
go test ./cmd/genwav
go test -race ./...
go test -run '^$' -fuzz=FuzzParseMessage -fuzztime=30s .
go test -run '^$' -fuzz=FuzzPack77 -fuzztime=30s ./internal/protocol
go test -run '^$' -fuzz=FuzzReadWAVMono -fuzztime=30s .
```

配置和 AI 执行环境验证已通过：

```bash
python3 /home/yida/.codex/skills/.system/skill-creator/scripts/quick_validate.py .codex/skills/goft8-improvement
python3 /home/yida/.codex/skills/.system/skill-creator/scripts/quick_validate.py .codex/skills/goft8-fftw-sync
python3 -c 'import json, pathlib; json.loads(pathlib.Path(".mcp.json").read_text())'
python3 -c 'import tomllib, pathlib; tomllib.loads(pathlib.Path(".codex/mcp.toml").read_text())'
python3 -c 'import pathlib, yaml; [yaml.safe_load(p.read_text()) for p in pathlib.Path(".github/workflows").glob("*.yml")]'
test -f internal/dsp/fft_fftw.go
test -f internal/ldpc/decode_cgo.go
test -f internal/ldpc/mshv_decode.cpp
test -f internal/ldpc/mshv_decode.h
test ! -f internal/dsp/fft_gonum.go
```

性能证据：

- `fftw` 修改前重复 decode 基线存在环境波动：`BenchmarkDecodeWAVCap1` 约 `371-464 ms/op`，约 `168-171 MB/op`，约 `7293-7346 allocs/op`。
- 同步后重复 decode 结果约 `405-438 ms/op`，约 `29-37 MB/op`，约 `6360-6399 allocs/op`；内存显著下降，墙钟时间处于本机 FFTW 基线波动区间内，未确认存在超过 10% 的稳定性能回退。
- 同步后全量 benchmark 样本：`BenchmarkDecodeWAVCap1` 约 `457 ms/op, 32.7 MB/op, 6424 allocs/op`；`BenchmarkDecodeWAVCap2` 约 `568 ms/op, 50.0 MB/op, 7416 allocs/op`；`BenchmarkDecodeWAVCap3` 约 `564 ms/op, 45.6 MB/op, 5093 allocs/op`；`BenchmarkDecodeWAVCap4` 约 `339 ms/op, 36.7 MB/op, 6543 allocs/op`。
- Encoder 分配改善：`BenchmarkEncoderEncodeMulti2_48k` 约 `16.3 ms/op, 2.43 MB/op, 13 allocs/op`。
- LDPC CGO benchmark：`BenchmarkDecodeLDPCCleanBP` 约 `20.3 us/op, 3216 B/op, 5 allocs/op`。
- 最终收尾补跑样本：`BenchmarkDecodeWAVCap1` 约 `271 ms/op, 36.3 MB/op, 6396 allocs/op`；`BenchmarkEncoderEncodeMulti2_48k` 约 `7.96 ms/op, 2.43 MB/op, 13 allocs/op`；`BenchmarkDecodeLDPCCleanBP` 约 `21.5 us/op, 3216 B/op, 5 allocs/op`。

## 暂缓项

- 不在本轮加入 optional pure-Go fallback 或 build tags；`fftw` 仍是 native FFTW/MSHV 分支。
- 不移植会替换 `internal/dsp/fft_fftw.go` 或绕过 `decodeLDPCCGO` 的 main 分支实现。
- 后续若要支持无 FFTW 环境，应另立设计任务，明确 build tags、CI matrix、API 兼容性和 benchmark 对比。

## 任务 T0：配置同步

目标：让 `fftw` 分支具备 AI 协作、MCP、CI 和同步计划。

可修改：

- `.agents/`
- `.codex/`
- `.mcp.json`
- `.github/workflows/ci.yml`
- `.github/workflows/fuzz.yml`
- `AGENTS.md`
- `IMPROVEMENT_PLAN.md`
- `FFTW_SYNC_PLAN.md`

验收：

```bash
python3 /home/yida/.codex/skills/.system/skill-creator/scripts/quick_validate.py .codex/skills/goft8-improvement
python3 /home/yida/.codex/skills/.system/skill-creator/scripts/quick_validate.py .codex/skills/goft8-fftw-sync
python3 -c 'import json, pathlib; json.loads(pathlib.Path(".mcp.json").read_text())'
python3 -c 'import tomllib, pathlib; tomllib.loads(pathlib.Path(".codex/mcp.toml").read_text())'
go test ./...
```

## 任务 T1：低风险测试移植

目标：移植不改变生产代码或只触及测试辅助函数的 `main` 覆盖。

验收：

```bash
go test ./internal/protocol .
go test ./...
```

## 任务 T2：WAV hardening 与共享 reader

目标：把 `main` 的 robust WAV reader 移植到 `fftw`，并统一库和 CLI 输入路径。

必须保留：

- `ReadWAVMono12k` 对 12 kHz 与 48 kHz 的支持。
- PCM16 decoder scaling 与 MSHV 对齐；如沿用 `0.000390625`，测试要明确覆盖。

验收：

```bash
go test -run 'TestReadWAV|TestWriteWAV|TestDecodeCaptures|TestDecodeWAV' .
go test ./cmd/decodewav .
go test ./...
```

## 任务 T3：decodewav flags 与 JSON 输出

目标：移植 `main` 的 `cmd/decodewav` flags 和 JSON 输出，建立稳定 CLI contract。

验收：

```bash
go test ./cmd/decodewav
go run ./cmd/decodewav testdata/ft8_cap1.wav
go run ./cmd/decodewav -json testdata/ft8_cap1.wav
go test ./...
```

## 任务 T4：Encoder pooling 与输出等价

目标：移植 `main` 的 encoder buffer reuse/pooling，降低分配且不改变波形语义。

验收：

```bash
go test -run 'TestEncoder|TestEncode|TestWriteWAV' .
go test ./internal/encode .
go test -run '^$' -bench 'BenchmarkEncoder|BenchmarkEncodeMulti' -benchmem .
go test ./...
```

## 任务 T5：Decode workspace 与 soft metrics 优化

目标：移植 `main` 的 decode allocation 优化，同时保证 FFTW spectrogram 和 MSHV LDPC 解码质量。

禁止：

- 不替换 FFTW FFT 实现。
- 不通过降低 depth、减少 candidate 或跳过 AP 来换取性能。
- 不删除已有 fixture decode 断言。

验收：

```bash
go test -run 'TestDecodeCaptures|TestDecodeStream|TestDecodeWAV' .
go test ./internal/decode
go test -run '^$' -bench 'BenchmarkDecodeWAVCap1|BenchmarkSync|BenchmarkSoft' -benchmem . ./internal/decode
go test -race ./...
go test ./...
```

## 任务 T6：LDPC 覆盖与 benchmark

目标：补齐 LDPC 测试、benchmark 和 hardening，同时保留 MSHV CGO decode 为 `fftw` 基准。

禁止：

- 不删除或绕过 `decodeLDPCCGO`。
- 不把 MSHV C++ 源码替换为 Go 实现。

验收：

```bash
go test ./internal/ldpc
go test -run '^$' -bench . -benchmem ./internal/ldpc
go test ./...
```

## 任务 T7：CI、fuzz、race 完整化

目标：让 `fftw` 分支 CI 与本地质量门一致。

验收：

```bash
go test ./...
go test -race ./...
go test -run '^$' -fuzz=FuzzParseMessage -fuzztime=30s .
go test -run '^$' -fuzz=FuzzPack77 -fuzztime=30s ./internal/protocol
```

## 任务 T8：最终合流审计

目标：确认 `fftw` 已吸收可移植的 `main` 更新，并记录有意暂缓项。

操作：

```bash
git log --oneline --left-right --cherry-pick fftw...main
git diff --name-status $(git merge-base fftw main)..main
go test ./...
go test -race ./...
go test -bench . -benchmem ./...
```

通过条件：

- 所有已移植任务的验收命令通过。
- 未移植项都有明确理由：不适用、已由 FFTW 分支替代、或拆分到后续设计任务。
- 解码质量无回退，性能无未经解释的稳定回退。
