package goft8

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

// WAVFormat describes the mono WAV format parsed by ReadWAVMono.
type WAVFormat struct {
	SampleRate int
	BitDepth   int
	PCMFormat  int
	Channels   int
}

// WriteWAV writes float32 samples as a standard RIFF WAV file.
//
// Parameters:
//   - name: output file path
//   - samples: float32 mono samples in [-1, 1]
//   - sampleRate: sample rate in Hz (e.g. 12000 or 48000)
//   - bitDepth: PCM bit depth (16, 24, or 32)
func WriteWAV(name string, samples []float32, sampleRate int, bitDepth int) error {
	var data []byte
	var audioFormat uint16

	switch bitDepth {
	case 24:
		data = FloatToPCM(samples, 24)
		audioFormat = 1 // PCM
	case 32:
		data = FloatToPCM(samples, 32)
		audioFormat = 1 // PCM
	default:
		data = FloatToPCM(samples, 16)
		audioFormat = 1 // PCM
		bitDepth = 16
	}

	numChannels := 1
	bitsPerSample := bitDepth
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataSize := len(data)

	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()

	// RIFF header
	if _, err := f.WriteString("RIFF"); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(36+dataSize)); err != nil {
		return err
	}
	if _, err := f.WriteString("WAVE"); err != nil {
		return err
	}

	// fmt sub-chunk
	if _, err := f.WriteString("fmt "); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(16)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, audioFormat); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(numChannels)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(byteRate)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(blockAlign)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(bitsPerSample)); err != nil {
		return err
	}

	// data sub-chunk
	if _, err := f.WriteString("data"); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(dataSize)); err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

// ReadWAVParams reads just the fmt chunk from a WAV file and returns sample
// rate, bit depth, and PCM format. Only mono WAV files are accepted.
func ReadWAVParams(path string) (sampleRate int, bitDepth int, pcmFormat int, err error) {
	_, format, err := readWAVMono(path, false, wavScaleNormalized)
	if err != nil {
		return 0, 0, 0, err
	}
	return format.SampleRate, format.BitDepth, format.PCMFormat, nil
}

// ReadWAVMono decodes a mono WAV file into normalized float32 samples and
// returns the parsed WAV format. It accepts integer PCM 16/24/32-bit and IEEE
// float32 samples.
func ReadWAVMono(path string) ([]float32, WAVFormat, error) {
	return readWAVMono(path, true, wavScaleNormalized)
}

// ReadWAVMono12k decodes a mono WAV file into float32 samples at 12 kHz. It
// accepts 12 kHz input directly and 48 kHz input via simple 4:1 downsampling.
func ReadWAVMono12k(path string) ([]float32, error) {
	samples, format, err := readWAVMono(path, true, wavScaleDecoder)
	if err != nil {
		return nil, err
	}
	switch format.SampleRate {
	case AudioSampleRate:
		return samples, nil
	case 48000:
		return downsample4x(samples), nil
	default:
		return nil, fmt.Errorf("goft8: want %d Hz or 48000 Hz WAV, got %d Hz", AudioSampleRate, format.SampleRate)
	}
}

type wavScaleMode int

const (
	wavScaleNormalized wavScaleMode = iota
	wavScaleDecoder
)

func readWAVMono(path string, readData bool, scale wavScaleMode) ([]float32, WAVFormat, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, WAVFormat{}, err
	}
	defer f.Close()

	var riff [12]byte
	if _, err := io.ReadFull(f, riff[:]); err != nil {
		return nil, WAVFormat{}, fmt.Errorf("goft8: read RIFF header: %w", err)
	}
	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		return nil, WAVFormat{}, errors.New("goft8: not a RIFF/WAVE file")
	}

	var format WAVFormat
	var fmtFound bool

	for {
		var hdr [8]byte
		if _, err := io.ReadFull(f, hdr[:]); err != nil {
			return nil, WAVFormat{}, fmt.Errorf("goft8: read chunk header: %w", err)
		}
		chunkID := string(hdr[0:4])
		chunkSize := binary.LittleEndian.Uint32(hdr[4:8])

		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return nil, WAVFormat{}, errors.New("goft8: fmt chunk too short")
			}
			body := make([]byte, chunkSize)
			if _, err := io.ReadFull(f, body); err != nil {
				return nil, WAVFormat{}, fmt.Errorf("goft8: read fmt chunk: %w", err)
			}
			format = WAVFormat{
				PCMFormat:  int(binary.LittleEndian.Uint16(body[0:2])),
				Channels:   int(binary.LittleEndian.Uint16(body[2:4])),
				SampleRate: int(binary.LittleEndian.Uint32(body[4:8])),
				BitDepth:   int(binary.LittleEndian.Uint16(body[14:16])),
			}
			if format.Channels != 1 {
				return nil, WAVFormat{}, fmt.Errorf("goft8: want mono WAV, got %d channels", format.Channels)
			}
			if !readData {
				return nil, format, nil
			}
			fmtFound = true
		case "data":
			if !fmtFound {
				return nil, WAVFormat{}, errors.New("goft8: data chunk before fmt chunk")
			}
			samples, err := readWAVSamples(f, int(chunkSize), format, scale)
			if err != nil {
				return nil, WAVFormat{}, err
			}
			return samples, format, nil
		default:
			if err := skipWAVChunk(f, chunkID, chunkSize); err != nil {
				return nil, WAVFormat{}, err
			}
		}
	}
}

