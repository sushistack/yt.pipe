package main

import (
	"os"

	"github.com/sushistack/yt.pipe/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
