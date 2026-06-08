//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/bh4gdf/goft8"
)

func main() {
	// Read the generated WAV file.
	data, err := os.ReadFile("ft8_multi.wav")
	if err != nil {
		fmt.Fprintf(os.Stderr, "read failed: %v\n", err)
		os.Exit(1)
	}

	// Skip RIFF header (44 bytes for standard 16-bit PCM mono).
	if len(data) < 44 {
		fmt.Fprintf(os.Stderr, "file too short\n")
		os.Exit(1)
	}
	if (len(data)-44)%2 != 0 {
		fmt.Fprintf(os.Stderr, "PCM16 data is not sample-aligned\n")
		os.Exit(1)
	}

	// Extract raw int16 samples and convert to float32.
	rawSamples := make([]float32, (len(data)-44)/2)
	for i := range rawSamples {
		val := int16(data[44+i*2]) | int16(data[44+i*2+1])<<8
		rawSamples[i] = float32(val) / 32767.0
	}

	// Pad to exactly 180000 samples (15 s @ 12 kHz) as required by the decoder.
	samples := make([]float32, goft8.AudioSamplesPerCycle)
	copy(samples, rawSamples)

	// Decode.
	dec := goft8.NewDecoder()
	decodes, err := dec.Decode(samples)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Decoded messages:")
	for _, d := range decodes {
		fmt.Printf("  %s @ %.1f Hz, dt=%.2f s, SNR=%.0f dB\n", d.Message, d.Freq, d.DT, d.SNR)
	}
}
