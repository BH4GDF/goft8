package decode

import (
	"math/cmplx"
	"testing"

	ft8params "github.com/bh4gdf/goft8/params"
)

func TestDownsamplerReleaseClearsReusableBuffers(t *testing.T) {
	audio := make([]float32, ft8params.NMAX)
	for i := range audio {
		audio[i] = float32(i%17) / 17
	}

	ds := NewDownsampler()
	newdat := true
	got := ds.Downsample(audio, &newdat, 1500)
	if newdat {
		t.Fatal("Downsample left newdat=true")
	}
	if len(got) != ft8params.NFFT2 {
		t.Fatalf("Downsample length = %d, want %d", len(got), ft8params.NFFT2)
	}
	if ds.cx == nil || ds.c1buf == nil || ds.xbuf == nil {
		t.Fatalf("Downsample did not populate reusable buffers: cx=%v c1=%v x=%v", ds.cx != nil, ds.c1buf != nil, ds.xbuf != nil)
	}

	clone := CloneFrom(ds)
	if clone.cx == nil || !clone.sharedCX {
		t.Fatal("CloneFrom did not share cx")
	}
	noNewdat := false
	clone.Downsample(audio, &noNewdat, 1510)
	clone.Release()
	if clone.cx != nil || clone.c1buf != nil || clone.xbuf != nil {
		t.Fatal("clone Release did not clear buffers")
	}
	if ds.cx == nil {
		t.Fatal("clone Release cleared source shared cx")
	}

	ds.Release()
	if ds.cx != nil || ds.c1buf != nil || ds.xbuf != nil || ds.ready {
		t.Fatal("Release did not clear source buffers")
	}
}

func TestCshiftHandlesPositiveNegativeAndEmptyShifts(t *testing.T) {
	in := []complex128{1, 2, 3, 4}

	tests := []struct {
		name  string
		shift int
		want  []complex128
	}{
		{"zero", 0, []complex128{1, 2, 3, 4}},
		{"positive", 1, []complex128{2, 3, 4, 1}},
		{"negative", -1, []complex128{4, 1, 2, 3}},
		{"wrapped", 5, []complex128{2, 3, 4, 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Cshift(in, tt.shift)
			assertComplexSlice(t, got, tt.want)

			inPlace := append([]complex128(nil), in...)
			CshiftInPlace(inPlace, tt.shift)
			assertComplexSlice(t, inPlace, tt.want)
		})
	}

	var empty []complex128
	if got := Cshift(empty, 3); len(got) != 0 {
		t.Fatalf("Cshift(empty) length = %d", len(got))
	}
	CshiftInPlace(empty, 3)
}

func TestTwkFreq1ZeroCoefficientsIsIdentity(t *testing.T) {
	in := []complex128{1, complex(0.5, -0.25), -1, complex(0, 2)}
	var coeffs [5]float64

	got := TwkFreq1(in, 200, coeffs)
	assertComplexSliceApprox(t, got, in, 1e-12)

	dst := make([]complex128, len(in))
	TwkFreq1Into(dst, in, 200, coeffs)
	assertComplexSliceApprox(t, dst, in, 1e-12)
}

func TestTwkFreq1AppliesPrimaryFrequencyCorrection(t *testing.T) {
	in := []complex128{1, 1, 1, 1}
	coeffs := [5]float64{25}

	got := TwkFreq1(in, 100, coeffs)

	for i := 1; i < len(got); i++ {
		ratio := got[i] / got[i-1]
		if cmplx.Abs(ratio-complex(0, 1)) > 1e-12 {
			t.Fatalf("step ratio[%d] = %v, want +j", i, ratio)
		}
	}
}

func assertComplexSlice(t *testing.T, got, want []complex128) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("value[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func assertComplexSliceApprox(t *testing.T, got, want []complex128, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if cmplx.Abs(got[i]-want[i]) > tol {
			t.Fatalf("value[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
