package goft8

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func FuzzParseMessage(f *testing.F) {
	for _, seed := range []string{
		"CQ BH4GDF PM00",
		"BH4GDF K1ABC -12",
		"BH4GDF K1ABC RR73",
		"ABCDEF1234567890AB",
		"",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		_, _ = ParseMessage(raw)
	})
}

func FuzzReadWAVMono(f *testing.F) {
	f.Add([]byte("not a wav"))
	f.Add(fuzzWAVSeed())

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		path := filepath.Join(t.TempDir(), "input.wav")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		_, _, _, _ = ReadWAVParams(path)
		_, _, _ = ReadWAVMono(path)
	})
}

func fuzzWAVSeed() []byte {
	var buf bytes.Buffer
	buf.WriteString("RIFF")
	writeSeedLE(&buf, uint32(40))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	writeSeedLE(&buf, uint32(16))
	writeSeedLE(&buf, uint16(1))
	writeSeedLE(&buf, uint16(1))
	writeSeedLE(&buf, uint32(12000))
	writeSeedLE(&buf, uint32(24000))
	writeSeedLE(&buf, uint16(2))
	writeSeedLE(&buf, uint16(16))
	buf.WriteString("data")
	writeSeedLE(&buf, uint32(4))
	writeSeedLE(&buf, int16(0))
	writeSeedLE(&buf, int16(1024))
	return buf.Bytes()
}

func writeSeedLE(buf *bytes.Buffer, v any) {
	_ = binary.Write(buf, binary.LittleEndian, v)
}
