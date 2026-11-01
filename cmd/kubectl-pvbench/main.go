package main

import (
	"os"

	"github.com/sdmoradi/pvbench/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
