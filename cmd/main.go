package main

import (
	"fmt"
	"os"

	"github.com/amrkmn/scg/cmd/commands"
)

var Version = "dev"

func main() {
	root := commands.NewRootCommand(Version)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
