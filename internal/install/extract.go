package install

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Extractor handles archive extraction for Scoop packages.
type Extractor struct {
	useExternal7zip bool
	verbose         bool
}

// NewExtractor creates a new Extractor. If useExternal7zip is true, it prefers
// the system 7z over the Scoop-installed one.
func NewExtractor(useExternal7zip bool, verbose bool) *Extractor {
	return &Extractor{
		useExternal7zip: useExternal7zip,
		verbose:         verbose,
	}
}

// ExtractionOptions configures how an archive should be extracted.
type ExtractionOptions struct {
	// InnoSetup indicates the .exe is an Inno Setup installer.
	InnoSetup bool
	// MSI indicates the file should be extracted as an MSI.
	MSI bool
}

// Extract extracts an archive to the destination directory.
// It selects the appropriate extraction method based on the file extension.
func (e *Extractor) Extract(archivePath, destDir string, opts ExtractionOptions) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	ext := ExtractExtension(archivePath)

	// Determine extraction method.
	switch {
	case opts.InnoSetup:
		return e.extractInnoSetup(archivePath, destDir)
	case ext == ".msi" || opts.MSI:
		return e.extractMSI(archivePath, destDir)
	case ext == ".zip":
		return e.extractZip(archivePath, destDir)
	case isSevenZipArchive(ext):
		return e.extract7zip(archivePath, destDir)
	default:
		// Try 7zip as a fallback for unknown formats.
		return e.extract7zip(archivePath, destDir)
	}
}

// extractZip extracts a .zip file. It tries Go's built-in archive/zip first,
// then falls back to 7zip.
func (e *Extractor) extractZip(archivePath, destDir string) error {
	// Try native Go extraction first.
	if err := e.extractZipNative(archivePath, destDir); err == nil {
		return nil
	}

	// Fall back to 7zip.
	return e.extract7zip(archivePath, destDir)
}

// extractZipNative uses Go's archive/zip package.
func (e *Extractor) extractZipNative(archivePath, destDir string) error {
	// Read the zip file.
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		targetPath := filepath.Join(destDir, f.Name)

		// Security: prevent zip slip.
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("zip slip detected: %s escapes destination", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, f.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := extractZipFile(f, targetPath); err != nil {
			return err
		}
	}
	return nil
}

// extract7zip extracts using 7z.exe.
func (e *Extractor) extract7zip(archivePath, destDir string) error {
	sevenZipPath, err := e.find7zip()
	if err != nil {
		return fmt.Errorf("7zip not found: %w", err)
	}

	args := []string{"x", archivePath, fmt.Sprintf("-o%s", destDir), "-aoa", "-y"}
	if e.verbose {
		args = append(args, "-bb1")
	}

	cmd := exec.Command(sevenZipPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("7zip extraction failed: %w\n%s", err, out)
	}

	// Handle double-layer archives (.tgz, .tar.gz, .tar.bz2, .tar.xz).
	// 7z extracts .tgz → .tar but does not auto-extract the inner .tar.
	ext := ExtractExtension(archivePath)
	if isTarWrapper(ext) {
		if err := extractInnerTar(sevenZipPath, destDir); err != nil {
			return fmt.Errorf("inner tar extraction failed: %w", err)
		}
	}

	return nil
}

// isTarWrapper returns true if the extension indicates a compressed tar archive
// that 7z will decompress to a .tar file. Matches Scoop's pattern: .t[abgpx]z2?
func isTarWrapper(ext string) bool {
	switch ext {
	case ".tgz", ".tbz", ".tbz2", ".txz", ".tpz", ".taz",
		".tar.gz", ".tar.bz2", ".tar.xz", ".gz", ".bz2":
		return true
	default:
		return false
	}
}