func skipWAVChunk(r io.Reader, chunkID string, chunkSize uint32) error {
	if _, err := io.CopyN(io.Discard, r, int64(chunkSize)); err != nil {
		return fmt.Errorf("goft8: skip %s chunk: %w", chunkID, err)
	}
	if chunkSize%2 == 1 {
		if _, err := io.CopyN(io.Discard, r, 1); err != nil {
			return fmt.Errorf("goft8: skip %s padding: %w", chunkID, err)
		}
	}
	return nil
}

func readWAVSamples(r io.Reader, size int, format WAVFormat, scale wavScaleMode) ([]float32, error) {
	switch {
	case format.PCMFormat == 1 && format.BitDepth == 16:
		return readPCM16(r, size, scale)
	case format.PCMFormat == 1 && format.BitDepth == 24:
		return readPCM24(r, size)
	case format.PCMFormat == 1 && format.BitDepth == 32:
		return readPCM32(r, size)
	case format.PCMFormat == 3 && format.BitDepth == 32:
		return readFloat32(r, size)
	default:
		return nil, fmt.Errorf("goft8: unsupported WAV format=%d bits=%d (want PCM16/24/32 or float32)", format.PCMFormat, format.BitDepth)
	}
}

func readPCM16(r io.Reader, size int, scale wavScaleMode) ([]float32, error) {
	if size%2 != 0 {
		return nil, fmt.Errorf("goft8: PCM16 data size %d is not sample-aligned", size)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("goft8: read data chunk: %w", err)
	}
	out := make([]float32, size/2)
	for i := range out {
		s := int16(binary.LittleEndian.Uint16(buf[i*2 : i*2+2]))
		if scale == wavScaleDecoder {
			// Match MSHV static_dat0 = raw_in_s * 0.000390625 scaling.
			out[i] = float32(s) * 0.000390625
		} else {
			out[i] = float32(s) / 32768.0
		}
	}
	return out, nil
}

func readPCM24(r io.Reader, size int) ([]float32, error) {
	if size%3 != 0 {
		return nil, fmt.Errorf("goft8: PCM24 data size %d is not sample-aligned", size)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("goft8: read data chunk: %w", err)
	}
	out := make([]float32, size/3)
	for i := range out {
		b0 := int32(buf[i*3])
		b1 := int32(buf[i*3+1])
		b2 := int32(buf[i*3+2])
		s := (b0 & 0xFF) | ((b1 & 0xFF) << 8) | ((b2 & 0xFF) << 16)
		if s&0x800000 != 0 {
			s |= ^0x7FFFFF
		}
		out[i] = float32(s) / 8388608.0
	}
	return out, nil
}

func readPCM32(r io.Reader, size int) ([]float32, error) {
	if size%4 != 0 {
		return nil, fmt.Errorf("goft8: PCM32 data size %d is not sample-aligned", size)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("goft8: read data chunk: %w", err)
	}
	out := make([]float32, size/4)
	for i := range out {
		out[i] = float32(int32(binary.LittleEndian.Uint32(buf[i*4:]))) / 2147483648.0
	}
	return out, nil
}

func readFloat32(r io.Reader, size int) ([]float32, error) {
	if size%4 != 0 {
		return nil, fmt.Errorf("goft8: float32 data size %d is not sample-aligned", size)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("goft8: read data chunk: %w", err)
	}
	out := make([]float32, size/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return out, nil
}

func downsample4x(in []float32) []float32 {
	if len(in)%4 != 0 {
		in = in[:len(in)/4*4]
	}
	out := make([]float32, len(in)/4)
	for i := range out {
		j := i * 4
		out[i] = (in[j] + in[j+1] + in[j+2] + in[j+3]) * 0.25
	}
	return out
}
