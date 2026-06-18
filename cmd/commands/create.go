package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
)

type createManifestSkeleton struct {
	Homepage   string `json:"homepage"`
	License    string `json:"license"`
	Version    string `json:"version"`
	URL        string `json:"url"`
	Hash       string `json:"hash"`
	ExtractDir string `json:"extract_dir"`
	Bin        string `json:"bin"`
	Depends    string `json:"depends"`
}

// NewCreateCommand creates the create subcommand.
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <url>",
		Short: "Create a custom app manifest from a URL",
		Long:  "Create a Scoop-compatible manifest skeleton in the current directory from a download URL.",
		Args:  cobra.MaximumNArgs(1),
		Example: `scg create https://example.com/releases/app-1.2.3.zip
scg create https://github.com/foo/bar/releases/download/v1.0.0/tool.zip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			ctx := cmdctx.MustFromCmd(cmd)

			segments, err := parseCreateURLSegments(args[0])
			if err != nil {
				return err
			}

			reader := bufio.NewReader(cmd.InOrStdin())
			writer := cmd.OutOrStdout()

			name, err := chooseCreateValue(reader, writer, segments, "App name")
			if err != nil {
				return err
			}
			if name == "" {
				name = createFilenameStem(segments[len(segments)-1])
			}

			version, err := chooseCreateValue(reader, writer, segments, "Version")
			if err != nil {
				return err
			}

			manifestPath, err := writeCreateManifestFile(".", name, newCreateManifestSkeleton(args[0], version))
			if err != nil {
				return err
			}

			ctx.GetLogger().Success(fmt.Sprintf("Created '%s'.", manifestPath))
			return nil
		},
	}

	return cmd
}

func newCreateManifestSkeleton(rawURL, version string) createManifestSkeleton {
	return createManifestSkeleton{
		Homepage:   "",
		License:    "",
		Version:    version,
		URL:        rawURL,
		Hash:       "",
		ExtractDir: "",
		Bin:        "",
		Depends:    "",
	}
}

func parseCreateURLSegments(rawURL string) ([]string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("%s is not a valid URL", rawURL)
	}

	pathAndQuery := strings.TrimPrefix(parsed.Path, "/")
	if parsed.RawQuery != "" {
		pathAndQuery += "?" + parsed.RawQuery
	}
	if pathAndQuery == "" {
		return nil, fmt.Errorf("%s is not a valid URL", rawURL)
	}

	segments := strings.Split(pathAndQuery, "/")
	filtered := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment != "" {
			filtered = append(filtered, segment)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("%s is not a valid URL", rawURL)
	}

	return filtered, nil
}

func createFilenameStem(segment string) string {
	if idx := strings.LastIndex(segment, "."); idx > 0 {
		return segment[:idx]
	}
	return segment
}

func chooseCreateValue(reader *bufio.Reader, writer io.Writer, options []string, query string) (string, error) {
	for i, item := range options {
		if _, err := fmt.Fprintf(writer, "%d) %s\n", i+1, item); err != nil {
			return "", err
		}
	}
	if _, err := fmt.Fprintf(writer, "%s: ", query); err != nil {
		return "", err
	}

	selection, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	selection = strings.TrimSpace(selection)

	if idx, err := strconv.Atoi(selection); err == nil && idx >= 1 && idx <= len(options) {
		return options[idx-1], nil
	}

	return selection, nil
}

func marshalCreateManifestSkeleton(manifest createManifestSkeleton) ([]byte, error) {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func writeCreateManifestFile(dir, name string, manifest createManifestSkeleton) (string, error) {
	path := filepath.Join(dir, name+".json")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("manifest already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	data, err := marshalCreateManifestSkeleton(manifest)
	if err != nil {
		return "", fmt.Errorf("failed to build manifest: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write manifest: %w", err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return absPath, nil
}
