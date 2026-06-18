package commands

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseCreateURLSegments(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "release url",
			input: "https://example.com/releases/app-1.2.3.zip",
			want:  []string{"releases", "app-1.2.3.zip"},
		},
		{
			name:  "query preserved on last segment",
			input: "https://example.com/files/tool.zip?download=1",
			want:  []string{"files", "tool.zip?download=1"},
		},
		{
			name:    "invalid url",
			input:   "not-a-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCreateURLSegments(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCreateURLSegments() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseCreateURLSegments() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCreateFilenameStem(t *testing.T) {
	tests := []struct {
		name    string
		segment string
		want    string
	}{
		{name: "zip file", segment: "app-1.2.3.zip", want: "app-1.2.3"},
		{name: "query suffix", segment: "tool.exe?download=1", want: "tool"},
		{name: "no extension", segment: "portable", want: "portable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := createFilenameStem(tt.segment); got != tt.want {
				t.Fatalf("createFilenameStem() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChooseCreateValue(t *testing.T) {
	t.Run("numeric selection", func(t *testing.T) {
		var out bytes.Buffer
		reader := bufio.NewReader(strings.NewReader("2\n"))
		got, err := chooseCreateValue(reader, &out, []string{"one", "two", "three"}, "Version")
		if err != nil {
			t.Fatalf("chooseCreateValue() error = %v", err)
		}
		if got != "two" {
			t.Fatalf("chooseCreateValue() = %q, want %q", got, "two")
		}
		if !strings.Contains(out.String(), "Version: ") {
			t.Fatalf("prompt output = %q, want Version prompt", out.String())
		}
	})

	t.Run("typed selection", func(t *testing.T) {
		reader := bufio.NewReader(strings.NewReader("custom-name\n"))
		got, err := chooseCreateValue(reader, &bytes.Buffer{}, []string{"one", "two"}, "App name")
		if err != nil {
			t.Fatalf("chooseCreateValue() error = %v", err)
		}
		if got != "custom-name" {
			t.Fatalf("chooseCreateValue() = %q, want %q", got, "custom-name")
		}
	})
}

func TestMarshalCreateManifestSkeleton(t *testing.T) {
	manifest := newCreateManifestSkeleton("https://example.com/app.zip", "1.2.3")
	got, err := marshalCreateManifestSkeleton(manifest)
	if err != nil {
		t.Fatalf("marshalCreateManifestSkeleton() error = %v", err)
	}

	want := strings.Join([]string{
		"{",
		"  \"homepage\": \"\",",
		"  \"license\": \"\",",
		"  \"version\": \"1.2.3\",",
		"  \"url\": \"https://example.com/app.zip\",",
		"  \"hash\": \"\",",
		"  \"extract_dir\": \"\",",
		"  \"bin\": \"\",",
		"  \"depends\": \"\"",
		"}",
		"",
	}, "\n")

	if string(got) != want {
		t.Fatalf("marshalCreateManifestSkeleton() = %q, want %q", string(got), want)
	}
}

func TestWriteCreateManifestFile(t *testing.T) {
	dir := t.TempDir()
	manifest := newCreateManifestSkeleton("https://example.com/app.zip", "1.2.3")

	path, err := writeCreateManifestFile(dir, "app", manifest)
	if err != nil {
		t.Fatalf("writeCreateManifestFile() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "app.json"))
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if string(data) == "" {
		t.Fatal("expected manifest data to be written")
	}
	if !strings.HasSuffix(path, filepath.Join(dir, "app.json")) {
		t.Fatalf("writeCreateManifestFile() path = %q", path)
	}

	if _, err := writeCreateManifestFile(dir, "app", manifest); err == nil {
		t.Fatal("expected existing-file protection error")
	}
}

func TestNewCreateCommand_ShowsHelpWhenURLMissing(t *testing.T) {
	cmd := NewCreateCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Usage:") {
		t.Fatalf("help output = %q, want usage", output)
	}
	if !strings.Contains(output, "create <url>") {
		t.Fatalf("help output = %q, want create usage", output)
	}
}
