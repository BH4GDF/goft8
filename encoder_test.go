package goft8

import (
	"testing"
)

func TestEncoderDecodeRoundTrip(t *testing.T) {
	// Create encoder and decoder.
	enc := NewEncoder(WithTxFreq(1500))
	dec := NewDecoder()

	tests := []string{
		"CQ BH4GDF PM00",
		"BH4GDF BH4ABC -10",
		"BH4GDF BH4ABC RR73",
		"W1ABC K1ABC FN20",
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
