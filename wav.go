package goft8

import (
	"encoding/binary"
	"fmt"
	"os"
)

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

// ReadWAVParams reads just the fmt chunk from a WAV file and returns
// sample rate, bit depth, and PCM format.
func ReadWAVParams(path string) (sampleRate int, bitDepth int, pcmFormat int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, 0, err
	}
	defer f.Close()

	var riff [12]byte
	if _, err := f.Read(riff[:]); err != nil {
		return 0, 0, 0, fmt.Errorf("read RIFF header: %w", err)
	}
	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		return 0, 0, 0, fmt.Errorf("not a RIFF/WAVE file")
	}

	for {
		var hdr [8]byte
		if _, err := f.Read(hdr[:]); err != nil {
			return 0, 0, 0, fmt.Errorf("read chunk header: %w", err)
		}
		chunkID := string(hdr[0:4])
		chunkSize := binary.LittleEndian.Uint32(hdr[4:8])

		if chunkID == "fmt " {
			if chunkSize < 16 {
				return 0, 0, 0, fmt.Errorf("fmt chunk too short")
			}
			body := make([]byte, chunkSize)
			if _, err := f.Read(body); err != nil {
				return 0, 0, 0, fmt.Errorf("read fmt chunk: %w", err)
			}
			pcmFormat = int(binary.LittleEndian.Uint16(body[0:2]))
			numChannels := int(binary.LittleEndian.Uint16(body[2:4]))
			if numChannels != 1 {
				return 0, 0, 0, fmt.Errorf("want mono WAV, got %d channels", numChannels)
			}
			sampleRate = int(binary.LittleEndian.Uint32(body[4:8]))
			bitDepth = int(binary.LittleEndian.Uint16(body[14:16]))
			return sampleRate, bitDepth, pcmFormat, nil
		}

		// Skip non-fmt chunks (WAV spec: odd-size chunks have a padding byte).
		if _, err := f.Seek(int64(chunkSize)+int64(chunkSize%2), 1); err != nil {
			return 0, 0, 0, err
		}
	}
}
