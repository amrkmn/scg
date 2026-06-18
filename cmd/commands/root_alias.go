package commands

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/app"
	"go.noz.one/scg/internal/scoop"
)

// ExpandAliasArgs rewrites root-level alias invocations before cobra parses commands.
func ExpandAliasArgs(root *cobra.Command, version string, args []string) ([]string, bool, error) {
	aliasIndex := firstCommandArg(args)
	if aliasIndex < 0 {
		return nil, false, nil
	}

	name := args[aliasIndex]
	if commandExists(root, name) {
		return nil, false, nil
	}

	appCtx := app.NewContext(version, hasVerboseFlag(args), false, false)
	aliases, _, err := loadAliases(appCtx.Services.Config)
	if err != nil {
		return nil, false, err
	}
	if _, ok := aliases[name]; !ok {
		return nil, false, nil
	}

	paths := scoop.ResolvePaths(scoop.ScopeUser)
	command, _ := readAliasScript(filepath.Join(paths.Shims, aliasScriptName(aliases, name)+".ps1"))
	if command == "" {
		return nil, false, fmt.Errorf("alias %q is broken; missing command script", name)
	}

	commandArgs, err := splitAliasCommand(command)
	if err != nil {
		return nil, false, fmt.Errorf("alias %q command is invalid: %w", name, err)
	}
	commandArgs = trimAliasExecutable(commandArgs)
	if len(commandArgs) == 0 {
		return nil, false, fmt.Errorf("alias %q command is empty", name)
	}

	forwarded := args[aliasIndex+1:]
	expanded, err := expandAliasPlaceholders(commandArgs, forwarded)
	if err != nil {
		return nil, false, fmt.Errorf("alias %q: %w", name, err)
	}

	newArgs := make([]string, 0, len(args)+len(expanded))
	newArgs = append(newArgs, args[:aliasIndex]...)
	newArgs = append(newArgs, expanded...)
	return newArgs, true, nil
}

func firstCommandArg(args []string) int {
	for i := range args {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				return i + 1
			}
			return -1
		}
		if arg == "-h" || arg == "--help" {
			return -1
		}
		if strings.HasPrefix(arg, "--") {
			continue
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			continue
		}
		return i
	}
	return -1
}

func commandExists(root *cobra.Command, name string) bool {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return true
		}
		for _, alias := range cmd.Aliases {
			if alias == name {
				return true
			}
		}
	}
	return false
}

func hasVerboseFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-v" || arg == "--verbose" {
			return true
		}
	}
	return false
}

func splitAliasCommand(command string) ([]string, error) {
	var args []string
	var b strings.Builder
	var quote rune
	escaped := false
	inToken := false

	for _, r := range command {
		if escaped {
			b.WriteRune(r)
			escaped = false
			inToken = true
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
			inToken = true
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			inToken = true
			continue
		}
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			if inToken {
				args = append(args, b.String())
				b.Reset()
				inToken = false
			}
			continue
		}
		b.WriteRune(r)
		inToken = true
	}
	if escaped {
		b.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	if inToken {
		args = append(args, b.String())
	}
	return args, nil
}

func trimAliasExecutable(args []string) []string {
	if len(args) == 0 {
		return args
	}
	name := strings.ToLower(filepath.Base(args[0]))
	name = strings.TrimSuffix(strings.TrimSuffix(name, ".exe"), ".ps1")
	if name == "scg" || name == "scoop" {
		return args[1:]
	}
	return args
}

func expandAliasPlaceholders(args, forwarded []string) ([]string, error) {
	usedForwarded := false
	expanded := make([]string, 0, len(args)+len(forwarded))
	for _, arg := range args {
		if arg == "$args" || arg == "$args[*]" {
			expanded = append(expanded, forwarded...)
			usedForwarded = true
			continue
		}
		if strings.HasPrefix(arg, "$args[") && strings.HasSuffix(arg, "]") {
			idxText := strings.TrimSuffix(strings.TrimPrefix(arg, "$args["), "]")
			idx, err := strconv.Atoi(idxText)
			if err != nil || idx < 0 {
				return nil, fmt.Errorf("invalid placeholder %s", arg)
			}
			if idx >= len(forwarded) {
				return nil, fmt.Errorf("missing argument for placeholder %s", arg)
			}
			expanded = append(expanded, forwarded[idx])
			usedForwarded = true
			continue
		}
		expanded = append(expanded, arg)
	}
	if !usedForwarded {
		expanded = append(expanded, forwarded...)
	}
	return expanded, nil
}
