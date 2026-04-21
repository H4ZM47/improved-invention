package main

import (
	"fmt"
	"os"

	"github.com/H4ZM47/improved-invention/internal/cli"
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

	if err := cli.Execute(build); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
