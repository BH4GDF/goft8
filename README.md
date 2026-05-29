# goft8

A Go implementation of the FT8 digital mode encoder and decoder.

## Features

- **Decoder**: Full FT8 decode pipeline (sync8, LDPC BP+OSD, CRC, message unpack)
  ported from WSJT-X 2.7.0 algorithms.
- **Encoder**: Full message packing matching MSHV priority order:
  - Standard messages (CQ, grid, reports, RRR/RR73/73, /R /P suffixes)
  - DXpedition, ARRL Field Day, ARRL RTTY Contest, EU VHF Contest
  - Non-standard callsigns, Telemetry, Free text
  - CRC-14, LDPC (174,91), GFSK tone generation, waveform synthesis
- **Pure Go**: No CGO dependencies. Works on any Go-supported platform.

## Installation

```bash
go get github.com/bh4gdf/goft8
```

## Quick Start

### Decode a WAV file

```go
package main

import (
    "fmt"
    "log"
    "github.com/bh4gdf/goft8"
)

func main() {
    decodes, err := DecodeWAV("ft8_cap1.wav",
        WithDepth(DepthDeep),
        WithMyCall("W1ABC"),
    )
    if err != nil {
        log.Fatal(err)
    }
    for _, d := range decodes {
        fmt.Printf("%3d dB  %s\n", d.SNR, d.Message)
    }
}
```

### Encode a message

```go
package main

import (
    "log"
    "github.com/bh4gdf/goft8"
)

func main() {
    enc := NewEncoder(WithTxFreq(1500))
    waveform, err := enc.Encode("CQ W1ABC FN20")
    if err != nil {
        log.Fatal(err)
    }
    // waveform is []float32 at 12 kHz mono, length NTXSamples (151680)
}
```

## Architecture

```
goft8/
├── *.go               # FT8 core implementation
│   ├── decoder.go     # Public Decoder API
│   ├── encoder.go     # Public Encoder API
│   ├── pack77.go      # Message packing
│   ├── unpack77.go    # Message unpacking
│   ├── encode.go      # Tone & waveform generation
│   ├── decode.go      # Decode pipeline
│   ├── ldpc.go        # LDPC BP+OSD decoder
│   ├── crc.go         # CRC-14
│   ├── sync8.go       # Costas sync search
│   ├── fft.go         # Pure-Go mixed-radix FFT
│   └── ...
└── testdata/          # Test WAV captures
```

## License

GPL v3 — see [LICENSE](LICENSE).

Parts of the decoder implementation are derived from
[goft8](https://github.com/ColonelBlimp/goft8) by ColonelBlimp (MIT),
which is a clean-room port of WSJT-X 2.7.0 Fortran sources.
