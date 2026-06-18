package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/install"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
)

func NewShimCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shim <subcommand> [options] [args]",
		Short: "Manage Scoop shims",
		Long: `Manipulate Scoop shims.

Available subcommands: add, rm, list, info, alter.`,
		Example: `  scg shim list
  scg shim add myapp "C:\Program Files\MyApp\myapp.exe"
  scg shim add myapp "C:\Program Files\MyApp\myapp.exe" -- --flag
  scg shim rm myapp
  scg shim info git
  scg shim alter git`,
	}

	cmd.AddCommand(
		newShimAddCmd(),
		newShimRmCmd(),
		newShimListCmd(),
		newShimInfoCmd(),
		newShimAlterCmd(),
	)

	return cmd
}

func newShimAlterCmd() *cobra.Command {
	var flagGlobal bool

	cmd := &cobra.Command{
		Use:   "alter <name> [source]",
		Short: "Switch a shim to an alternative source",
		Long:  "Switch a shim to one of its saved alternative sources. If source is omitted, the first alternative is used.",
		Example: `  scg shim alter git
  scg shim alter git mingit
  scg shim alter -g python`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			logger := ctx.GetLogger()

			name := args[0]
			paths := scoop.ResolvePaths(scoop.ScopeUser)
			if flagGlobal {
				paths = scoop.ResolvePaths(scoop.ScopeGlobal)
			}

			shimPath := filepath.Join(paths.Shims, name+".shim")
			if _, err := os.Stat(shimPath); err != nil {
				scopeLabel := "local"
				otherScope := scoop.ScopeGlobal
				if flagGlobal {
					scopeLabel = "global"
					otherScope = scoop.ScopeUser
				}
				logger.Error(fmt.Sprintf("%s shim not found: %s", scopeLabel, name))
				otherPaths := scoop.ResolvePaths(otherScope)
				if _, otherErr := os.Stat(filepath.Join(otherPaths.Shims, name+".shim")); otherErr == nil {
					scopeFlag := ""
					if !flagGlobal {
						scopeFlag = " --global"
					}
					logger.Info(fmt.Sprintf("shim exists in the other scope; run 'scg shim alter %s%s'", name, scopeFlag))
				}
				return nil
			}

			currentSource := appFromPath(readShimTarget(shimPath))
			if currentSource == "" {
				currentSource = "External"
			}
			alternatives := findAlternatives(name, paths.Shims, currentSource)
			if len(alternatives) == 0 {
				logger.Skip(name, "no alternatives found")
				return nil
			}

			source := alternatives[0]
			if len(args) == 2 {
				source = args[1]
				found := false
				for _, alt := range alternatives {
					if strings.EqualFold(alt, source) {
						source = alt
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("alternative %q not found for shim %q (available: %s)", source, name, strings.Join(alternatives, ", "))
				}
			}

			if strings.EqualFold(source, currentSource) {
				logger.Skip(name, fmt.Sprintf("already from %s", currentSource))
				return nil
			}

			if err := alterShimSource(paths.Shims, name, currentSource, source); err != nil {
				return err
			}
			logger.Done(name, fmt.Sprintf("using %s as default", source))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Alter a global shim")
	return cmd
}

func newShimAddCmd() *cobra.Command {
	var flagGlobal bool

	cmd := &cobra.Command{
		Use:   "add <name> <command_path> [args...]",
		Short: "Add a custom shim",
		Long: `Add a custom shim that points to an executable.

If command_path contains no path separator, it is resolved as an existing shim
name or via PATH lookup. Arguments after '--' are passed as default args.`,
		Example: `  scg shim add myapp "C:\Program Files\MyApp\myapp.exe"
  scg shim add myapp "C:\Program Files\MyApp\myapp.exe" -- --verbose
  scg shim add -g myapp "C:\Program Files\MyApp\myapp.exe"`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			logger := ctx.GetLogger()

			name := args[0]
			commandPath := args[1]
			commandArgs := ""

			if len(args) > 2 {
				commandArgs = strings.Join(args[2:], " ")
			}

			scope := scoop.ScopeUser
			if flagGlobal {
				scope = scoop.ScopeGlobal
			}

			resolvedPath := commandPath
			if !strings.ContainsAny(commandPath, `/\`) {
				var err error
				resolvedPath, err = resolveShimOrPath(commandPath, flagGlobal)
				if err != nil {
					logger.Error(fmt.Sprintf("command path does not exist: %s", commandPath))
					return err
				}
			}

			if _, err := os.Stat(resolvedPath); err != nil {
				logger.Error(fmt.Sprintf("command path does not exist: %s", commandPath))
				return err
			}

			def := install.ShimDef{
				Target: resolvedPath,
				Name:   name,
				Args:   commandArgs,
			}

			if err := install.CreateShims([]install.ShimDef{def}, filepath.Dir(resolvedPath), scope); err != nil {
				return fmt.Errorf("failed to add shim: %w", err)
			}

			scopeLabel := "local"
			if flagGlobal {
				scopeLabel = "global"
			}
			logger.Header("Adding shim")
			logger.Detail(fmt.Sprintf("%s -> %s", name, resolvedPath))
			logger.Done("shim", fmt.Sprintf("added %s %s", scopeLabel, name))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Add a global shim")
	return cmd
}

func newShimRmCmd() *cobra.Command {
	var flagGlobal bool

	cmd := &cobra.Command{
		Use:   "rm <name> [name...]",
		Short: "Remove shim(s)",
		Long:  "Remove one or more shims. Caution: this could remove shims added by an app manifest.",
		Example: `  scg shim rm myapp
  scg shim rm -g myapp`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			logger := ctx.GetLogger()

			scope := scoop.ScopeUser
			if flagGlobal {
				scope = scoop.ScopeGlobal
			}
			paths := scoop.ResolvePaths(scope)

			var failed []string
			logger.Header("Removing shims")
			for _, name := range args {
				shimPath := filepath.Join(paths.Shims, name+".shim")
				exePath := filepath.Join(paths.Shims, name+".exe")
				cmdPath := filepath.Join(paths.Shims, name+".cmd")
				ps1Path := filepath.Join(paths.Shims, name+".ps1")

				found := false
				for _, p := range []string{shimPath, exePath, cmdPath, ps1Path} {
					if _, err := os.Stat(p); err == nil {
						found = true
						_ = os.Remove(p)
					}
				}

				if !found {
					failed = append(failed, name)
				} else {
					logger.Done("shim", name)
				}
			}

			if len(failed) > 0 {
				scopeLabel := "local"
				if flagGlobal {
					scopeLabel = "global"
				}
				for _, name := range failed {
					logger.Error(fmt.Sprintf("%s shim not found: %s", scopeLabel, name))
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Remove a global shim")
	return cmd
}

func newShimListCmd() *cobra.Command {
	var flagGlobal bool

	cmd := &cobra.Command{
		Use:   "list [pattern...]",
		Short: "List shims",
		Long:  "List all shims, optionally filtered by pattern(s).",
		Example: `  scg shim list
  scg shim list git
  scg shim list -g`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			logger := ctx.GetLogger()

			var patterns []*regexp.Regexp
			for _, p := range args {
				re, err := regexp.Compile(p)
				if err != nil {
					logger.Error(fmt.Sprintf("invalid pattern: %s", p))
					return err
				}
				patterns = append(patterns, re)
			}

			var shims []shimEntry

			if !flagGlobal {
				userPaths := scoop.ResolvePaths(scoop.ScopeUser)
				shims = append(shims, scanShimDir(userPaths.Shims, false, patterns)...)
			}

			globalPaths := scoop.ResolvePaths(scoop.ScopeGlobal)
			if _, err := os.Stat(globalPaths.Shims); err == nil {
				shims = append(shims, scanShimDir(globalPaths.Shims, true, patterns)...)
			}

			if len(shims) == 0 {
				logger.Skip("shims", "none found")
				return nil
			}

			logger.Header("Shims")
			sort.Slice(shims, func(i, j int) bool {
				return shims[i].Name < shims[j].Name
			})

			rows := make([][]string, 0, len(shims))

			for _, s := range shims {
				source := s.Source
				if source == "" {
					source = ui.Dim("unknown")
				}
				scopeTag := ""
				if s.Global {
					scopeTag = " " + ui.Dim("[global]")
				}
				rows = append(rows, []string{ui.BoldCyan(s.Name), source, s.Path + scopeTag})
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderTable([]string{"Name", "Source", "Path"}, rows, []float64{0.25, 0.2, 0.55}, fmt.Sprintf("%d shim(s)", len(shims))))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "List only global shims")
	return cmd
}

func newShimInfoCmd() *cobra.Command {
	var flagGlobal bool

	cmd := &cobra.Command{
		Use:   "info <name>",
		Short: "Show shim information",
		Long:  "Show detailed information about a shim.",
		Example: `  scg shim info git
  scg shim info -g python`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			logger := ctx.GetLogger()

			name := args[0]
			paths := scoop.ResolvePaths(scoop.ScopeUser)
			if flagGlobal {
				paths = scoop.ResolvePaths(scoop.ScopeGlobal)
			}

			shimPath := filepath.Join(paths.Shims, name+".shim")
			exePath := filepath.Join(paths.Shims, name+".exe")

			shimData, shimErr := os.ReadFile(shimPath)
			if shimErr != nil && flagGlobal {
				logger.Error(fmt.Sprintf("global shim not found: %s", name))
				return nil
			} else if shimErr != nil {
				globalPaths := scoop.ResolvePaths(scoop.ScopeGlobal)
				globalShim := filepath.Join(globalPaths.Shims, name+".shim")
				if _, err := os.ReadFile(globalShim); err == nil {
					logger.Info(fmt.Sprintf("%s not found in local shims, but a global shim exists", name))
					logger.Detail(fmt.Sprintf("run 'scg shim info %s --global' to show its info", name))
					return nil
				}
				logger.Error(fmt.Sprintf("local shim not found: %s", name))
				return nil
			}

			scopeLabel := "local"
			if flagGlobal {
				scopeLabel = "global"
			}

			targetPath := ""
			args2 := ""
			for _, line := range strings.Split(string(shimData), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(strings.ToLower(line), "path") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						targetPath = strings.Trim(strings.TrimSpace(parts[1]), `"`)
					}
				} else if strings.HasPrefix(strings.ToLower(line), "args") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						args2 = strings.Trim(strings.TrimSpace(parts[1]), `"`)
					}
				}
			}

			source := appFromPath(targetPath)

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderKeyValueBlock("Shim "+name, buildShimInfoPairs(name, targetPath, args2, source, scopeLabel, exePath, findAlternatives(name, paths.Shims, source))))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Show global shim info")
	return cmd
}

