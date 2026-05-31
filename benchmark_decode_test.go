package goft8

import (
	"testing"
)

func BenchmarkDecodeWAVCap1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := DecodeWAV(
			"testdata/ft8_cap1.wav",
			WithFreqRange(200, 2600),
			WithDepth(DepthDeep),
			WithAPEnabled(true),
			WithCQOnlyAP(true),
			WithWorkers(-1),
		)
		if err != nil {
			b.Fatalf("DecodeWAV: %v", err)
		}
	}
}

func BenchmarkDecodeWAVCap2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := DecodeWAV(
			"testdata/ft8_cap2.wav",
			WithFreqRange(200, 2600),
			WithDepth(DepthDeep),
			WithAPEnabled(true),
			WithCQOnlyAP(true),
			WithWorkers(-1),
		)
		if err != nil {
			b.Fatalf("DecodeWAV: %v", err)
		}
	}
}

func BenchmarkDecodeWAVCap3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := DecodeWAV(
			"testdata/ft8_cap3.wav",
			WithFreqRange(200, 2600),
			WithDepth(DepthDeep),
			WithAPEnabled(true),
			WithCQOnlyAP(true),
			WithWorkers(-1),
		)
		if err != nil {
			b.Fatalf("DecodeWAV: %v", err)
		}
	}
}

func BenchmarkDecodeWAVCap4(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := DecodeWAV(
			"testdata/ft8_cap4.wav",
			WithFreqRange(100, 3000),
			WithDepth(DepthDeep),
			WithAPEnabled(true),
			WithCQOnlyAP(true),
			WithWorkers(-1),
		)
		if err != nil {
			b.Fatalf("DecodeWAV: %v", err)
		}
	}
}
