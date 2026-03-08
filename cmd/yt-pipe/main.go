package main

import (
	"os"

	"github.com/jay/youtube-pipeline/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
