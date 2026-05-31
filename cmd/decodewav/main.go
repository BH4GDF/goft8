// decodewav is a command-line tool that decodes FT8 messages from a WAV file.
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"

	"github.com/bh4gdf/goft8"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <wavfile>\n", os.Args[0])
		os.Exit(1)
	}

	path := os.Args[1]

	// Read WAV parameters.
	sampleRate, bitDepth, pcmFormat, err := goft8.ReadWAVParams(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read wav params failed: %v\n", err)
		os.Exit(1)
	}

	if pcmFormat != 1 && pcmFormat != 3 {
		fmt.Fprintf(os.Stderr, "unsupported PCM format %d (want 1=PCM or 3=IEEE float)\n", pcmFormat)
		os.Exit(1)
	}

	// Read raw samples.
	data, nsamp, err := readWAVData(path, bitDepth, pcmFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read wav data failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("File: %s\n", path)
	fmt.Printf("Format: %d Hz, %d-bit, %s\n", sampleRate, bitDepth,
		map[int]string{1: "PCM", 3: "float"}[pcmFormat])
	fmt.Printf("Samples: %d (%.3f s @ %d Hz)\n", nsamp, float64(nsamp)/float64(sampleRate), sampleRate)

	// Downsample 48 kHz → 12 kHz if needed.
	if sampleRate == 48000 {
		data = downsample4x(data)
		nsamp = len(data)
		sampleRate = 12000
		fmt.Printf("Downsampled to %d samples @ %d Hz\n", nsamp, sampleRate)
	}

	// Pad or truncate to exactly AudioSamplesPerCycle (180000 @ 12 kHz).
	want := goft8.AudioSamplesPerCycle
	samples := make([]float32, want)
	n := nsamp
	if n > want {
		n = want
	}
	copy(samples, data[:n])

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

// readWAVData reads the PCM samples from a WAV file after the fmt chunk.
func readWAVData(path string, bitDepth, pcmFormat int) ([]float32, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	// Skip RIFF header.
	var riff [12]byte
	f.Read(riff[:])

	// Walk chunks to find data.
	for {
		var hdr [8]byte
		if _, err := f.Read(hdr[:]); err != nil {
			return nil, 0, err
		}
		chunkID := string(hdr[0:4])
		chunkSize := binary.LittleEndian.Uint32(hdr[4:8])

		if chunkID == "data" {
			return parseSamples(f, int(chunkSize), bitDepth, pcmFormat)
		}

		f.Seek(int64(chunkSize), 1)
		if chunkSize%2 == 1 {
			f.Seek(1, 1)
		}
	}
}

func parseSamples(r *os.File, size, bitDepth, pcmFormat int) ([]float32, int, error) {
	buf := make([]byte, size)
	if _, err := r.Read(buf); err != nil {
		return nil, 0, err
	}

	if pcmFormat == 3 { // IEEE float
		if bitDepth == 32 {
			n := size / 4
			out := make([]float32, n)
			for i := 0; i < n; i++ {
				out[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
			}
			return out, n, nil
		}
		return nil, 0, fmt.Errorf("unsupported float bit depth %d", bitDepth)
	}

	// Integer PCM
	switch bitDepth {
	case 16:
		n := size / 2
		out := make([]float32, n)
		for i := 0; i < n; i++ {
			out[i] = float32(int16(binary.LittleEndian.Uint16(buf[i*2:]))) / 32768.0
		}
		return out, n, nil
	case 24:
		n := size / 3
		out := make([]float32, n)
		for i := 0; i < n; i++ {
			b0 := int32(buf[i*3])
			b1 := int32(buf[i*3+1])
			b2 := int32(buf[i*3+2])
			s := (b0 & 0xFF) | ((b1 & 0xFF) << 8) | ((b2 & 0xFF) << 16)
			if s&0x800000 != 0 {
				s |= ^0x7FFFFF
			}
			out[i] = float32(s) / 8388608.0
		}
		return out, n, nil
	case 32:
		n := size / 4
		out := make([]float32, n)
		for i := 0; i < n; i++ {
			out[i] = float32(int32(binary.LittleEndian.Uint32(buf[i*4:]))) / 2147483648.0
		}
		return out, n, nil
	default:
		return nil, 0, fmt.Errorf("unsupported bit depth %d", bitDepth)
	}
}

// downsample4x converts 48 kHz samples to 12 kHz by simple 4:1 averaging.
func downsample4x(in []float32) []float32 {
	if len(in)%4 != 0 {
		in = in[:len(in)/4*4]
	}
	out := make([]float32, len(in)/4)
	for i := 0; i < len(out); i++ {
		j := i * 4
		out[i] = (in[j] + in[j+1] + in[j+2] + in[j+3]) * 0.25
	}
	return out
}
