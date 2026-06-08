// decodewav is a command-line tool that decodes FT8 messages from a WAV file.
package main

import (
	"encoding/json"
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
	prog := "decodewav"
	if len(args) > 0 {
		prog = args[0]
	}

	fs := flag.NewFlagSet(prog, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		freqMin = fs.Int("freq-min", 100, "Minimum decode frequency in Hz")
		freqMax = fs.Int("freq-max", 3000, "Maximum decode frequency in Hz")
		depth   = fs.String("depth", "normal", "Decode depth: fast, normal, or deep")
		workers = fs.Int("workers", 0, "Worker count: 0 serial, >0 fixed, <0 auto")
		jsonOut = fs.Bool("json", false, "Write decoded messages as JSON")
	)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: %s [options] <wavfile>\n", prog)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 1
	}
	if *freqMin < 0 || *freqMax <= *freqMin {
		fmt.Fprintf(stderr, "invalid frequency range: freq-min=%d freq-max=%d\n", *freqMin, *freqMax)
		return 1
	}
	decodeDepth, err := parseDepth(*depth)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	path := fs.Arg(0)

	raw, format, err := goft8.ReadWAVMono(path)
	if err != nil {
		fmt.Fprintf(stderr, "read wav failed: %v\n", err)
		return 1
	}

	formatName := map[int]string{1: "PCM", 3: "float"}[format.PCMFormat]
	if !*jsonOut {
		fmt.Fprintf(stdout, "File: %s\n", path)
		fmt.Fprintf(stdout, "Format: %d Hz, %d-bit, %s\n", format.SampleRate, format.BitDepth, formatName)
		fmt.Fprintf(stdout, "Samples: %d (%.3f s @ %d Hz)\n", len(raw), float64(len(raw))/float64(format.SampleRate), format.SampleRate)
	}

	audio, err := goft8.ReadWAVMono12k(path)
	if err != nil {
		fmt.Fprintf(stderr, "prepare decode audio failed: %v\n", err)
		return 1
	}
	downsampled := format.SampleRate == 48000
	if downsampled && !*jsonOut {
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

	dec := goft8.NewDecoder(
		goft8.WithFreqRange(*freqMin, *freqMax),
		goft8.WithDepth(decodeDepth),
		goft8.WithWorkers(*workers),
	)
	decodes, err := dec.Decode(samples)
	if err != nil {
		fmt.Fprintf(stderr, "decode failed: %v\n", err)
		return 1
	}

	if *jsonOut {
		if err := writeJSON(stdout, path, format, formatName, len(raw), downsampled, len(audio), decodes); err != nil {
			fmt.Fprintf(stderr, "write json failed: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "\nDecoded %d message(s):\n", len(decodes))
	for _, d := range decodes {
		fmt.Fprintf(stdout, "  %-22s  |  %7.1f Hz  |  dt=%+.2f s  |  SNR=%3d dB\n",
			d.Message, d.Freq, d.DT, int(d.SNR))
	}
	return 0
}

func parseDepth(raw string) (int, error) {
	switch raw {
	case "fast":
		return goft8.DepthFast, nil
	case "normal":
		return goft8.DepthNormal, nil
	case "deep":
		return goft8.DepthDeep, nil
	default:
		return 0, fmt.Errorf("invalid depth %q (use fast, normal, or deep)", raw)
	}
}

type jsonResult struct {
	File        string        `json:"file"`
	Format      jsonFormat    `json:"format"`
	Samples     int           `json:"samples"`
	DurationSec float64       `json:"duration_sec"`
	Downsampled *jsonDownrate `json:"downsampled,omitempty"`
	Decoded     []jsonDecode  `json:"decoded"`
}

type jsonFormat struct {
	SampleRate int    `json:"sample_rate"`
	BitDepth   int    `json:"bit_depth"`
	PCMFormat  int    `json:"pcm_format"`
	Name       string `json:"name"`
	Channels   int    `json:"channels"`
}

type jsonDownrate struct {
	Samples    int `json:"samples"`
	SampleRate int `json:"sample_rate"`
}

type jsonDecode struct {
	Message     string  `json:"message"`
	Freq        float64 `json:"freq"`
	DT          float64 `json:"dt"`
	SNR         int     `json:"snr"`
	Pass        int     `json:"pass,omitempty"`
	APType      int     `json:"ap_type,omitempty"`
	NHardErrors int     `json:"n_hard_errors,omitempty"`
}

func writeJSON(stdout io.Writer, path string, format goft8.WAVFormat, formatName string, rawSamples int, downsampled bool, decodeSamples int, decodes []goft8.Decoded) error {
	result := jsonResult{
		File: path,
		Format: jsonFormat{
			SampleRate: format.SampleRate,
			BitDepth:   format.BitDepth,
			PCMFormat:  format.PCMFormat,
			Name:       formatName,
			Channels:   format.Channels,
		},
		Samples:     rawSamples,
		DurationSec: float64(rawSamples) / float64(format.SampleRate),
		Decoded:     make([]jsonDecode, len(decodes)),
	}
	if downsampled {
		result.Downsampled = &jsonDownrate{
			Samples:    decodeSamples,
			SampleRate: goft8.AudioSampleRate,
		}
	}
	for i, d := range decodes {
		result.Decoded[i] = jsonDecode{
			Message:     d.Message,
			Freq:        d.Freq,
			DT:          d.DT,
			SNR:         d.SNR,
			Pass:        d.Pass,
			APType:      d.APType,
			NHardErrors: d.NHardErrors,
		}
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
