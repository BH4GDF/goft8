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
