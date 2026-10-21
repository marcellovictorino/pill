// Package cmd holds the cobra commands, one file per command.
package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"example.com/notes/internal/output"
)

// usageError marks a mistake in how the command was called (exit code 2).
// Any other error is a runtime failure (exit code 1).
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func usageErrorf(format string, a ...any) error {
	return usageError{msg: fmt.Sprintf(format, a...)}
}

// formatOf reads the global --format flag.
func formatOf(cmd *cobra.Command) (output.Format, error) {
	v, _ := cmd.Flags().GetString("format")
	f, err := output.ParseFormat(v)
	if err != nil {
		return "", usageError{msg: err.Error()}
	}
	return f, nil
}

func newRoot(stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "notes",
		Short:         "A tiny note-taking CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageErrorf("unknown command %q", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().String("format", "human", "output format: human or json")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{msg: err.Error()} })
	root.AddCommand(newAdd(), newLs())
	return root
}

// Execute runs the CLI and returns the process exit code: 0 ok, 1 failure,
// 2 usage error. Errors go to stderr as "error: <message>"; nothing is
// printed to stdout on failure.
func Execute(args []string, stdout, stderr io.Writer) int {
	root := newRoot(stdout, stderr)
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		return 0
	}
	fmt.Fprintf(stderr, "error: %s\n", err)
	if _, ok := err.(usageError); ok {
		return 2
	}
	return 1
}
