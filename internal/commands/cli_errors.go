package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type CLIError struct {
	message string
	exit    int
}

func (e *CLIError) Error() string { return e.message }

func ExitCode(err error) int {
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return cliErr.exit
	}
	return 1
}

func newCLIError(message string) *CLIError { return &CLIError{message: message, exit: 2} }

func Execute() error {
	root := newRoot()
	cmd, err := root.ExecuteC()
	if err == nil {
		return nil
	}
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return err
	}
	return normalizeCobraParseError(cmd, err)
}

func normalizeCobraParseError(cmd *cobra.Command, err error) error {
	message := err.Error()
	if strings.HasPrefix(message, "unknown flag: ") {
		argument := strings.TrimPrefix(message, "unknown flag: ")
		return unexpectedArgument(cmd, argument)
	}
	if strings.HasPrefix(message, "unknown shorthand flag: ") {
		start := strings.Index(message, "in -")
		if start >= 0 {
			return unexpectedArgument(cmd, message[start+3:])
		}
	}
	if strings.HasPrefix(message, "unknown command ") {
		start := strings.IndexByte(message, '"')
		end := strings.IndexByte(message[start+1:], '"')
		if start >= 0 && end >= 0 {
			name := message[start+1 : start+1+end]
			return newCLIError(fmt.Sprintf("error: unrecognized subcommand '%s'\n\nUsage: smdctl [COMMAND]\n\nFor more information, try '--help'.\n", name))
		}
	}
	if strings.HasPrefix(message, "invalid argument ") {
		var value, flag string
		if _, scanErr := fmt.Sscanf(message, "invalid argument %q for %q flag:", &value, &flag); scanErr == nil {
			label := flagLabel(cmd, flag)
			reason := "invalid value"
			if strings.Contains(message, "invalid syntax") {
				reason = "invalid digit found in string"
			}
			return newCLIError(fmt.Sprintf("error: invalid value '%s' for '%s': %s\n\nFor more information, try '--help'.\n", value, label, reason))
		}
	}
	return err
}

func flagLabel(cmd *cobra.Command, flag string) string {
	if cmd != nil {
		flag = strings.TrimSpace(flag[strings.LastIndex(flag, ",")+1:])
		if f := cmd.Flags().Lookup(strings.TrimPrefix(flag, "--")); f != nil {
			name := "--" + f.Name
			if f.Value.Type() == "int" {
				name += " <" + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_")) + ">"
			}
			return name
		}
	}
	return flag
}

func requireArguments(cmd *cobra.Command, args []string, min, max int, metavar string) error {
	if len(args) < min {
		return newCLIError(fmt.Sprintf("error: the following required arguments were not provided:\n  %s\n\nUsage: %s\n\nFor more information, try '--help'.\n", metavar, usageLine(cmd)))
	}
	if max >= 0 && len(args) > max {
		return unexpectedArgument(cmd, args[max])
	}
	return nil
}

func unexpectedArgument(cmd *cobra.Command, argument string) error {
	return newCLIError(fmt.Sprintf("error: unexpected argument '%s' found\n\nUsage: %s\n\nFor more information, try '--help'.\n", argument, usageLine(cmd)))
}

func usageLine(cmd *cobra.Command) string {
	if cmd == nil {
		return "smdctl [COMMAND]"
	}
	switch cmd.Name() {
	case "run":
		return "smdctl run [OPTIONS] NAME -- COMMAND [ARGS...]"
	case "ps":
		return "smdctl ps [OPTIONS]"
	case "tasks":
		return "smdctl tasks [OPTIONS] [FILTER]"
	case "start", "stop", "restart":
		return "smdctl " + cmd.Name() + " <NAMES>..."
	case "logs":
		return "smdctl logs <SERVICE>"
	case "status", "env", "inspect", "regenerate":
		return "smdctl " + cmd.Name() + " <NAME>"
	case "rm":
		return "smdctl rm <NAMES>..."
	case "explain":
		return "smdctl explain [REST]..."
	case "migrate", "version":
		return "smdctl " + cmd.Name()
	default:
		return "smdctl [COMMAND]"
	}
}

func invalidOutputFormat(value string) error {
	return newCLIError(fmt.Sprintf("error: invalid value '%s' for '--output <OUTPUT>'\n  [possible values: table, json]\n\nFor more information, try '--help'.\n", value))
}