type shimEntry struct {
	Name   string
	Source string
	Path   string
	Global bool
}

func scanShimDir(dir string, isGlobal bool, patterns []*regexp.Regexp) []shimEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var result []shimEntry
	for _, e := range entries {
		name := e.Name()
		ext := filepath.Ext(name)
		if ext != ".shim" && ext != ".ps1" {
			continue
		}

		baseName := strings.TrimSuffix(name, ext)

		if len(patterns) > 0 {
			match := false
			for _, p := range patterns {
				if p.MatchString(baseName) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		shimFilePath := filepath.Join(dir, name)
		target := ""
		if ext == ".shim" {
			data, err := os.ReadFile(shimFilePath)
			if err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(strings.ToLower(line), "path") {
						parts := strings.SplitN(line, "=", 2)
						if len(parts) == 2 {
							target = strings.Trim(strings.TrimSpace(parts[1]), `"`)
						}
						break
					}
				}
			}
		}

		source := appFromPath(target)
		if ext == ".ps1" {
			source = "ExternalScript"
		}
		if source == "" {
			source = "External"
		}

		result = append(result, shimEntry{
			Name:   baseName,
			Source: source,
			Path:   target,
			Global: isGlobal,
		})
	}

	return result
}

func appFromPath(target string) string {
	if target == "" {
		return ""
	}
	parts := strings.Split(filepath.Clean(target), string(filepath.Separator))
	for i := 0; i+2 < len(parts); i++ {
		if strings.EqualFold(parts[i], "apps") {
			return parts[i+1]
		}
	}
	return ""
}

