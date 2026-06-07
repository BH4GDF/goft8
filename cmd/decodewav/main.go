// decodewav is a command-line tool that decodes FT8 messages from a WAV file.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/bh4gdf/goft8"
)

func main() {
	if code := run(os.Args, os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func run(args []string, stdout, stderr io.Writer) int {
	prog := "decodewav"
	if len(args) > 0 {
		prog = args[0]
	}
	if len(args) < 2 {
		fmt.Fprintf(stderr, "usage: %s <wavfile>\n", prog)
		return 1
	}
	if len(args) > 2 {
		fmt.Fprintf(stderr, "usage: %s <wavfile>\n", prog)
		return 1
	}

	path := args[1]

	raw, format, err := goft8.ReadWAVMono(path)
	if err != nil {
		fmt.Fprintf(stderr, "read wav failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "File: %s\n", path)
	fmt.Fprintf(stdout, "Format: %d Hz, %d-bit, %s\n", format.SampleRate, format.BitDepth,
		map[int]string{1: "PCM", 3: "float"}[format.PCMFormat])
	fmt.Fprintf(stdout, "Samples: %d (%.3f s @ %d Hz)\n", len(raw), float64(len(raw))/float64(format.SampleRate), format.SampleRate)

	audio, err := goft8.ReadWAVMono12k(path)
	if err != nil {
		fmt.Fprintf(stderr, "prepare decode audio failed: %v\n", err)
		return 1
	}
	if format.SampleRate == 48000 {
		fmt.Fprintf(stdout, "Downsampled to %d samples @ %d Hz\n", len(audio), goft8.AudioSampleRate)
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
		fmt.Fprintf(stderr, "decode failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "\nDecoded %d message(s):\n", len(decodes))
	for _, d := range decodes {
		fmt.Fprintf(stdout, "  %-22s  |  %7.1f Hz  |  dt=%+.2f s  |  SNR=%3d dB\n",
			d.Message, d.Freq, d.DT, int(d.SNR))
	}
	return 0
}
