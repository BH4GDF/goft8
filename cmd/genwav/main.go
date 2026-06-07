// genwav is a command-line tool that generates FT8 WAV files.
package main

import (
	"flag"
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
	prog := "genwav"
	if len(args) > 0 {
		prog = args[0]
	}

	fs := flag.NewFlagSet(prog, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		rate = fs.Int("rate", 48000, "Sample rate: 12000 or 48000")
		bits = fs.Int("bits", 24, "Bit depth: 16, 24, or 32")
	)
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}

	if *rate != 12000 && *rate != 48000 {
		fmt.Fprintf(stderr, "unsupported sample rate %d (use 12000 or 48000)\n", *rate)
		return 1
	}
	if *bits != 16 && *bits != 24 && *bits != 32 {
		fmt.Fprintf(stderr, "unsupported bit depth %d (use 16, 24, or 32)\n", *bits)
		return 1
	}

	// Generate two FT8 signals on different frequencies.
	enc := goft8.NewEncoder(
		goft8.WithSampleRate(*rate),
		goft8.WithBitDepth(*bits),
	)
	msgs := []goft8.MessageFreq{
		{Message: "CQ BH4GDF PM00", Freq: 1500},
		{Message: "KL0I BH4GDF +20", Freq: 1600},
	}
	waveform, err := enc.EncodeMulti(msgs)
	if err != nil {
		fmt.Fprintf(stderr, "encode failed: %v\n", err)
		return 1
	}

	// Write as mono WAV at the selected sample rate and bit depth.
	fname := fmt.Sprintf("ft8_multi_%dhz_%dbit.wav", *rate, *bits)
	if err := goft8.WriteWAV(fname, waveform, *rate, *bits); err != nil {
		fmt.Fprintf(stderr, "write wav failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s written (%d Hz, %d-bit PCM, %d samples)\n",
		fname, *rate, *bits, len(waveform))
	return 0
}
