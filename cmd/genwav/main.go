// genwav is a command-line tool that generates FT8 WAV files.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bh4gdf/goft8"
)

func main() {
	var (
		rate = flag.Int("rate", 48000, "Sample rate: 12000 or 48000")
		bits = flag.Int("bits", 24, "Bit depth: 16, 24, or 32")
	)
	flag.Parse()

	if *rate != 12000 && *rate != 48000 {
		fmt.Fprintf(os.Stderr, "unsupported sample rate %d (use 12000 or 48000)\n", *rate)
		os.Exit(1)
	}
	if *bits != 16 && *bits != 24 && *bits != 32 {
		fmt.Fprintf(os.Stderr, "unsupported bit depth %d (use 16, 24, or 32)\n", *bits)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "encode failed: %v\n", err)
		os.Exit(1)
	}

	// Write as mono WAV at the selected sample rate and bit depth.
	fname := fmt.Sprintf("ft8_multi_%dhz_%dbit.wav", *rate, *bits)
	if err := goft8.WriteWAV(fname, waveform, *rate, *bits); err != nil {
		fmt.Fprintf(os.Stderr, "write wav failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s written (%d Hz, %d-bit PCM, %d samples)\n",
		fname, *rate, *bits, len(waveform))
}
