package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/install"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
)

type checkResult struct {
	ok     bool
	label  string
	detail string
}

func NewCheckupCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "checkup",
		Short:   "Check for potential Scoop environment problems",
		Long:    "Performs a series of diagnostic tests to identify things that may cause problems with Scoop.",
		Example: "  scg checkup",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmdctx.MustFromCmd(cmd)

			var results []checkResult

			results = append(results, checkScoopDirs()...)
			results = append(results, checkGit()...)
			results = append(results, check7zip()...)
			results = append(results, checkInnounp()...)
			results = append(results, checkLessmsi()...)
			results = append(results, checkDark()...)
			results = append(results, checkLongPaths()...)
			results = append(results, checkDeveloperMode()...)
			results = append(results, checkNTFS()...)
			results = append(results, checkDefenderExclusion()...)
			results = append(results, checkMainBucket()...)

			issues := 0
			for _, r := range results {
				if !r.ok {
					issues++
				}
			}

			renderCheckupOutput(cmd.OutOrStdout(), results, issues)

			return nil
		},
	}
}

func renderCheckupOutput(w io.Writer, results []checkResult, issues int) {
	_, _ = fmt.Fprintln(w, ui.Heading("Checking Scoop environment"))
	for _, r := range results {
		kind := ui.StatusDone
		line := ui.StatusWithOptions(kind, r.label, "", ui.StatusOptions{})
		if !r.ok {
			line = ui.StatusWithOptions(ui.StatusFail, r.label, "", ui.StatusOptions{})
		}
		_, _ = fmt.Fprintln(w, line)
		if !r.ok && r.detail != "" {
			_, _ = fmt.Fprintln(w, ui.Detail(ui.Dim("  "+r.detail)))
		}
	}
	_, _ = fmt.Fprintln(w)

	summaryLine := ui.Done("checkup", "no problems identified")
	if issues > 0 {
		summaryLine = ui.WarnLine(fmt.Sprintf("found %d potential %s", issues, pluralize(issues, "problem")))
	}
	_, _ = fmt.Fprintln(w, ui.RenderSummary(summaryLine))
}

func checkScoopDirs() []checkResult {
	var results []checkResult

	for _, scope := range []scoop.InstallScope{scoop.ScopeUser, scoop.ScopeGlobal} {
		paths := scoop.ResolvePaths(scope)
		if paths.Root == "" {
			continue
		}

		if _, err := os.Stat(paths.Root); err != nil {
			results = append(results, checkResult{
				ok:     false,
				label:  fmt.Sprintf("%s Scoop root directory exists", scope),
				detail: fmt.Sprintf("Directory %s does not exist", paths.Root),
			})
		} else {
			results = append(results, checkResult{
				ok:    true,
				label: fmt.Sprintf("%s Scoop root directory exists", scope),
			})
		}

		if paths.Shims != "" {
			pathDirs := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
			found := false
			for _, d := range pathDirs {
				if strings.EqualFold(d, paths.Shims) {
					found = true
					break
				}
			}
			scopeLabel := "User"
			if scope == scoop.ScopeGlobal {
				scopeLabel = "Global"
			}
			if found {
				results = append(results, checkResult{
					ok:    true,
					label: fmt.Sprintf("%s shims directory in PATH", scopeLabel),
				})
			} else {
				results = append(results, checkResult{
					ok:     false,
					label:  fmt.Sprintf("%s shims directory in PATH", scopeLabel),
					detail: fmt.Sprintf("Add %s to your PATH", paths.Shims),
				})
			}
		}
	}

	return results
}

func checkGit() []checkResult {
	path, err := exec.LookPath("git.exe")
	if err != nil {
		return []checkResult{
			{ok: false, label: "Git is installed", detail: "Git is required for Scoop. Install it with: scg install git"},
		}
	}
	return []checkResult{{ok: true, label: fmt.Sprintf("Git is installed (%s)", path)}}
}

func check7zip() []checkResult {
	if install.HelperAvailable("7zip", "7z.exe") {
		return []checkResult{{ok: true, label: "7-Zip is available"}}
	}
	if _, err := exec.LookPath("7z.exe"); err == nil {
		return []checkResult{{ok: true, label: "7-Zip (external) is available"}}
	}
	return []checkResult{
		{ok: false, label: "7-Zip is available", detail: "7-Zip is required for unpacking most programs. Run: scg install 7zip"},
	}
}

func checkInnounp() []checkResult {
	if install.HelperAvailable("innounp", "innounp.exe") || install.HelperAvailable("innounp-unicode", "innounp.exe") {
		return []checkResult{{ok: true, label: "Inno Setup Unpacker is available"}}
	}
	return []checkResult{
		{ok: false, label: "Inno Setup Unpacker is available", detail: "Required for InnoSetup installers. Run: scg install innounp"},
	}
}

func checkLessmsi() []checkResult {
	if install.HelperAvailable("lessmsi", "lessmsi.exe") {
		return []checkResult{{ok: true, label: "Lessmsi is available"}}
	}
	return []checkResult{{ok: true, label: "Lessmsi (optional, not installed)"}}
}

