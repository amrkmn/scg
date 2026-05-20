package commands

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/app"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/service"
)

// NewVirusTotalCommand creates the virustotal subcommand.
func NewVirusTotalCommand() *cobra.Command {
	var flagAll, flagScan, flagNoDepends bool
	var flagArch string

	cmd := &cobra.Command{
		Use:   "virustotal [* | app...]",
		Short: "Look for app hashes or URLs on VirusTotal",
		Long:  "Look for app download hashes or URLs on VirusTotal using the configured virustotal_api_key.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && args[0] == "*" {
				flagAll = true
				args = nil
			}
			if !flagAll && len(args) == 0 {
				return fmt.Errorf("specify apps to check or use --all")
			}

			ctx := cmdctx.MustFromCmd(cmd)
			apiKey, ok := virustotalAPIKey(ctx.Services.Config)
			if !ok {
				return fmt.Errorf("VirusTotal API key is not configured; run: scg config virustotal_api_key <key>")
			}
			if flagArch == "" {
				flagArch = defaultDownloadArch()
			}

			if flagAll {
				installed, _ := ctx.Services.Apps.ListInstalled("")
				args = args[:0]
				for _, app := range installed {
					args = append(args, app.Name)
				}
			}
			if !flagNoDepends {
				ctx.GetLogger().Verbose("Dependency expansion is not implemented yet; checking requested apps only.")
			}

			client := &http.Client{}
			var unsafe, failed int
			for _, input := range args {
				bad, err := checkVirusTotalApp(ctx, client, apiKey, input, flagArch, flagScan)
				if err != nil {
					failed++
					ctx.GetLogger().Warn(fmt.Sprintf("%s: %v", input, err))
					continue
				}
				if bad {
					unsafe++
				}
			}
			if unsafe > 0 {
				return fmt.Errorf("%d app(s) marked unsafe by VirusTotal", unsafe)
			}
			if failed > 0 {
				return fmt.Errorf("%d app(s) could not be checked", failed)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagAll, "all", "a", false, "Check all installed apps")
	cmd.Flags().BoolVarP(&flagScan, "scan", "s", false, "Submit URLs when VirusTotal has no information")
	cmd.Flags().BoolVarP(&flagNoDepends, "no-depends", "n", false, "Do not check dependencies")
	cmd.Flags().StringVar(&flagArch, "arch", "", "Use the specified architecture (64bit, 32bit, arm64)")
	return cmd
}

func virustotalAPIKey(cfg *service.ConfigService) (string, bool) {
	for _, key := range []string{"virustotal_api_key", "VIRUSTOTAL_API_KEY"} {
		if v, ok := cfg.Get(key); ok {
			if s, ok := v.(string); ok && s != "" {
				return s, true
			}
		}
	}
	return "", false
}

func checkVirusTotalApp(ctx *app.Context, client *http.Client, apiKey, input, arch string, scan bool) (bool, error) {
	_, bucket := ctx.Services.Manifests.FindManifestPair(input)
	if bucket == nil {
		return false, fmt.Errorf("manifest not found")
	}
	urls, err := resolveDownloadURLs(bucket.Manifest, arch)
	if err != nil {
		return false, err
	}
	hashes := resolveDownloadHashes(bucket.Manifest, arch)
	appName := appNameFromInput(input)

	var unsafe bool
	for i, dlURL := range urls {
		hash := strings.TrimSpace(hashAt(hashes, i))
		if hash != "" {
			algo, rawHash := splitVTSupportedHash(hash)
			if rawHash != "" {
				bad, found, err := virustotalFileReport(client, apiKey, rawHash, appName)
				if err != nil {
					return unsafe, err
				}
				if found {
					ctx.GetLogger().Verbose(fmt.Sprintf("%s: checked %s hash %s", appName, algo, rawHash))
					unsafe = unsafe || bad
					continue
				}
			}
		}

		found, err := virustotalURLReport(client, apiKey, dlURL, appName)
		if err != nil {
			return unsafe, err
		}
		if !found {
			if scan {
				if err := virustotalSubmitURL(client, apiKey, dlURL); err != nil {
					return unsafe, err
				}
				ctx.GetLogger().Info(fmt.Sprintf("%s: submitted URL for analysis", appName))
			} else {
				ctx.GetLogger().Warn(fmt.Sprintf("%s: report not found for %s", appName, dlURL))
			}
		}
	}
	return unsafe, nil
}

func splitVTSupportedHash(hash string) (algo, raw string) {
	if before, after, ok := strings.Cut(hash, ":"); ok {
		algo, raw = strings.ToLower(before), after
	} else {
		raw = hash
		switch len(hash) {
		case 32:
			algo = "md5"
		case 40:
			algo = "sha1"
		case 64:
			algo = "sha256"
		}
	}
	if algo != "md5" && algo != "sha1" && algo != "sha256" {
		return algo, ""
	}
	return algo, raw
}

func virustotalFileReport(client *http.Client, apiKey, hash, appName string) (bad bool, found bool, err error) {
	body, status, err := virustotalRequest(client, "GET", "https://www.virustotal.com/api/v3/files/"+hash, apiKey, "")
	if err != nil {
		return false, false, err
	}
	if status == http.StatusNotFound {
		return false, false, nil
	}
	if status < 200 || status >= 300 {
		return false, false, fmt.Errorf("VirusTotal file lookup failed: HTTP %d", status)
	}
	stats := parseVTStats(body)
	unsafe := stats.Malicious + stats.Suspicious
	total := unsafe + stats.Undetected + stats.Harmless
	if unsafe == 0 {
		fmt.Printf("%s: %d/%d, see https://www.virustotal.com/gui/file/%s\n", appName, unsafe, total, hash)
	} else {
		fmt.Printf("%s: %d/%d unsafe, see https://www.virustotal.com/gui/file/%s\n", appName, unsafe, total, hash)
	}
	return unsafe > 0, true, nil
}

func virustotalURLReport(client *http.Client, apiKey, rawURL, appName string) (bool, error) {
	id := base64.RawURLEncoding.EncodeToString([]byte(rawURL))
	_, status, err := virustotalRequest(client, "GET", "https://www.virustotal.com/api/v3/urls/"+id, apiKey, "")
	if err != nil {
		return false, err
	}
	if status == http.StatusNotFound {
		return false, nil
	}
	if status < 200 || status >= 300 {
		return false, fmt.Errorf("VirusTotal URL lookup failed for %s: HTTP %d", appName, status)
	}
	fmt.Printf("%s: URL report found, see https://www.virustotal.com/gui/url/%s\n", appName, id)
	return true, nil
}

func virustotalSubmitURL(client *http.Client, apiKey, rawURL string) error {
	form := url.Values{"url": {rawURL}}
	_, status, err := virustotalRequest(client, "POST", "https://www.virustotal.com/api/v3/urls", apiKey, form.Encode())
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("VirusTotal URL submission failed: HTTP %d", status)
	}
	return nil
}

func virustotalRequest(client *http.Client, method, endpoint, apiKey, body string) ([]byte, int, error) {
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(method, endpoint, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-apikey", apiKey)
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data, resp.StatusCode, nil
}

type vtStats struct {
	Harmless   int `json:"harmless"`
	Malicious  int `json:"malicious"`
	Suspicious int `json:"suspicious"`
	Undetected int `json:"undetected"`
}

func parseVTStats(data []byte) vtStats {
	var raw struct {
		Data struct {
			Attributes struct {
				Stats vtStats `json:"last_analysis_stats"`
			} `json:"attributes"`
		} `json:"data"`
	}
	_ = json.Unmarshal(data, &raw)
	return raw.Data.Attributes.Stats
}
