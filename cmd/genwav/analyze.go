//go:build ignore

package main

import (
	"fmt"
	"math"
	"os"

	"gonum.org/v1/gonum/dsp/fourier"
)

func main() {
	data, err := os.ReadFile("ft8_multi.wav")
	if err != nil {
		fmt.Fprintf(os.Stderr, "read failed: %v\n", err)
		os.Exit(1)
	}
	if len(data) < 44 {
		fmt.Fprintf(os.Stderr, "file too short\n")
		os.Exit(1)
	}
	if (len(data)-44)%2 != 0 {
		fmt.Fprintf(os.Stderr, "PCM16 data is not sample-aligned\n")
		os.Exit(1)
	}
	samples := make([]float64, (len(data)-44)/2)
	for i := range samples {
		val := int16(data[44+i*2]) | int16(data[44+i*2+1])<<8
		samples[i] = float64(val) / 32767.0
	}
	fs := 12000.0

	// Use a Hann-windowed FFT for a cleaner spectrum.
	nfft := 65536
	if len(samples) < nfft {
		tmp := make([]float64, nfft)
		copy(tmp, samples)
		samples = tmp
	}

	// Apply Hann window.
	win := make([]float64, nfft)
	for i := range win {
		win[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(nfft-1)))
	}
	for i := range samples {
		if i < nfft {
			samples[i] *= win[i]
		}
	}

	fft := fourier.NewFFT(nfft)
	spec := fft.Coefficients(nil, samples[:nfft])
	binWidth := fs / float64(nfft)

	fmt.Println("=== Hann-windowed spectrum (65536 pts, ~0.18 Hz/bin) ===")
	fmt.Println("Freq(Hz)  dB(rel to peak)")
	peak := 0.0
	for _, c := range spec {
		p := real(c)*real(c) + imag(c)*imag(c)
		if p > peak {
			peak = p
		}
	}
	peakDB := 10 * math.Log10(peak+1e-20)

	// Print spectrum around the two carriers and their ±100 Hz offsets.
	for _, center := range []float64{1500, 1600} {
		fmt.Printf("\n--- Around %.0f Hz ---\n", center)
		for f := center - 200; f <= center+200; f += 10 {
			bin := int(math.Round(f / binWidth))
			if bin < 0 || bin >= len(spec) {
				continue
			}
			p := real(spec[bin])*real(spec[bin]) + imag(spec[bin])*imag(spec[bin])
			dB := 10*math.Log10(p+1e-20) - peakDB
			marker := ""
			if math.Abs(f-center) < 1 {
				marker = "  <-- carrier"
			} else if math.Abs(f-(center-100)) < 1 || math.Abs(f-(center+100)) < 1 {
				marker = "  <-- ±100 Hz"
			}
			fmt.Printf("%.0f  %6.1f dB%s\n", f, dB, marker)
		}
	}
}
