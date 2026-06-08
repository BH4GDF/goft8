package decode

import (
	"math"
	"testing"

	ft8params "github.com/bh4gdf/goft8/params"
)

var (
	benchmarkCSSink    [8][ft8params.NN]complex128
	benchmarkS8Sink    [8][ft8params.NN]float64
	benchmarkBmetSink  [174]float64
	benchmarkSyncSink  float64
	benchmarkNsyncSink int
)

func BenchmarkComputeSymbolSpectra(b *testing.B) {
	cd0 := metricsTestCD0()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkCSSink, benchmarkS8Sink = ComputeSymbolSpectra(cd0, 100)
	}
}

func BenchmarkComputeSoftMetrics(b *testing.B) {
	cd0 := metricsTestCD0()
	cs, _ := ComputeSymbolSpectra(cd0, 100)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bmeta, bmetb, bmetc, bmetd, bmete := ComputeSoftMetrics(&cs)
		benchmarkBmetSink = bmeta
		benchmarkBmetSink = bmetb
		benchmarkBmetSink = bmetc
		benchmarkBmetSink = bmetd
		benchmarkBmetSink = bmete
	}
}

func BenchmarkSync8d(b *testing.B) {
	cd0 := metricsTestCD0()
	var ctwk [32]complex128
	for i := range ctwk {
		ctwk[i] = 1
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkSyncSink = Sync8d(cd0, 100, ctwk, 0)
	}
}

func BenchmarkSync8dTwk(b *testing.B) {
	cd0 := metricsTestCD0()
	var ctwk [32]complex128
	for i := range ctwk {
		phi := 2 * math.Pi * float64(i) / 32
		ctwk[i] = complex(math.Cos(phi), math.Sin(phi))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkSyncSink = Sync8d(cd0, 100, ctwk, 1)
	}
}

func BenchmarkHardSync(b *testing.B) {
	cd0 := metricsTestCD0()
	_, s8 := ComputeSymbolSpectra(cd0, 100)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkNsyncSink = HardSync(&s8)
	}
}
