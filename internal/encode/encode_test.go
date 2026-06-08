package encode

import (
	"math"
	"testing"

	"github.com/bh4gdf/goft8/internal/protocol"
	ft8params "github.com/bh4gdf/goft8/params"
)

func TestEncodeParamsSupportedRates(t *testing.T) {
	tests := []struct {
		rate      int
		wantNSPS  int
		wantWave  int
		wantDelta float64
	}{
		{rate: 12000, wantNSPS: ft8params.NSPS, wantWave: ft8params.NN * ft8params.NSPS, wantDelta: 1.0 / 12000},
		{rate: 48000, wantNSPS: 4 * ft8params.NSPS, wantWave: ft8params.NN * 4 * ft8params.NSPS, wantDelta: 1.0 / 48000},
	}

	for _, tc := range tests {
		nsps, nwave, dt := EncodeParams(tc.rate)
		if nsps != tc.wantNSPS || nwave != tc.wantWave || math.Abs(dt-tc.wantDelta) > 1e-15 {
			t.Fatalf("EncodeParams(%d) = (%d, %d, %.18f), want (%d, %d, %.18f)",
				tc.rate, nsps, nwave, dt, tc.wantNSPS, tc.wantWave, tc.wantDelta)
		}
	}
}

func TestGenFT8TonesPlacesCostasBlocks(t *testing.T) {
	bits, _, _, ok := protocol.Pack77("CQ BH4GDF PM00")
	if !ok {
		t.Fatal("Pack77 failed")
	}
	tones := GenFT8Tones(bits)

	if len(tones) != ft8params.NN {
		t.Fatalf("len(tones) = %d, want %d", len(tones), ft8params.NN)
	}
	for i, want := range ft8params.Icos7 {
		if tones[i] != want {
			t.Fatalf("first Costas tone %d = %d, want %d", i, tones[i], want)
		}
		if tones[36+i] != want {
			t.Fatalf("middle Costas tone %d = %d, want %d", i, tones[36+i], want)
		}
		if tones[ft8params.NN-7+i] != want {
			t.Fatalf("last Costas tone %d = %d, want %d", i, tones[ft8params.NN-7+i], want)
		}
	}
	for i, tone := range tones {
		if tone < 0 || tone > 7 {
			t.Fatalf("tone %d = %d, want 0..7", i, tone)
		}
	}
}

func TestGenFT8WaveSupportedRates(t *testing.T) {
	bits, _, _, ok := protocol.Pack77("CQ BH4GDF PM00")
	if !ok {
		t.Fatal("Pack77 failed")
	}
	tones := GenFT8Tones(bits)

	for _, rate := range []int{12000, 48000} {
		_, wantLen, _ := EncodeParams(rate)
		wave := GenFT8WaveSR(tones, 1500, rate)
		if len(wave) != wantLen {
			t.Fatalf("GenFT8WaveSR rate %d length = %d, want %d", rate, len(wave), wantLen)
		}
		var energy float64
		for _, v := range wave {
			energy += float64(v) * float64(v)
		}
		if energy == 0 {
			t.Fatalf("GenFT8WaveSR rate %d produced all-zero waveform", rate)
		}
	}
}
