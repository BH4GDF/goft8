// Package goft8 is a clean-room Go implementation of the FT8 digital
// mode decoder and encoder.
//
// It is designed for embedding in amateur-radio station software and
// targets WSJT-X 2.7.0 decode-yield parity. The public API includes:
//   - Decoder: stateful FT8 decoder with iterative subtraction and AP support
//   - Encoder: GFSK waveform generator with 12/48 kHz and 16/24/32-bit output
//   - Message parser: structured parsing of decoded FT8 messages
//   - WAV I/O: read/write mono WAV with automatic rate conversion
//
// Typical receive usage:
//
//	dec := NewDecoder(WithMyCall("W1ABC"))
//	for audio := range cycleCh { // 180000 samples, 12 kHz mono
//	    decodes, err := dec.Decode(audio)
//	    if err != nil {
//	        log.Println(err)
//	        continue
//	    }
//	    for _, d := range decodes {
//	        fmt.Printf("%+d %4.1f %6.1f %s\n", d.SNR, d.DT, d.Freq, d.Message)
//	    }
//	}
//
// A Decoder is stateful and NOT safe for concurrent use by multiple
// goroutines. Each receive stream should have its own Decoder.
//
// goft8 is not a binding of WSJT-X or JTDX; it is an original Go
// implementation of the published FT8 protocol, developed with
// reference to the GPLv3 WSJT-X 2.7.0 Fortran source. The Go code is
// licensed under GPL v3. See README.md for clean-room attribution.
package goft8
