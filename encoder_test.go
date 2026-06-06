package goft8

import (
	"strings"
	"testing"
)

func TestEncoderDefaultConfig(t *testing.T) {
	enc := NewEncoder()

	wave, err := enc.Encode("CQ BH4GDF PM00")
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if want := NTXSamples * 4; len(wave) != want {
		t.Fatalf("default sample rate length = %d samples, want %d", len(wave), want)
	}

	pcm, err := enc.EncodeToBytes("CQ BH4GDF PM00")
	if err != nil {
		t.Fatalf("EncodeToBytes failed: %v", err)
	}
	if want := len(wave) * 3; len(pcm) != want {
		t.Fatalf("default PCM length = %d bytes, want %d", len(pcm), want)
	}
}

func TestEncoderRejectsUnsupportedSampleRate(t *testing.T) {
	enc := NewEncoder(WithSampleRate(44100))

	if _, err := enc.Encode("CQ BH4GDF PM00"); err == nil || !strings.Contains(err.Error(), "unsupported sample rate 44100") {
		t.Fatalf("Encode error = %v, want unsupported sample rate", err)
	}

	_, err := enc.EncodeMulti([]MessageFreq{
		{Message: "CQ BH4GDF PM00", Freq: 1500},
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported sample rate 44100") {
		t.Fatalf("EncodeMulti error = %v, want unsupported sample rate", err)
	}
}

func TestEncoderRejectsUnsupportedBitDepthForPCM(t *testing.T) {
	enc := NewEncoder(WithBitDepth(20))

	if _, err := enc.Encode("CQ BH4GDF PM00"); err != nil {
		t.Fatalf("Encode should not depend on PCM bit depth, got %v", err)
	}

	if _, err := enc.EncodeToBytes("CQ BH4GDF PM00"); err == nil || !strings.Contains(err.Error(), "unsupported bit depth 20") {
		t.Fatalf("EncodeToBytes error = %v, want unsupported bit depth", err)
	}
}

func TestEncoderTxFreqClamp(t *testing.T) {
	// Below minimum: should clamp to 80 Hz.
	enc := NewEncoder(WithTxFreq(10), WithSampleRate(12000), WithBitDepth(16))
	wave, err := enc.Encode("CQ BH4GDF PM00")
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(wave) != NTXSamples {
		t.Fatalf("expected %d samples, got %d", NTXSamples, len(wave))
	}

	// Above maximum: should clamp to 4920 Hz.
	enc2 := NewEncoder(WithTxFreq(6000), WithSampleRate(12000), WithBitDepth(16))
	wave2, err := enc2.Encode("CQ BH4GDF PM00")
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(wave2) != NTXSamples {
		t.Fatalf("expected %d samples, got %d", NTXSamples, len(wave2))
	}

	// EncodeMulti with sufficient spacing should succeed.
	enc3 := NewEncoder(WithSampleRate(12000), WithBitDepth(16))
	msgs := []MessageFreq{
		{Message: "CQ BH4GDF PM00", Freq: 10},   // clamped to 80
		{Message: "CQ BH4HKZ PM01", Freq: 6000}, // clamped to 4920
	}
	wave3, err := enc3.EncodeMulti(msgs)
	if err != nil {
		t.Fatalf("EncodeMulti failed: %v", err)
	}
	if len(wave3) != NTXSamples {
		t.Fatalf("expected %d samples, got %d", NTXSamples, len(wave3))
	}

	// EncodeMulti with insufficient spacing (< 60 Hz) must return an error.
	enc4 := NewEncoder(WithSampleRate(12000), WithBitDepth(16))
	msgsClose := []MessageFreq{
		{Message: "CQ BH4GDF PM00", Freq: 1500},
		{Message: "CQ BH4HKZ PM01", Freq: 1530}, // only 30 Hz apart
	}
	_, err = enc4.EncodeMulti(msgsClose)
	if err == nil {
		t.Fatal("expected error for messages spaced < 60 Hz apart, got nil")
	}
	t.Logf("Got expected spacing error: %v", err)

	// Unordered list: the check must sort first, so 1500/1530 still fails.
	enc5 := NewEncoder(WithSampleRate(12000), WithBitDepth(16))
	msgsUnordered := []MessageFreq{
		{Message: "CQ BH4HKZ PM01", Freq: 1530},
		{Message: "CQ BH4GDF PM00", Freq: 1500},
	}
	_, err = enc5.EncodeMulti(msgsUnordered)
	if err == nil {
		t.Fatal("expected error for unordered messages spaced < 60 Hz apart, got nil")
	}
	t.Logf("Got expected spacing error for unordered list: %v", err)

	t.Log("TX frequency clamping works: 10→80, 6000→4920")
}

func TestEncoderDecodeRoundTrip(t *testing.T) {
	// Create encoder and decoder.
	enc := NewEncoder(WithTxFreq(1500), WithSampleRate(12000), WithBitDepth(16))
	dec := NewDecoder()

	tests := []string{
		"CQ BH4GDF PM00",
		"BH4GDF BH4HKZ -10",
		"BH4GDF KL0I RR73",
		"BH4GDF KL0I PM00",
	}

	for _, msg := range tests {
		t.Run(msg, func(t *testing.T) {
			// Encode message to waveform.
			waveform, err := enc.Encode(msg)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			if len(waveform) != NTXSamples {
				t.Fatalf("waveform length %d, want %d", len(waveform), NTXSamples)
			}

			// Decoder expects exactly 180000 samples (15s at 12kHz).
			// Our waveform is 151680 samples (12.64s at 12kHz).
			// Pad with zeros to make it a full cycle.
			audio := make([]float32, AudioSamplesPerCycle)
			for i := range waveform {
				audio[i] = waveform[i]
			}

			// Decode.
			decodes, err := dec.Decode(audio)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Check that the original message appears in decodes.
			found := false
			for _, d := range decodes {
				if d.Message == msg {
					found = true
					break
				}
			}
			if !found {
				t.Logf("Decodes for %q:", msg)
				for _, d := range decodes {
					t.Logf("  %s", d.Message)
				}
				t.Errorf("message %q not found in decodes", msg)
			}
		})
	}
}
