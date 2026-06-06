// decodewav is a command-line tool that decodes FT8 messages from a WAV file.
package main

import (
	"fmt"
	"os"

	"github.com/bh4gdf/goft8"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <wavfile>\n", os.Args[0])
		os.Exit(1)
	}

	path := os.Args[1]

	raw, format, err := goft8.ReadWAVMono(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read wav failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("File: %s\n", path)
	fmt.Printf("Format: %d Hz, %d-bit, %s\n", format.SampleRate, format.BitDepth,
		map[int]string{1: "PCM", 3: "float"}[format.PCMFormat])
	fmt.Printf("Samples: %d (%.3f s @ %d Hz)\n", len(raw), float64(len(raw))/float64(format.SampleRate), format.SampleRate)

	audio, err := goft8.ReadWAVMono12k(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare decode audio failed: %v\n", err)
		os.Exit(1)
	}
	if format.SampleRate == 48000 {
		fmt.Printf("Downsampled to %d samples @ %d Hz\n", len(audio), goft8.AudioSampleRate)
	}

	// Pad or truncate to exactly AudioSamplesPerCycle (180000 @ 12 kHz).
	want := goft8.AudioSamplesPerCycle
	samples := make([]float32, want)
	n := len(audio)
	if n > want {
		n = want
	}
	copy(samples, audio[:n])

	dec := goft8.NewDecoder()
	decodes, err := dec.Decode(samples)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nDecoded %d message(s):\n", len(decodes))
	for _, d := range decodes {
		fmt.Printf("  %-22s  |  %7.1f Hz  |  dt=%+.2f s  |  SNR=%3d dB\n",
			d.Message, d.Freq, d.DT, int(d.SNR))
	}
}
