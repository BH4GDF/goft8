package goft8

import (
	"testing"
)

func TestEncode_CQ_BH4GDF_PM00_1500Hz(t *testing.T) {
	msg := "CQ BH4GDF PM00"
	freq := 1500.0

	// 1. Pack77
	bits, _, _, ok := Pack77(msg)
	if !ok {
		t.Fatalf("Pack77 failed for %q", msg)
	}
	t.Logf("Pack77 bits for %q: %v", msg, bits)

	// 2. GenFT8Tones
	tones := GenFT8Tones(bits)
	t.Logf("Tones for %q: %v", msg, tones)

	if len(tones) != NN {
		t.Fatalf("expected %d tones, got %d", NN, len(tones))
	}

	// 3. Encode waveform
	enc := NewEncoder(WithTxFreq(freq))
	waveform, err := enc.Encode(msg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(waveform) != NTXSamples {
		t.Fatalf("expected %d samples, got %d", NTXSamples, len(waveform))
	}

	// Check waveform is not all zeros
	var sum float32
	for _, v := range waveform {
		sum += v * v
	}
	if sum == 0 {
		t.Fatal("waveform is all zeros")
	}

	t.Logf("Encoded %q @ %.0f Hz: %d samples, energy=%.6f", msg, freq, len(waveform), sum)
}

func TestEncode_CQ_BH4HKZ_PM01_1560Hz(t *testing.T) {
	msg := "CQ BH4HKZ PM01"
	freq := 1560.0

	// 1. Pack77
	bits, _, _, ok := Pack77(msg)
	if !ok {
		t.Fatalf("Pack77 failed for %q", msg)
	}
	t.Logf("Pack77 bits for %q: %v", msg, bits)

	// 2. GenFT8Tones
	tones := GenFT8Tones(bits)
	t.Logf("Tones for %q: %v", msg, tones)

	if len(tones) != NN {
		t.Fatalf("expected %d tones, got %d", NN, len(tones))
	}

	// 3. Encode waveform
	enc := NewEncoder(WithTxFreq(freq))
	waveform, err := enc.Encode(msg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(waveform) != NTXSamples {
		t.Fatalf("expected %d samples, got %d", NTXSamples, len(waveform))
	}

	var sum float32
	for _, v := range waveform {
		sum += v * v
	}
	if sum == 0 {
		t.Fatal("waveform is all zeros")
	}

	t.Logf("Encoded %q @ %.0f Hz: %d samples, energy=%.6f", msg, freq, len(waveform), sum)
}

func TestEncodeMulti_TwoMessages(t *testing.T) {
	msgs := []MessageFreq{
		{Message: "CQ BH4GDF PM00", Freq: 1500},
		{Message: "CQ BH4HKZ PM01", Freq: 1560},
	}

	enc := NewEncoder()
	waveform, err := enc.EncodeMulti(msgs)
	if err != nil {
		t.Fatalf("EncodeMulti failed: %v", err)
	}
	if len(waveform) != NTXSamples {
		t.Fatalf("expected %d samples, got %d", NTXSamples, len(waveform))
	}

	var sum float32
	for _, v := range waveform {
		sum += v * v
	}
	if sum == 0 {
		t.Fatal("combined waveform is all zeros")
	}

	t.Logf("EncodeMulti: %d messages, %d samples, energy=%.6f", len(msgs), len(waveform), sum)
}
