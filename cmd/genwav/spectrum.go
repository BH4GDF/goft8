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
		panic(err)
	}
	// Skip 44-byte WAV header.
	samples := make([]float64, (len(data)-44)/2)
	for i := range samples {
		val := int16(data[44+i*2]) | int16(data[44+i*2+1])<<8
		samples[i] = float64(val) / 32767.0
	}

	fs := 12000.0
	nfft := 192000 // 16 seconds worth
	if len(samples) < nfft {
		tmp := make([]float64, nfft)
		copy(tmp, samples)
		samples = tmp
	}

	fft := fourier.NewFFT(nfft)
	spec := fft.Coefficients(nil, samples)

	// Print power spectrum around 1400-1700 Hz.
	binWidth := fs / float64(nfft)
	fmt.Printf("Bin width: %.3f Hz\n", binWidth)
	fmt.Println("Freq(Hz)  dB")
	for f := 1300.0; f <= 1800.0; f += 1.0 {
		bin := int(math.Round(f / binWidth))
		if bin < 0 || bin >= len(spec) {
			continue
		}
		p := real(spec[bin])*real(spec[bin]) + imag(spec[bin])*imag(spec[bin])
		dB := 10 * math.Log10(p+1e-20)
		fmt.Printf("%.0f  %.1f\n", f, dB)
	}
}
