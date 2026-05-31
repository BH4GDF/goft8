//go:build ignore

package main

import (
	"fmt"
	"math"

	"github.com/bh4gdf/goft8"
	"github.com/bh4gdf/goft8/internal/encode"
	"github.com/bh4gdf/goft8/internal/protocol"
	"gonum.org/v1/gonum/dsp/fourier"
)

func main() {
	for _, msg := range []goft8.MessageFreq{
		{Message: "CQ BH4GDF PM00", Freq: 1500},
		{Message: "KL0I BH4GDF +20", Freq: 1600},
	} {
		bits, _, _, ok := protocol.Pack77(msg.Message)
		if !ok {
			panic("pack failed")
		}
		itone := encode.GenFT8Tones(bits)
		wave := encode.GenFT8Wave(itone, msg.Freq)

		nfft := 65536
		samples := make([]float64, nfft)
		for i := range wave {
			if i < nfft {
				samples[i] = float64(wave[i])
			}
		}

		// Hann window.
		for i := range samples {
			samples[i] *= 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(nfft-1)))
		}

		fft := fourier.NewFFT(nfft)
		spec := fft.Coefficients(nil, samples)
		binWidth := 12000.0 / float64(nfft)

		fmt.Printf("\n=== %s @ %.0f Hz ===\n", msg.Message, msg.Freq)
		peak := 0.0
		for _, c := range spec {
			p := real(c)*real(c) + imag(c)*imag(c)
			if p > peak {
				peak = p
			}
		}
		peakDB := 10 * math.Log10(peak+1e-20)

		for f := msg.Freq - 200; f <= msg.Freq+200; f += 10 {
			bin := int(math.Round(f / binWidth))
			p := real(spec[bin])*real(spec[bin]) + imag(spec[bin])*imag(spec[bin])
			dB := 10*math.Log10(p+1e-20) - peakDB
			marker := ""
			if math.Abs(f-msg.Freq) < 1 {
				marker = "  <-- carrier"
			} else if math.Abs(f-(msg.Freq-100)) < 1 || math.Abs(f-(msg.Freq+100)) < 1 {
				marker = "  <-- ±100 Hz"
			}
			fmt.Printf("%.0f  %6.1f dB%s\n", f, dB, marker)
		}
	}
}
