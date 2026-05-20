package main

import (
	"fmt"
	"os"

	"go.noz.one/scg/cmd/commands"
)

var Version = "dev"

func main() {
	root := commands.NewRootCommand(Version)
	if expanded, ok, err := commands.ExpandAliasArgs(root, Version, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	} else if ok {
		root.SetArgs(expanded)
	}
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
