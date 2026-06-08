# GoFT8 Project Map

## Layout

- Root package: public API, encoder/decoder facades, options, WAV helpers.
- `internal/decode`: sync search, candidate metrics, AP handling, subtract pipeline.
- `internal/encode`: FT8 tone generation and waveform synthesis.
- `internal/dsp`: FFT and real FFT utilities.
- `internal/ldpc`: LDPC encode/decode and CRC.
- `internal/protocol`: message packing, unpacking, hash tables.
- `cmd/decodewav` and `cmd/genwav`: CLI tools.
- `testdata`: fixture WAV captures and generated test artifacts.

## Current Improvement Themes

- P0: Fix comments and option behavior mismatches. Completed first pass.
- P1: Consolidate WAV parsing and improve CLI error handling. Completed first pass.
- P2: Add CI and internal package tests. Completed first pass.
- P3: Reduce decode and multi-message encode allocations.
- P4: Add fuzz tests and malformed WAV/message hardening.
- P5: Clarify Fox/Hound unimplemented behavior.

## Baseline Commands

- Full tests: `go test ./...`
- Race tests: `go test -race ./...`
- Coverage: `go test -cover ./...`
- Encode/decode sample benchmark: `go test -bench='Benchmark(DecodeWAVCap1|EncoderEncode)$' -benchmem .`

## Known Baseline

- Root package coverage is about 54%.
- `internal/encode` coverage is about 81%.
- `internal/dsp` coverage is about 39%.
- `internal/protocol` coverage is about 32%.
- `internal/ldpc` coverage is about 8%.
- `BenchmarkEncoderEncode` is about `6.65 ms/op, 7.4 MB/op`.
- `BenchmarkDecodeWAVCap1` is about `496 ms/op, 46 MB/op`.
