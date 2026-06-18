package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/app"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
)

func NewDependsCommand() *cobra.Command {
	var flagArch string

	cmd := &cobra.Command{
		Use:     "depends <app>",
		Short:   "List dependencies for an app",
		Long:    "Shows the dependency tree for an app, in the order they would be installed.",
		Example: "  scg depends git\n  scg depends --arch 32bit python",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			appName := args[0]

			_ = detectArch(flagArch)

			deps, err := resolveDependencies(ctx, appName)
			if err != nil {
				ctx.GetLogger().Error(err.Error())
				return err
			}

			if len(deps) == 0 {
				ctx.GetLogger().Skip(appName, "no dependencies")
				return nil
			}

			rows := make([][]string, 0, len(deps))
			for _, dep := range deps {
				value := dep.Name
				if dep.Source != "" && dep.Source != "missing" {
					value = dep.Source + "/" + dep.Name
				}
				rows = append(rows, []string{ui.BoldCyan(value)})
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderTable([]string{"Dependency"}, rows, []float64{1.0}, fmt.Sprintf("%d dependenc%s", len(deps), pluralizeDepends(len(deps)))))
			return nil
		},
	}

	cmd.Flags().StringVarP(&flagArch, "arch", "a", "", "Architecture to use (64bit, 32bit, arm64)")
	return cmd
}

type depNode struct {
	Name   string
	Source string
}

func resolveDependencies(ctx *app.Context, appName string) ([]depNode, error) {
	manifestSvc := ctx.Services.Manifests

	installed, bucket := manifestSvc.FindManifestPair(appName)
	var m *scoop.Manifest

	if bucket != nil {
		m = bucket.Manifest
	} else if installed != nil {
		m = installed.Manifest
	} else {
		return nil, fmt.Errorf("couldn't find manifest for '%s'", appName)
	}

	deps := scoop.GetDependencies(m.Depends)
	if len(deps) == 0 {
		return nil, nil
	}

	var resolved []depNode
	visited := make(map[string]bool)
	inProgress := make(map[string]bool)

	var resolve func(name string) error
	resolve = func(name string) error {
		if visited[name] {
			return nil
		}
		if inProgress[name] {
			return fmt.Errorf("circular dependency detected: %s", name)
		}
		inProgress[name] = true

		depInstalled, depBucket := manifestSvc.FindManifestPair(name)

		var depSource string
		if depBucket != nil {
			depSource = depBucket.Bucket
		} else if depInstalled != nil {
			depSource = depInstalled.Bucket
		} else {
			depSource = "missing"
		}

		if depBucket != nil {
			subDeps := scoop.GetDependencies(depBucket.Manifest.Depends)
			for _, sd := range subDeps {
				if err := resolve(sd); err != nil {
					return err
				}
			}
		} else if depInstalled != nil {
			subDeps := scoop.GetDependencies(depInstalled.Manifest.Depends)
			for _, sd := range subDeps {
				if err := resolve(sd); err != nil {
					return err
				}
			}
		}

		visited[name] = true
		delete(inProgress, name)
		resolved = append(resolved, depNode{Name: name, Source: depSource})
		return nil
	}

	for _, dep := range deps {
		if err := resolve(dep); err != nil {
			return nil, err
		}
	}

	return resolved, nil
}

func detectArch(archFlag string) string {
	if archFlag != "" {
		a := strings.ToLower(archFlag)
		switch a {
		case "64bit", "64", "x64", "amd64":
			return "64bit"
		case "32bit", "32", "x86", "i386":
			return "32bit"
		case "arm64", "aarch64":
			return "arm64"
		default:
			return a
		}
	}
	return "64bit"
}

func pluralizeDepends(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
