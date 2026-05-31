package goft8

import (
	"testing"
)

func TestEncoderMultiFDMA(t *testing.T) {
	msgs := []MessageFreq{
		{Message: "CQ BH4GDF PM00", Freq: 1200},
		{Message: "BH4GDF BH4HKZ -10", Freq: 1800},
	}

	enc := NewEncoder(WithSampleRate(12000), WithBitDepth(16))
	waveform, err := enc.EncodeMulti(msgs)
	if err != nil {
		t.Fatalf("EncodeMulti failed: %v", err)
	}
	if len(waveform) != NTXSamples {
		t.Fatalf("waveform length %d, want %d", len(waveform), NTXSamples)
	}

	// Scale to avoid clipping when writing to WAV or decoding.
	scale := float32(len(msgs))
	for i := range waveform {
		waveform[i] /= scale
	}

	// Pad to full 15-second cycle expected by the decoder.
	audio := make([]float32, AudioSamplesPerCycle)
	copy(audio, waveform)

	// Decode.
	dec := NewDecoder()
	decodes, err := dec.Decode(audio)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify both messages are decoded.
	found := make(map[string]bool)
	for _, d := range decodes {
		found[d.Message] = true
	}

	for _, mf := range msgs {
		if !found[mf.Message] {
			t.Logf("Decodes:")
			for _, d := range decodes {
				t.Logf("  f=%.1f  %s", d.Freq, d.Message)
			}
			t.Errorf("message %q not found in decodes", mf.Message)
		}
	}
}

func TestEncoderMultiEmpty(t *testing.T) {
	enc := NewEncoder(WithSampleRate(12000), WithBitDepth(16))
	_, err := enc.EncodeMulti(nil)
	if err == nil {
		t.Fatal("expected error for empty message list")
	}
}
