package decode

import (
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
