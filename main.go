package main

import (
	"fmt"
	"os"

	"github.com/janvseticek/lazyglab/internal/app"
)

var version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("lazyglab %s\n", version)
		os.Exit(0)
	}

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
