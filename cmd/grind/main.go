package main

import (
	"os"

	"github.com/H4ZM47/grind/internal/cli"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	build := cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}

	os.Exit(cli.Run(build, os.Args[1:], os.Stdout, os.Stderr))
}
