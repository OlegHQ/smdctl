package main

import (
	"fmt"
	"os"

	"github.com/OlegHQ/smdctl/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		if code := commands.ExitCode(err); code != 1 {
			fmt.Fprint(os.Stderr, err)
			os.Exit(code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(commands.ExitCode(err))
	}
}