func buildShimInfoPairs(name, targetPath, args, source, scopeLabel, exePath string, alternatives []string) []ui.KeyValue {
	pairs := []ui.KeyValue{
		{Key: "Name", Value: name},
		{Key: "Path", Value: targetPath},
		{Key: "Source", Value: source},
		{Key: "Scope", Value: scopeLabel},
	}
	if args != "" {
		pairs = append(pairs[:2], append([]ui.KeyValue{{Key: "Args", Value: args}}, pairs[2:]...)...)
	}
	if exePath != "" {
		if _, err := os.Stat(exePath); err == nil {
			pairs = append(pairs, ui.KeyValue{Key: "Shim exe", Value: exePath})
		}
	}
	if len(alternatives) > 0 {
		pairs = append(pairs, ui.KeyValue{Key: "Alternatives", Value: strings.Join(alternatives, ", ")})
	}
	return pairs
}

func findAlternatives(name, shimDir, currentSource string) []string {
	var alts []string
	entries, err := os.ReadDir(shimDir)
	if err != nil {
		return nil
	}
	prefix := name + ".shim."
	for _, e := range entries {
		fn := e.Name()
		if strings.HasPrefix(fn, prefix) {
			alt := strings.TrimPrefix(fn, prefix)
			if alt != currentSource && alt != "" {
				alts = append(alts, alt)
			}
		}
	}
	sort.Strings(alts)
	return alts
}

