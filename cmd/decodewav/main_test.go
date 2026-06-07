package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"decodewav"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("run returned success for missing wav path")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: decodewav <wavfile>") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestRunMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"decodewav", filepath.Join(t.TempDir(), "missing.wav")}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("run returned success for missing file")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "read wav failed") {
		t.Fatalf("stderr = %q, want read failure", stderr.String())
	}
}

func TestRunRejectsInvalidWAV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.wav")
	if err := os.WriteFile(path, []byte("not a wav"), 0o600); err != nil {
		t.Fatalf("write invalid wav: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{"decodewav", path}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("run returned success for invalid wav")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "read wav failed") {
		t.Fatalf("stderr = %q, want read failure", stderr.String())
	}
}

func TestRunDecodeFixture(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "ft8_cap1.wav")
	var stdout, stderr bytes.Buffer

	code := run([]string{"decodewav", path}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run failed with code %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"File: " + path,
		"Format: 12000 Hz, 16-bit, PCM",
		"Samples: 180000 (15.000 s @ 12000 Hz)",
		"Decoded 12 message(s):",
		"SV2SIH ES2AJ -16",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
