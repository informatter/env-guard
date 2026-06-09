package main

import (
	"os"

	"github.com/env-guard/env-guard/internal/tui"
)

func main() {
	os.Exit(tui.Run())
}
