package main

import (
	"fmt"
	"os"

	"github.com/env-guard/env-guard/internal/commands"
	"github.com/env-guard/env-guard/internal/tui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "login":
			os.Exit(commands.Login())
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
			os.Exit(1)
		}
	}
	os.Exit(tui.Run())
}
