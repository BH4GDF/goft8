package decode

import (
	"testing"

	ft8params "github.com/bh4gdf/goft8/params"
)

func TestSync8WorkspaceSmoke(t *testing.T) {
	var dd [ft8params.NMAX]float32
	for i := range dd {
		dd[i] = float32((i%31)-15) / 31
	}

	spec := ComputeSpectrogramForTest(dd[:], ft8params.NMAX, 1)
	if spec == nil || len(spec.S) == 0 || len(spec.Savg) == 0 || spec.backing == nil {
		t.Fatalf("ComputeSpectrogramForTest returned incomplete workspace: %#v", spec)
	}
	if got := len(spec.S[1]); got != ft8params.NHSYM+1 {
		t.Fatalf("spectrogram columns = %d, want %d", got, ft8params.NHSYM+1)
	}

	df := ft8params.Fs / float64(ft8params.NFFT1)
	nssy := ft8params.NSPS / ft8params.NSTEP
	nfos := ft8params.NFFT1 / ft8params.NSPS
	tstep := float64(ft8params.NSTEP) / ft8params.Fs
	jstrt := int(0.5 / tstep)
	sync2d := ComputeSync2DForTest(spec, 200, 2600, df, nssy, nfos, jstrt, 1)
	if sync2d == nil {
		t.Fatal("ComputeSync2DForTest returned nil")
	}
	if got := len(sync2d[1]); got != 2*jz+1 {
		t.Fatalf("sync2d lag columns = %d, want %d", got, 2*jz+1)
	}

	releaseSpectrogram(spec)
	if spec.backing != nil || spec.Savg != nil {
		t.Fatal("releaseSpectrogram did not clear pooled fields")
	}
	sync2dPool.Put(sync2d)
}
