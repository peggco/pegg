package main

import (
	"fmt"
	"os"

	"github.com/peggco/pegg/internal/core/bootstrap"
)

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "hooks" && os.Args[2] == "serve" {
		_ = os.Setenv("PEGG_HOOKS_SERVE", "1")
	}
	if err := bootstrap.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "pegg: %v\n", err)
		os.Exit(1)
	}
}
