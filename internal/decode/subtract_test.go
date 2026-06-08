package decode

import (
	"math"
	"testing"

	"github.com/bh4gdf/goft8/internal/encode"
	"github.com/bh4gdf/goft8/internal/protocol"
	ft8params "github.com/bh4gdf/goft8/params"
)

func TestSubtractFT8ChangesMatchingSignal(t *testing.T) {
	msg := "CQ BH4GDF PM00"
	bits, _, _, ok := protocol.Pack77(msg)
	if !ok {
		t.Fatalf("Pack77(%q) failed", msg)
	}
	tones := encode.GenFT8Tones(bits)
	audio := makeSubtractAudio(tones, 1500, 0.5)
	before := energy(audio)

	SubtractFT8(audio, tones, 1500, 0.5)

	after := energy(audio)
	if after >= before {
		t.Fatalf("energy after subtract = %g, want less than before %g", after, before)
	}
	if after == 0 {
		t.Fatal("subtract unexpectedly zeroed all audio")
	}
}

func TestBatchSubtractFT8EmptyAndSingleMatchStandalone(t *testing.T) {
	msg := "CQ BH4GDF PM00"
	bits, _, _, ok := protocol.Pack77(msg)
	if !ok {
		t.Fatalf("Pack77(%q) failed", msg)
	}
	tones := encode.GenFT8Tones(bits)
	base := makeSubtractAudio(tones, 1500, 0.5)

	empty := append([]float32(nil), base...)
	BatchSubtractFT8(empty, nil)
	assertFloat32ApproxSlice(t, empty, base, 0)

	standalone := append([]float32(nil), base...)
	SubtractFT8(standalone, tones, 1500, 0.5)

	batched := append([]float32(nil), base...)
	BatchSubtractFT8(batched, []SubtractSignal{{Tones: tones, Freq: 1500, DT: 0.5}})
	assertFloat32ApproxSlice(t, batched, standalone, 1e-6)
}

func TestBatchSubtractFT8MultipleSignals(t *testing.T) {
	msgA := "CQ BH4GDF PM00"
	msgB := "KL0I BH4GDF +20"
	bitsA, _, _, ok := protocol.Pack77(msgA)
	if !ok {
		t.Fatalf("Pack77(%q) failed", msgA)
	}
	bitsB, _, _, ok := protocol.Pack77(msgB)
	if !ok {
		t.Fatalf("Pack77(%q) failed", msgB)
	}
	tonesA := encode.GenFT8Tones(bitsA)
	tonesB := encode.GenFT8Tones(bitsB)
	audio := makeSubtractAudio(tonesA, 1500, 0.5)
	addSubtractAudio(audio, tonesB, 1600, 0.5)
	before := energy(audio)

	BatchSubtractFT8(audio, []SubtractSignal{
		{Tones: tonesA, Freq: 1500, DT: 0.5},
		{Tones: tonesB, Freq: 1600, DT: 0.5},
	})

	after := energy(audio)
	if after >= before {
		t.Fatalf("energy after batch subtract = %g, want less than before %g", after, before)
	}
}

func makeSubtractAudio(tones [ft8params.NN]int, freq, dt float64) []float32 {
	audio := make([]float32, ft8params.NMAX)
	addSubtractAudio(audio, tones, freq, dt)
	return audio
}

func addSubtractAudio(audio []float32, tones [ft8params.NN]int, freq, dt float64) {
	wave := encode.GenFT8WaveSR(tones, freq, ft8params.Fs)
	start := int(dt*ft8params.Fs) + 1 - 1
	for i, v := range wave {
		j := start + i
		if j >= 0 && j < len(audio) {
			audio[j] += v
		}
	}
}

func energy(x []float32) float64 {
	var e float64
	for _, v := range x {
		e += float64(v) * float64(v)
	}
	return e
}

func assertFloat32ApproxSlice(t *testing.T, got, want []float32, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if math.Abs(float64(got[i]-want[i])) > tol {
			t.Fatalf("value[%d] = %g, want %g", i, got[i], want[i])
		}
	}
}
