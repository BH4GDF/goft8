package decode

import (
	"math"
	"testing"

	ft8params "github.com/bh4gdf/goft8/params"
)

var (
	benchmarkBaselineSink []float64
	benchmarkSbaseSink    [ft8params.NH1]float64
)

func benchmarkAudio() []float32 {
	audio := make([]float32, ft8params.NMAX)
	for i := range audio {
		t := float64(i) / ft8params.Fs
		audio[i] = float32(
			0.25*math.Sin(2*math.Pi*650*t) +
				0.18*math.Sin(2*math.Pi*1420*t+0.3) +
				0.05*math.Sin(2*math.Pi*37*t),
		)
	}
	return audio
}

func benchmarkSpectrum() []float64 {
	s := make([]float64, ft8params.NH1)
	for i := range s {
		f := float64(i) * ft8params.Fs / float64(ft8params.NFFT1)
		floor := 1.0 + 0.0015*f + 0.18*math.Sin(f/180.0)
		tone := 0.0
		if i%173 == 0 {
			tone = 18.0
		}
		s[i] = floor + tone
	}
	return s
}

func BenchmarkBaseline200To2600(b *testing.B) {
	src := benchmarkSpectrum()
	scratch := make([]float64, len(src))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(scratch, src)
		benchmarkBaselineSink = baseline(scratch, 200, 2600)
	}
}

func BenchmarkGetSpectrumBaselineSerial(b *testing.B) {
	audio := benchmarkAudio()
	benchmarkSbaseSink = getSpectrumBaseline(audio, 200, 2600, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkSbaseSink = getSpectrumBaseline(audio, 200, 2600, 1)
	}
}

func BenchmarkGetSpectrumBaselineParallel4(b *testing.B) {
	audio := benchmarkAudio()
	benchmarkSbaseSink = getSpectrumBaseline(audio, 200, 2600, 4)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkSbaseSink = getSpectrumBaseline(audio, 200, 2600, 4)
	}
}