func readShimTarget(shimPath string) string {
	data, err := os.ReadFile(shimPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "path") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.Trim(strings.TrimSpace(parts[1]), `"`)
			}
		}
	}
	return ""
}

func alterShimSource(shimDir, name, currentSource, newSource string) error {
	base := filepath.Join(shimDir, name)
	for _, ext := range []string{".shim", ".exe", ".cmd", ".ps1"} {
		current := base + ext
		if _, err := os.Stat(current); err != nil {
			continue
		}

		oldAlt := current + "." + currentSource
		newAlt := current + "." + newSource
		if _, err := os.Stat(newAlt); err != nil {
			if ext == ".shim" {
				return fmt.Errorf("alternative %q is missing %s", newSource, filepath.Base(newAlt))
			}
			continue
		}

		_ = os.Remove(oldAlt)
		if err := os.Rename(current, oldAlt); err != nil {
			return fmt.Errorf("failed to save current shim %s: %w", filepath.Base(current), err)
		}
		if err := os.Rename(newAlt, current); err != nil {
			_ = os.Rename(oldAlt, current)
			return fmt.Errorf("failed to activate alternative %s: %w", filepath.Base(newAlt), err)
		}
	}
	return nil
}

func resolveShimOrPath(name string, global bool) (string, error) {
	for _, scope := range []scoop.InstallScope{scoop.ScopeUser, scoop.ScopeGlobal} {
		if global && scope != scoop.ScopeGlobal {
			continue
		}
		paths := scoop.ResolvePaths(scope)
		shimFile := filepath.Join(paths.Shims, name+".shim")
		if data, err := os.ReadFile(shimFile); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(strings.ToLower(line), "path") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						target := strings.Trim(strings.TrimSpace(parts[1]), `"`)
						if _, err := os.Stat(target); err == nil {
							return target, nil
						}
					}
				}
			}
		}
	}

	if found, err := execLookPath(name); err == nil {
		return found, nil
	}

	return "", fmt.Errorf("command not found: %s", name)
}

func execLookPath(name string) (string, error) {
	var tried []string
	for _, ext := range []string{".exe", ".cmd", ".bat"} {
		if found, err := findInPath(name + ext); err == nil {
			return found, nil
		}
		tried = append(tried, name+ext)
	}
	return "", fmt.Errorf("not found in PATH: %s", strings.Join(tried, ", "))
}

func findInPath(name string) (string, error) {
	pathDirs := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	for _, dir := range pathDirs {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return filepath.Abs(candidate)
		}
	}
	return "", fmt.Errorf("not found: %s", name)
}
