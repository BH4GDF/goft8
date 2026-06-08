package goft8

import (
	"path/filepath"
	"testing"
)

func TestDefaultDecodeParamsAndDecoderOptions(t *testing.T) {
	params := DefaultDecodeParams()
	if params.Depth != 2 || params.APWidth != 25 || params.MaxPasses != 5 {
		t.Fatalf("DefaultDecodeParams = %+v", params)
	}

	logger := testLogger{}
	dec := NewDecoder(
		WithMyCall("K1ABC"),
		WithDxCall("W9XYZ"),
		WithMaxPasses(1),
		WithAudioStartSeconds(1.25),
		WithLogger(logger),
		WithWorkers(2),
	)
	if dec.cfg.myCall != "K1ABC" || dec.cfg.dxCall != "W9XYZ" || !dec.cfg.apEnabled {
		t.Fatalf("decoder AP/callsign config = %+v", dec.cfg)
	}
	if dec.cfg.maxPasses != 1 || dec.cfg.audioStartSeconds != 1.25 || dec.cfg.workers != 2 {
		t.Fatalf("decoder option config = %+v", dec.cfg)
	}
	if dec.cfg.logger == nil {
		t.Fatal("WithLogger did not set logger")
	}

	dec.Reset()
	if dec.cfg.myCall != "K1ABC" || dec.cfg.workers != 2 {
		t.Fatalf("Reset changed configuration: %+v", dec.cfg)
	}
}

func TestDecoderRejectsWrongLengthWithOptions(t *testing.T) {
	dec := NewDecoder(WithWorkers(2), WithMaxPasses(1))

	if _, err := dec.Decode(make([]float32, AudioSamplesPerCycle-1)); err == nil {
		t.Fatal("Decode accepted wrong length audio")
	}
	if _, err := dec.DecodeStream(make([]float32, AudioSamplesPerCycle-1), func(Decoded) {}); err == nil {
		t.Fatal("DecodeStream accepted wrong length audio")
	}
}

func TestWriteWAVMono12kRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mono12k.wav")
	want := []float32{-1, -0.5, 0, 0.5, 1}

	if err := WriteWAVMono12k(path, want); err != nil {
		t.Fatalf("WriteWAVMono12k: %v", err)
	}
	got, format, err := ReadWAVMono(path)
	if err != nil {
		t.Fatalf("ReadWAVMono: %v", err)
	}
	if format.SampleRate != AudioSampleRate || format.BitDepth != 16 || format.PCMFormat != 1 || format.Channels != 1 {
		t.Fatalf("format = %+v", format)
	}
	assertFloat32SliceClose(t, got, []float32{-32767.0 / 32768.0, -0.5, 0, 0.5, 32767.0 / 32768.0}, 1e-5)
}

func TestSync8FindCandidatesAndDecodeInterval(t *testing.T) {
	audio := encodeTestAudio(t, "CQ BH4GDF PM00", 1500)

	cands := Sync8FindCandidates(audio, 1000, 2000, 5, 0, 5, 1)
	if len(cands) == 0 {
		t.Fatal("Sync8FindCandidates returned no candidates")
	}
	if len(cands) > 5 {
		t.Fatalf("candidate count = %d, want <= 5", len(cands))
	}
	foundNear := false
	for _, c := range cands {
		if c.Freq > 1400 && c.Freq < 1600 {
			foundNear = true
			break
		}
	}
	if !foundNear {
		t.Fatalf("no candidate near 1500 Hz: %#v", cands)
	}

	params := DecodeParams{Depth: DepthFast, MaxPasses: 1}
	if got, state := DecodeInterval(audio, 0, nil, params, 1000, 2000); got != nil || state != nil {
		t.Fatalf("invalid interval returned got=%#v state=%#v", got, state)
	}
	if got, state := DecodeInterval(audio, 2, nil, params, 1000, 2000); got != nil || state != nil {
		t.Fatalf("interval 2 without state returned got=%#v state=%#v", got, state)
	}

	results, state := DecodeInterval(audio, 1, nil, params, 1000, 2000)
	if state == nil {
		t.Fatal("interval 1 returned nil state")
	}
	if len(results) == 0 {
		t.Fatal("DecodeInterval interval 1 returned no results")
	}
	if got, next := DecodeInterval(audio, 2, state, params, 1000, 2000); next == nil {
		t.Fatalf("interval 2 returned nil state with %d results", len(got))
	}
	if got, next := DecodeInterval(audio, 3, state, params, 1000, 2000); next == nil {
		t.Fatalf("interval 3 returned nil state with %d results", len(got))
	}
}

func encodeTestAudio(t *testing.T, msg string, freq float64) []float32 {
	t.Helper()
	enc := NewEncoder(WithTxFreq(freq), WithSampleRate(AudioSampleRate), WithBitDepth(16))
	wave, err := enc.Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	audio := make([]float32, AudioSamplesPerCycle)
	copy(audio, wave)
	return audio
}

type testLogger struct{}

func (testLogger) Printf(string, ...any) {}
