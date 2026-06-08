package main

import (
	"bytes"
	"encoding/json"
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
	if !strings.Contains(stderr.String(), "usage: decodewav [options] <wavfile>") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestRunRejectsInvalidFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "unknown flag",
			args: []string{"decodewav", "-unknown"},
			want: "flag provided but not defined",
		},
		{
			name: "bad depth",
			args: []string{"decodewav", "-depth", "extreme", "testdata/ft8_cap1.wav"},
			want: "invalid depth",
		},
		{
			name: "bad frequency range",
			args: []string{"decodewav", "-freq-min", "2000", "-freq-max", "1000", "testdata/ft8_cap1.wav"},
			want: "invalid frequency range",
		},
		{
			name: "too many positional args",
			args: []string{"decodewav", "a.wav", "b.wav"},
			want: "usage: decodewav [options] <wavfile>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)

			if code == 0 {
				t.Fatal("run returned success for invalid args")
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.want)
			}
		})
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

func TestRunDecodeFixtureWithFlags(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "ft8_cap1.wav")
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"decodewav",
		"-freq-min", "500",
		"-freq-max", "1200",
		"-depth", "deep",
		"-workers", "2",
		path,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run failed with code %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Decoded ") {
		t.Fatalf("stdout missing decoded summary:\n%s", out)
	}
	if strings.Contains(out, "SV2SIH ES2AJ -16") {
		t.Fatalf("frequency range did not filter high-frequency message:\n%s", out)
	}
	if !strings.Contains(out, "A61CK W3DQS -12") {
		t.Fatalf("stdout missing expected in-range decode:\n%s", out)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunJSONOutput(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "ft8_cap1.wav")
	var stdout, stderr bytes.Buffer

	code := run([]string{"decodewav", "-json", "-freq-min", "500", "-freq-max", "1200", path}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run failed with code %d, stderr: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if strings.Contains(stdout.String(), "File:") || strings.Contains(stdout.String(), "Decoded ") {
		t.Fatalf("json output contains text format:\n%s", stdout.String())
	}

	var got jsonResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal json output: %v\n%s", err, stdout.String())
	}
	if got.File != path {
		t.Fatalf("file = %q, want %q", got.File, path)
	}
	if got.Format.SampleRate != 12000 || got.Format.BitDepth != 16 || got.Format.Name != "PCM" {
		t.Fatalf("format = %+v", got.Format)
	}
	if got.Samples != 180000 || got.DurationSec != 15 {
		t.Fatalf("samples/duration = %d/%g", got.Samples, got.DurationSec)
	}
	if len(got.Decoded) == 0 {
		t.Fatal("decoded array is empty")
	}
	found := false
	for _, d := range got.Decoded {
		if d.Message == "A61CK W3DQS -12" {
			found = true
			if d.Freq < 500 || d.Freq > 1200 {
				t.Fatalf("decoded freq = %g outside requested range", d.Freq)
			}
		}
	}
	if !found {
		t.Fatalf("json missing expected message: %+v", got.Decoded)
	}
}
