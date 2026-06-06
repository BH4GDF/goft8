package goft8

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type wavTestChunk struct {
	id   string
	data []byte
}

func TestReadWAVMonoSupportsPCMFormats(t *testing.T) {
	tests := []struct {
		name     string
		bitDepth uint16
		data     []byte
		want     []float32
	}{
		{
			name:     "pcm16",
			bitDepth: 16,
			data:     pcm16TestData(-32768, 0, 16384, 32767),
			want:     []float32{-1, 0, 0.5, 32767.0 / 32768.0},
		},
		{
			name:     "pcm24",
			bitDepth: 24,
			data:     pcm24TestData(-8388608, 0, 4194304, 8388607),
			want:     []float32{-1, 0, 0.5, 8388607.0 / 8388608.0},
		},
		{
			name:     "pcm32",
			bitDepth: 32,
			data:     pcm32TestData(math.MinInt32, 0, 1073741824, math.MaxInt32),
			want:     []float32{-1, 0, 0.5, float32(float64(math.MaxInt32) / 2147483648.0)},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.name+".wav")
			writeTestWAV(t, path, 1, 1, 12000, tc.bitDepth, tc.data)

			got, format, err := ReadWAVMono(path)
			if err != nil {
				t.Fatalf("ReadWAVMono: %v", err)
			}
			if format.SampleRate != 12000 || format.BitDepth != int(tc.bitDepth) || format.PCMFormat != 1 || format.Channels != 1 {
				t.Fatalf("format = %+v", format)
			}
			assertFloat32SliceClose(t, got, tc.want, 1e-6)
		})
	}
}

func TestReadWAVMonoSupportsFloat32(t *testing.T) {
	path := filepath.Join(t.TempDir(), "float32.wav")
	want := []float32{-1, -0.25, 0.5, 1}
	writeTestWAV(t, path, 3, 1, 12000, 32, float32TestData(want...))

	got, format, err := ReadWAVMono(path)
	if err != nil {
		t.Fatalf("ReadWAVMono: %v", err)
	}
	if format.PCMFormat != 3 || format.BitDepth != 32 {
		t.Fatalf("format = %+v", format)
	}
	assertFloat32SliceClose(t, got, want, 0)
}

func TestReadWAVMono12kDownsamples48k(t *testing.T) {
	path := filepath.Join(t.TempDir(), "48k.wav")
	writeTestWAV(t, path, 1, 1, 48000, 16, pcm16TestData(4, 8, 12, 16, 20, 24, 28, 32))

	got, err := ReadWAVMono12k(path)
	if err != nil {
		t.Fatalf("ReadWAVMono12k: %v", err)
	}
	want := []float32{
		10 * 0.000390625,
		26 * 0.000390625,
	}
	assertFloat32SliceClose(t, got, want, 1e-8)
}

func TestReadWAVParamsSkipsUnknownOddChunk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "junk.wav")
	writeTestWAV(t, path, 1, 1, 12000, 16, pcm16TestData(0), wavTestChunk{id: "JUNK", data: []byte{1, 2, 3}})

	sampleRate, bitDepth, pcmFormat, err := ReadWAVParams(path)
	if err != nil {
		t.Fatalf("ReadWAVParams: %v", err)
	}
	if sampleRate != 12000 || bitDepth != 16 || pcmFormat != 1 {
		t.Fatalf("params = rate %d bits %d format %d", sampleRate, bitDepth, pcmFormat)
	}
}

