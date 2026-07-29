package main

import (
	"fmt"
	"os"

	"github.com/Malvi1697/lazyglab/internal/app"
)

var version = "0.1.0-dev"

const usage = `lazyglab — a terminal UI for GitLab

Usage:
  lazyglab            open the cockpit
  lazyglab setup      run the setup wizard again
  lazyglab update     download and install the newest release
  lazyglab --version  print the version
`

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("lazyglab %s\n", version)
			os.Exit(0)
		case "--help", "-h", "help":
			fmt.Print(usage)
			os.Exit(0)
		case "setup":
			if err := app.Setup(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		case "update":
			if err := app.SelfUpdate(version, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	if err := app.Run(version); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
