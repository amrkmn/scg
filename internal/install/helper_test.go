package install

import "testing"

func TestExtractExtension(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"file.zip", ".zip"},
		{"file.7z", ".7z"},
		{"file.tar.gz", ".gz"},
		{"FILE.MSI", ".msi"},
		{"noext", ""},
		{"path/to/file.exe", ".exe"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractExtension(tt.input)
			if got != tt.want {
				t.Errorf("ExtractExtension(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsArchive(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"file.zip", true},
		{"file.7z", true},
		{"file.tar.gz", true},
		{"file.msi", true},
		{"file.exe", false},
		{"file.txt", false},
		{"file.json", false},
		{"file.go", false},
		{"archive.tar.xz", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsArchive(tt.input)
			if got != tt.want {
				t.Errorf("IsArchive(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindHelper(t *testing.T) {
	t.Run("nonexistent helper", func(t *testing.T) {
		_, err := FindHelper("nonexistent-app", "nonexistent.exe")
		if err == nil {
			t.Error("FindHelper with nonexistent app should return error")
		}
	})
}
