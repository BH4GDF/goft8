package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bh4gdf/goft8"
)

func TestRunGeneratesWAV(t *testing.T) {
	var stdout, stderr bytes.Buffer
	chdir(t, t.TempDir())

	code := run([]string{"genwav", "-rate", "12000", "-bits", "16"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run failed with code %d, stderr: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "ft8_multi_12000hz_16bit.wav written") {
		t.Fatalf("stdout = %q, want generated filename", stdout.String())
	}
	writtenSamples := parseGeneratedSampleCount(t, stdout.String())

	path := "ft8_multi_12000hz_16bit.wav"
	samples, format, err := goft8.ReadWAVMono(path)
	if err != nil {
		t.Fatalf("ReadWAVMono generated file: %v", err)
	}
	if format.SampleRate != 12000 || format.BitDepth != 16 || format.Channels != 1 {
		t.Fatalf("format = %+v, want 12000 Hz 16-bit mono", format)
	}
	if len(samples) != writtenSamples {
		t.Fatalf("samples = %d, want stdout count %d", len(samples), writtenSamples)
	}
	if len(samples) == 0 {
		t.Fatal("generated WAV has no samples")
	}
}

func TestRunRejectsUnsupportedRate(t *testing.T) {
	var stdout, stderr bytes.Buffer
	chdir(t, t.TempDir())

	code := run([]string{"genwav", "-rate", "44100"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("run returned success for unsupported sample rate")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unsupported sample rate 44100") {
		t.Fatalf("stderr = %q, want unsupported sample rate", stderr.String())
	}
	if _, err := os.Stat("ft8_multi_44100hz_24bit.wav"); !os.IsNotExist(err) {
		t.Fatalf("unexpected output file exists or stat failed: %v", err)
	}
}

func TestRunRejectsUnsupportedBitDepth(t *testing.T) {
	var stdout, stderr bytes.Buffer
	chdir(t, t.TempDir())

	code := run([]string{"genwav", "-bits", "8"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("run returned success for unsupported bit depth")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unsupported bit depth 8") {
		t.Fatalf("stderr = %q, want unsupported bit depth", stderr.String())
	}
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	chdir(t, t.TempDir())

	code := run([]string{"genwav", "-unknown"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("run returned success for unknown flag")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("stderr = %q, want flag parse error", stderr.String())
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()

	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(filepath.Clean(dir)); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}

func parseGeneratedSampleCount(t *testing.T, out string) int {
	t.Helper()

	fields := strings.Fields(out)
	if len(fields) < 2 {
		t.Fatalf("stdout = %q, want sample count field", out)
	}
	raw := fields[len(fields)-2]
	n, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("parse sample count from %q: %v", out, err)
	}
	return n
}