// extractInnerTar finds and extracts any .tar files left in destDir after
// a .tgz/.tar.gz extraction, then cleans up the .tar files.
func extractInnerTar(sevenZipPath, destDir string) error {
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar") {
			continue
		}

		tarPath := filepath.Join(destDir, entry.Name())
		cmd := exec.Command(sevenZipPath, "x", tarPath, fmt.Sprintf("-o%s", destDir), "-aoa", "-y")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to extract %s: %w\n%s", entry.Name(), err, out)
		}

		// Clean up the intermediate .tar file.
		_ = os.Remove(tarPath)
	}

	return nil
}

// extractMSI extracts an MSI file using lessmsi or msiexec.
func (e *Extractor) extractMSI(archivePath, destDir string) error {
	// Try lessmsi first.
	if lessmsiPath, err := FindLessmsi(); err == nil {
		return e.extractLessmsi(lessmsiPath, archivePath, destDir)
	}

	// Fall back to msiexec.
	return e.extractMSIExec(archivePath, destDir)
}

// extractLessmsi extracts an MSI using lessmsi.
func (e *Extractor) extractLessmsi(lessmsiPath, archivePath, destDir string) error {
	cmd := exec.Command(lessmsiPath, "x", archivePath, destDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lessmsi extraction failed: %w\n%s", err, out)
	}
	return nil
}

// extractMSIExec extracts an MSI using msiexec.
func (e *Extractor) extractMSIExec(archivePath, destDir string) error {
	cmd := exec.Command("msiexec", "/a", archivePath, "/qn", fmt.Sprintf("TARGETDIR=%s", destDir))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("msiexec extraction failed: %w\n%s", err, out)
	}
	return nil
}

// extractInnoSetup extracts an Inno Setup .exe using innounp.
func (e *Extractor) extractInnoSetup(archivePath, destDir string) error {
	innounpPath, err := FindInnounp()
	if err != nil {
		return fmt.Errorf("innounp not found (required for Inno Setup installer): %w", err)
	}

	cmd := exec.Command(innounpPath, "-x", "-d"+destDir, "-y", archivePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("innounp extraction failed: %w\n%s", err, out)
	}
	return nil
}

// find7zip locates the 7z executable.
func (e *Extractor) find7zip() (string, error) {
	if e.useExternal7zip {
		// Try system 7z first.
		if p, err := exec.LookPath("7z"); err == nil {
			return p, nil
		}
	}
	return Find7zip()
}

// isSevenZipArchive returns true if the extension is a 7zip-supported archive format.
func isSevenZipArchive(ext string) bool {
	switch ext {
	case ".7z", ".tar", ".gz", ".bz2", ".xz", ".tgz",
		".lzma", ".lz4", ".zst",
		".tar.gz", ".tar.bz2", ".tar.xz":
		return true
	default:
		return strings.Contains(ext, ".tar") || ext == ".exe"
	}
}

// extractZipFile extracts a single file from a zip archive.
func extractZipFile(f *zip.File, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	w, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = w.Close() }()

	_, err = io.Copy(w, rc)
	return err
}

// FlattenExtractDir moves the contents of a subdirectory (extract_dir) to the parent directory.
// This matches Scoop's extract_dir behavior: extract the archive, then flatten the subdirectory.
func FlattenExtractDir(destDir, extractDir string) error {
	srcDir := filepath.Join(destDir, extractDir)
	fi, err := os.Stat(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("extract_dir %q not found in %s", extractDir, destDir)
		}
		return fmt.Errorf("failed to stat extract_dir: %w", err)
	}
	_ = fi

	// Move all entries from srcDir to destDir.
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("failed to read extract_dir: %w", err)
	}

	for _, entry := range entries {
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(destDir, entry.Name())
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("failed to move %s: %w", entry.Name(), err)
		}
	}

	// Remove the now-empty extract_dir.
	return os.Remove(srcDir)
}

// MoveContents moves all files and directories from srcDir into dstDir (creates if needed).
// Used for extract_to: move already-extracted contents into a subdirectory.
func MoveContents(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(dstDir, entry.Name())
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("failed to move %s: %w", entry.Name(), err)
		}
	}

	return nil
}
