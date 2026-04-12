package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"go.noz.one/scg/internal/scoop"
)

type ShortcutDef struct {
	Target string
	Name   string
	Args   string
	Icon   string
}

func ParseShortcutsField(shortcuts []any) []ShortcutDef {
	if len(shortcuts) == 0 {
		return nil
	}
	out := make([]ShortcutDef, 0, len(shortcuts))
	for _, entry := range shortcuts {
		items, ok := entry.([]any)
		if !ok || len(items) < 2 {
			continue
		}
		target, _ := items[0].(string)
		name, _ := items[1].(string)
		if target == "" || name == "" {
			continue
		}
		def := ShortcutDef{Target: target, Name: name}
		if len(items) >= 3 {
			if args, ok := items[2].(string); ok {
				def.Args = args
			}
		}
		if len(items) >= 4 {
			if icon, ok := items[3].(string); ok {
				def.Icon = icon
			}
		}
		out = append(out, def)
	}
	return out
}

func CreateStartMenuShortcuts(defs []ShortcutDef, appCurrentDir string, scope scoop.InstallScope) error {
	if len(defs) == 0 {
		return nil
	}

	programs := startMenuProgramsDir(scope)
	if programs == "" {
		return fmt.Errorf("start menu path not found")
	}
	if err := os.MkdirAll(programs, 0o755); err != nil {
		return err
	}

	psPath := FindPowerShell()
	if psPath == "" {
		return fmt.Errorf("no PowerShell found")
	}

	for _, def := range defs {
		target := filepath.Join(appCurrentDir, filepath.FromSlash(def.Target))
		target = filepath.Clean(target)
		lnkPath := filepath.Join(programs, def.Name+".lnk")
		args := def.Args
		icon := def.Icon
		if icon != "" {
			icon = filepath.Clean(filepath.Join(appCurrentDir, filepath.FromSlash(icon)))
		}

		script := buildShortcutScript(target, lnkPath, args, icon)
		cmd := exec.Command(psPath, "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create shortcut %q: %w\n%s", def.Name, err, out)
		}
	}

	return nil
}

func RemoveStartMenuShortcuts(defs []ShortcutDef, scope scoop.InstallScope) error {
	if len(defs) == 0 {
		return nil
	}

	programs := startMenuProgramsDir(scope)
	if programs == "" {
		return nil
	}

	for _, def := range defs {
		lnkPath := filepath.Join(programs, def.Name+".lnk")
		_ = os.Remove(lnkPath)
	}

	return nil
}

func StartMenuShortcutPath(name string, scope scoop.InstallScope) string {
	programs := startMenuProgramsDir(scope)
	if programs == "" {
		return ""
	}
	return filepath.Join(programs, name+".lnk")
}

func startMenuProgramsDir(scope scoop.InstallScope) string {
	if scope == scoop.ScopeGlobal {
		if pd := os.Getenv("ProgramData"); pd != "" {
			return filepath.Join(pd, "Microsoft", "Windows", "Start Menu", "Programs", "Scoop Apps")
		}
	}
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Scoop Apps")
	}
	profile := os.Getenv("USERPROFILE")
	if profile == "" {
		return ""
	}
	return filepath.Join(profile, "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs", "Scoop Apps")
}

func buildShortcutScript(target, lnkPath, args, icon string) string {
	lines := []string{
		"$ws = New-Object -ComObject WScript.Shell",
		fmt.Sprintf("$s = $ws.CreateShortcut('%s')", escapePS(lnkPath)),
		fmt.Sprintf("$s.TargetPath = '%s'", escapePS(target)),
	}
	if strings.TrimSpace(args) != "" {
		lines = append(lines, fmt.Sprintf("$s.Arguments = '%s'", escapePS(args)))
	}
	if strings.TrimSpace(icon) != "" {
		lines = append(lines, fmt.Sprintf("$s.IconLocation = '%s'", escapePS(icon)))
	}
	lines = append(lines, "$s.Save()")
	return strings.Join(lines, "; ")
}