func checkDeveloperMode() []checkResult {
	if isDeveloperModeEnabled() {
		return []checkResult{{ok: true, label: "Windows Developer Mode is enabled"}}
	}
	return []checkResult{
		{ok: false, label: "Windows Developer Mode is enabled", detail: "Required for symlinks without elevation. Enable in Settings > For developers, or: reg add HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\AppModelUnlock /v AllowDevelopmentWithoutDevLicense /t REG_DWORD /d 1 /f"},
	}
}

func isDeveloperModeEnabled() bool {
	psPath := install.FindPowerShell()
	if psPath == "" {
		return false
	}
	script := `(Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock").AllowDevelopmentWithoutDevLicense`
	out, err := exec.Command(psPath, "-NoProfile", "-Command", script).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "1"
}

func checkNTFS() []checkResult {
	var results []checkResult
	for _, scope := range []scoop.InstallScope{scoop.ScopeUser, scoop.ScopeGlobal} {
		paths := scoop.ResolvePaths(scope)
		if paths.Root == "" {
			continue
		}
		if _, err := os.Stat(paths.Root); err != nil {
			continue
		}
		volume := filepath.VolumeName(paths.Root)
		if volume == "" {
			continue
		}
		if isNTFSVolume(volume) {
			results = append(results, checkResult{ok: true, label: fmt.Sprintf("%s scoop directory is on NTFS volume %s", scope, volume)})
		} else {
			results = append(results, checkResult{ok: false, label: fmt.Sprintf("%s scoop directory is on NTFS volume %s", scope, volume), detail: "Scoop requires NTFS. Non-NTFS volumes may cause issues with symlinks and junctions."})
		}
	}
	return results
}

func isNTFSVolume(volume string) bool {
	psPath := install.FindPowerShell()
	if psPath == "" {
		return true
	}
	script := fmt.Sprintf(`(Get-Volume -FilePath '%s').FileSystemType.ToString()`, volume)
	out, err := exec.Command(psPath, "-NoProfile", "-Command", script).Output()
	if err != nil {
		return true
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(string(out))), "ntfs")
}

func checkDefenderExclusion() []checkResult {
	if !isAdmin() {
		return nil
	}
	for _, scope := range []scoop.InstallScope{scoop.ScopeUser, scoop.ScopeGlobal} {
		paths := scoop.ResolvePaths(scope)
		if paths.Root == "" {
			continue
		}
		if _, err := os.Stat(paths.Root); err != nil {
			continue
		}
		if isDefenderExcluded(paths.Root) {
			return []checkResult{{ok: true, label: fmt.Sprintf("%s scoop directory excluded from Windows Defender", scope)}}
		}
		return []checkResult{
			{ok: false, label: fmt.Sprintf("%s scoop directory excluded from Windows Defender", scope), detail: fmt.Sprintf("Consider adding %s to Windows Defender exclusions for better performance. Run as admin: Add-MpPreference -ExclusionPath '%s'", paths.Root, paths.Root)},
		}
	}
	return nil
}

func isAdmin() bool {
	psPath := install.FindPowerShell()
	if psPath == "" {
		return false
	}
	out, err := exec.Command(psPath, "-NoProfile", "-Command", "([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "True"
}

func isDefenderExcluded(path string) bool {
	psPath := install.FindPowerShell()
	if psPath == "" {
		return true
	}
	escaped := strings.ReplaceAll(path, "'", "''")
	script := fmt.Sprintf(`$p = '%s'; (Get-MpPreference).ExclusionPath | Where-Object { $_ -ieq $p }`, escaped)
	out, err := exec.Command(psPath, "-NoProfile", "-Command", script).Output()
	if err != nil {
		return true
	}
	return strings.TrimSpace(string(out)) != ""
}

func checkDark() []checkResult {
	if install.HelperAvailable("dark", "dark.exe") {
		return []checkResult{{ok: true, label: "Dark is available"}}
	}
	return []checkResult{
		{ok: false, label: "Dark is available", detail: "Required for WiX installers. Run: scg install dark"},
	}
}

func checkLongPaths() []checkResult {
	if !isLongPathsEnabled() {
		return []checkResult{
			{ok: false, label: "Long paths are enabled", detail: "Enable with: reg add HKLM\\SYSTEM\\CurrentControlSet\\Control\\FileSystem /v LongPathsEnabled /t REG_DWORD /d 1 /f"},
		}
	}
	return []checkResult{{ok: true, label: "Long paths are enabled"}}
}

func checkMainBucket() []checkResult {
	for _, paths := range scoop.BothScopes() {
		mainDir := filepath.Join(paths.Buckets, "main")
		if _, err := os.Stat(mainDir); err == nil {
			return []checkResult{{ok: true, label: "Main bucket is installed"}}
		}
	}
	return []checkResult{
		{ok: false, label: "Main bucket is installed", detail: "The main bucket provides essential packages. Run: scg bucket add main"},
	}
}

func isLongPathsEnabled() bool {
	psPath := install.FindPowerShell()
	if psPath == "" {
		return false
	}
	script := `(Get-ItemProperty "HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem").LongPathsEnabled`
	out, err := exec.Command(psPath, "-NoProfile", "-Command", script).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "1"
}

func pluralize(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
