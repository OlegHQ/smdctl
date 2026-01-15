package main

import (
	"fmt"
	"os"

	"github.com/nexo-tech/smdctl/internal/cmd"
	"github.com/nexo-tech/smdctl/internal/help"
)

var Version = "dev"

func main() {
	if len(os.Args) < 2 {
		help.ShowMain()
		os.Exit(0)
	}

	command := os.Args[1]
	args := os.Args[2:]

	var err error

	switch command {
	case "run":
		err = cmd.Run(args)
	case "ps":
		err = cmd.PS(args)
	case "start":
		err = cmd.Start(args)
	case "stop":
		err = cmd.Stop(args)
	case "restart":
		err = cmd.Restart(args)
	case "logs":
		err = cmd.Logs(args)
	case "status":
		err = cmd.Status(args)
	case "rm":
		err = cmd.Remove(args)
	case "env":
		err = cmd.Env(args)
	case "inspect":
		err = cmd.Inspect(args)
	case "explain":
		err = cmd.Explain(args)
	case "migrate":
		err = cmd.Migrate(args)
	case "help", "--help", "-h":
		if len(args) > 0 {
			showCommandHelp(args[0])
		} else {
			help.ShowMain()
		}
	case "version", "--version", "-v":
		fmt.Printf("smdctl version %s\n", Version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		fmt.Fprintf(os.Stderr, "Run 'smdctl help' for usage.\n")
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func showCommandHelp(command string) {
	switch command {
	case "run":
		help.ShowRun()
	default:
		fmt.Fprintf(os.Stderr, "No detailed help available for command: %s\n", command)
		fmt.Fprintf(os.Stderr, "Run 'smdctl help' for general usage.\n")
	}
}
