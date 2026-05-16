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

Available subcommands: add, rm, list, info.`,
		Example: `  scg shim list
  scg shim add myapp "C:\Program Files\MyApp\myapp.exe"
  scg shim add myapp "C:\Program Files\MyApp\myapp.exe" -- --flag
  scg shim rm myapp
  scg shim info git`,
	}

	cmd.AddCommand(
		newShimAddCmd(),
		newShimRmCmd(),
		newShimListCmd(),
		newShimInfoCmd(),
	)

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
			_ = ctx

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
					_, _ = fmt.Fprintf(os.Stderr, "%s Command path does not exist: %s\n", ui.Error("-"), commandPath)
					return err
				}
			}

			if _, err := os.Stat(resolvedPath); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%s Command path does not exist: %s\n", ui.Error("-"), commandPath)
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
			fmt.Printf("%s Added %s shim %s -> %s\n", ui.Success("+"), scopeLabel, ui.Cyan(name), resolvedPath)
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
			_ = ctx

			scope := scoop.ScopeUser
			if flagGlobal {
				scope = scoop.ScopeGlobal
			}
			paths := scoop.ResolvePaths(scope)

			var failed []string
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
					fmt.Printf("%s Removed shim %s\n", ui.Success("-"), ui.Cyan(name))
				}
			}

			if len(failed) > 0 {
				scopeLabel := "local"
				if flagGlobal {
					scopeLabel = "global"
				}
				for _, name := range failed {
					_, _ = fmt.Fprintf(os.Stderr, "%s %s shim not found: %s\n", ui.Error("-"), scopeLabel, name)
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
			_ = ctx

			var patterns []*regexp.Regexp
			for _, p := range args {
				re, err := regexp.Compile(p)
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "%s Invalid pattern: %s\n", ui.Error("-"), p)
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
				fmt.Println("No shims found.")
				return nil
			}

			sort.Slice(shims, func(i, j int) bool {
				return shims[i].Name < shims[j].Name
			})

			maxName := 0
			for _, s := range shims {
				if len(s.Name) > maxName {
					maxName = len(s.Name)
				}
			}

			_, _ = fmt.Fprintf(os.Stdout, "%s  %s  %s\n",
				ui.PadRight(ui.BoldGreen("Name"), maxName),
				ui.BoldGreen("Source"),
				ui.BoldGreen("Path"))

			for _, s := range shims {
				source := s.Source
				if source == "" {
					source = ui.Dim("unknown")
				}
				scopeTag := ""
				if s.Global {
					scopeTag = " " + ui.Dim("[global]")
				}
				_, _ = fmt.Fprintf(os.Stdout, "%s  %s  %s%s\n",
					ui.PadRight(ui.Cyan(s.Name), maxName),
					source,
					s.Path,
					scopeTag)
			}

			fmt.Printf("\n%s shim(s)\n", ui.Dim(fmt.Sprintf("%d", len(shims))))
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
			_ = ctx

			name := args[0]
			paths := scoop.ResolvePaths(scoop.ScopeUser)
			if flagGlobal {
				paths = scoop.ResolvePaths(scoop.ScopeGlobal)
			}

			shimPath := filepath.Join(paths.Shims, name+".shim")
			exePath := filepath.Join(paths.Shims, name+".exe")

			shimData, shimErr := os.ReadFile(shimPath)
			if shimErr != nil && flagGlobal {
				_, _ = fmt.Fprintf(os.Stderr, "%s Global shim not found: %s\n", ui.Error("-"), name)
				return nil
			} else if shimErr != nil {
				globalPaths := scoop.ResolvePaths(scoop.ScopeGlobal)
				globalShim := filepath.Join(globalPaths.Shims, name+".shim")
				if _, err := os.ReadFile(globalShim); err == nil {
					_, _ = fmt.Fprintf(os.Stdout, "%s not found in local shims, but a global shim exists.\n", name)
					_, _ = fmt.Fprintf(os.Stdout, "Run 'scg shim info %s --global' to show its info.\n", name)
					return nil
				}
				_, _ = fmt.Fprintf(os.Stderr, "%s Local shim not found: %s\n", ui.Error("-"), name)
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

			fmt.Printf("Name:      %s\n", name)
			fmt.Printf("Path:      %s\n", targetPath)
			if args2 != "" {
				fmt.Printf("Args:      %s\n", args2)
			}
			fmt.Printf("Source:    %s\n", source)
			fmt.Printf("Scope:     %s\n", scopeLabel)
			if _, err := os.Stat(exePath); err == nil {
				fmt.Printf("Shim exe:  %s\n", exePath)
			}

			alternatives := findAlternatives(name, paths.Shims, source)
			if len(alternatives) > 0 {
				fmt.Printf("Alternatives: %s\n", strings.Join(alternatives, ", "))
			}

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

func findAlternatives(name, shimDir, currentSource string) []string {
	var alts []string
	entries, err := os.ReadDir(shimDir)
	if err != nil {
		return nil
	}
	prefix := name + "."
	for _, e := range entries {
		fn := e.Name()
		if strings.HasPrefix(fn, prefix) && fn != name+".shim" && fn != name+".exe" && fn != name+".cmd" && fn != name+".ps1" {
			alt := strings.TrimPrefix(fn, prefix)
			if alt != currentSource && alt != "" {
				alts = append(alts, alt)
			}
		}
	}
	return alts
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