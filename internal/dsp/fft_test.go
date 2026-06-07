package dsp

import (
	"math"
	"math/cmplx"
	"testing"
)

func TestFFTThenIFFTRoundTrip(t *testing.T) {
	in := []complex128{
		complex(1.0, 0.5),
		complex(-2.0, 1.25),
		complex(0.25, -0.75),
		complex(3.0, 0),
		complex(-1.5, -2.0),
		complex(0, 0.5),
	}

	spec := FFT(in)
	got := IFFT(spec)
	if len(got) != len(in) {
		t.Fatalf("roundtrip length = %d, want %d", len(got), len(in))
	}
	for i := range got {
		if cmplx.Abs(got[i]-in[i]) > 1e-12 {
			t.Fatalf("roundtrip[%d] = %v, want %v", i, got[i], in[i])
		}
	}
}

func TestFFTIntoUsesProvidedDestination(t *testing.T) {
	in := []complex128{1, 2, 3, 4, 5, 6}
	dst := make([]complex128, len(in))

	got := FFTInto(dst, in)

	if len(got) != len(in) {
		t.Fatalf("len = %d, want %d", len(got), len(in))
	}
	if len(got) > 0 && &got[0] != &dst[0] {
		t.Fatal("FFTInto did not return the provided destination")
	}
	want := FFT(in)
	for i := range got {
		if cmplx.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("bin %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestIFFTIntoUsesProvidedDestination(t *testing.T) {
	in := []complex128{
		complex(1.0, 0.5),
		complex(-2.0, 1.25),
		complex(0.25, -0.75),
		complex(3.0, 0),
		complex(-1.5, -2.0),
		complex(0, 0.5),
	}
	spec := FFT(in)
	dst := make([]complex128, len(spec))

	IFFTInto(dst, spec)

	for i := range dst {
		if cmplx.Abs(dst[i]-in[i]) > 1e-12 {
			t.Fatalf("roundtrip[%d] = %v, want %v", i, dst[i], in[i])
		}
	}
}

func TestSmallestFactorPanicsForNonSmoothSize(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("smallestFactor(7) did not panic")
		}
	}()
	_ = smallestFactor(7)
}

func TestRealFFTZeroPadsShortInput(t *testing.T) {
	got := RealFFT([]float32{1, 0, 0, 0}, 8)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}
	for i, v := range got {
		if cmplx.Abs(v-1) > 1e-12 {
			t.Fatalf("bin %d = %v, want 1", i, v)
		}
	}
}

func TestRealFFTIntoTruncatesLongInputAndUsesDestination(t *testing.T) {
	dst := make([]complex128, 5)
	RealFFTInto(dst, []float32{1, 2, 3, 4, 5, 6, 7, 8, 99, 100}, 8)

	want := RealFFT([]float32{1, 2, 3, 4, 5, 6, 7, 8}, 8)
	for i := range dst {
		if cmplx.Abs(dst[i]-want[i]) > 1e-12 {
			t.Fatalf("bin %d = %v, want %v", i, dst[i], want[i])
		}
	}
}

func TestRealFFTBufferPools(t *testing.T) {
	buf3840 := getRealFFTBuf(3840)
	if len(buf3840) != 3840 {
		t.Fatalf("3840 buffer len = %d", len(buf3840))
	}
	putRealFFTBuf(3840, buf3840)

	buf192000 := getRealFFTBuf(192000)
	if len(buf192000) != 192000 {
		t.Fatalf("192000 buffer len = %d", len(buf192000))
	}
	putRealFFTBuf(192000, buf192000)

	buf := getRealFFTBuf(7)
	if len(buf) != 7 {
		t.Fatalf("fallback buffer len = %d", len(buf))
	}
	putRealFFTBuf(7, buf)
}

func TestFixedPoolRespectsCapacity(t *testing.T) {
	pool := newFixedPool[int](2)
	pool.put(1)
	pool.put(2)
	pool.put(3)

	seen := map[int]bool{
		pool.get(func() int { return 100 }): true,
		pool.get(func() int { return 100 }): true,
		pool.get(func() int { return 100 }): true,
	}
	if !seen[1] || !seen[2] || !seen[100] {
		t.Fatalf("pool returned unexpected values: %#v", seen)
	}
}

func TestSpectrogramFFT3840SingleTone(t *testing.T) {
	x := make([]float32, 3840)
	for i := range x {
		x[i] = float32(math.Sin(2 * math.Pi * float64(i) / 3840))
	}
	pow := SpectrogramFFT3840(x)
	if pow[0] <= 0 {
		t.Fatalf("first bin power = %g, want positive", pow[0])
	}
	for i := 1; i < 8; i++ {
		if pow[0] <= pow[i] {
			t.Fatalf("first bin power = %g, bin %d power = %g", pow[0], i+1, pow[i])
		}
	}
}

func TestSpectrogramFFT3840ZeroPadsShortInput(t *testing.T) {
	full := make([]float32, 3840)
	full[0] = 1

	short := SpectrogramFFT3840([]float32{1})
	want := SpectrogramFFT3840(full)

	for i := 0; i < 8; i++ {
		if math.Abs(short[i]-want[i]) > 1e-12 {
			t.Fatalf("bin %d = %g, want %g", i, short[i], want[i])
		}
	}
}