func TestReadWAVRejectsMalformedInputs(t *testing.T) {
	t.Run("truncated header", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "truncated.wav")
		if err := os.WriteFile(path, []byte("RIFF"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, _, err := ReadWAVMono(path)
		assertErrorContains(t, err, "read RIFF header")
	})

	t.Run("non mono", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "stereo.wav")
		writeTestWAV(t, path, 1, 2, 12000, 16, pcm16TestData(0, 0))
		_, _, err := ReadWAVMono(path)
		assertErrorContains(t, err, "want mono WAV")
	})

	t.Run("truncated unknown chunk", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "truncated-chunk.wav")
		var buf bytes.Buffer
		buf.WriteString("RIFF")
		writeLE(t, &buf, uint32(4+8+8))
		buf.WriteString("WAVE")
		buf.WriteString("JUNK")
		writeLE(t, &buf, uint32(8))
		buf.Write([]byte{1, 2})
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		_, _, _, err := ReadWAVParams(path)
		assertErrorContains(t, err, "skip JUNK")
	})

	t.Run("unsupported sample rate for decoder", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "44100.wav")
		writeTestWAV(t, path, 1, 1, 44100, 16, pcm16TestData(0, 0, 0, 0))
		_, err := ReadWAVMono12k(path)
		assertErrorContains(t, err, "want 12000 Hz or 48000 Hz")
	})

	t.Run("unsupported bit depth", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "pcm8.wav")
		writeTestWAV(t, path, 1, 1, 12000, 8, []byte{0})
		_, _, err := ReadWAVMono(path)
		assertErrorContains(t, err, "unsupported WAV format=1 bits=8")
	})
}

func writeTestWAV(t *testing.T, path string, audioFormat, channels uint16, sampleRate uint32, bitDepth uint16, data []byte, beforeFmt ...wavTestChunk) {
	t.Helper()

	blockAlign := channels * bitDepth / 8
	byteRate := sampleRate * uint32(blockAlign)

	var fmtBody bytes.Buffer
	writeLE(t, &fmtBody, audioFormat)
	writeLE(t, &fmtBody, channels)
	writeLE(t, &fmtBody, sampleRate)
	writeLE(t, &fmtBody, byteRate)
	writeLE(t, &fmtBody, blockAlign)
	writeLE(t, &fmtBody, bitDepth)

	chunks := append([]wavTestChunk{}, beforeFmt...)
	chunks = append(chunks,
		wavTestChunk{id: "fmt ", data: fmtBody.Bytes()},
		wavTestChunk{id: "data", data: data},
	)

	riffSize := uint32(4)
	for _, chunk := range chunks {
		riffSize += uint32(8 + len(chunk.data) + len(chunk.data)%2)
	}

	var buf bytes.Buffer
	buf.WriteString("RIFF")
	writeLE(t, &buf, riffSize)
	buf.WriteString("WAVE")
	for _, chunk := range chunks {
		writeChunk(t, &buf, chunk)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeChunk(t *testing.T, buf *bytes.Buffer, chunk wavTestChunk) {
	t.Helper()
	if len(chunk.id) != 4 {
		t.Fatalf("chunk id %q length = %d, want 4", chunk.id, len(chunk.id))
	}
	buf.WriteString(chunk.id)
	writeLE(t, buf, uint32(len(chunk.data)))
	buf.Write(chunk.data)
	if len(chunk.data)%2 == 1 {
		buf.WriteByte(0)
	}
}

func writeLE(t *testing.T, buf *bytes.Buffer, v any) {
	t.Helper()
	if err := binary.Write(buf, binary.LittleEndian, v); err != nil {
		t.Fatal(err)
	}
}

func pcm16TestData(vals ...int16) []byte {
	data := make([]byte, len(vals)*2)
	for i, v := range vals {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(v))
	}
	return data
}

func pcm24TestData(vals ...int32) []byte {
	data := make([]byte, len(vals)*3)
	for i, v := range vals {
		data[i*3] = byte(v)
		data[i*3+1] = byte(v >> 8)
		data[i*3+2] = byte(v >> 16)
	}
	return data
}

func pcm32TestData(vals ...int32) []byte {
	data := make([]byte, len(vals)*4)
	for i, v := range vals {
		binary.LittleEndian.PutUint32(data[i*4:], uint32(v))
	}
	return data
}

func float32TestData(vals ...float32) []byte {
	data := make([]byte, len(vals)*4)
	for i, v := range vals {
		binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(v))
	}
	return data
}

func assertFloat32SliceClose(t *testing.T, got, want []float32, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d; got %v", len(got), len(want), got)
	}
	for i := range got {
		if math.Abs(float64(got[i]-want[i])) > tol {
			t.Fatalf("sample %d = %g, want %g", i, got[i], want[i])
		}
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want substring %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want substring %q", err, want)
	}
}
