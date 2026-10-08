package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/marcellovictorino/pill/internal/output"
)

func newRoot(deps Deps) (*cobra.Command, *App) {
	app := &App{deps: deps}
	var showVersion bool

	root := &cobra.Command{
		Use:   "pill",
		Short: "Run the Pi coding agent with local models served by llama.cpp",
		Long: `pill opens the Pi coding agent on a local model.

Running "pill" starts the llama.cpp router if needed and opens Pi on your
default model. Pipe any command (or pass --format toon) to get compact,
agent-friendly TOON output instead of tables.`,
		SilenceUsage:  true, // we print our own errors
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				return printVersion(app)
			}
			return launchPi(cmd.Context(), app, nil)
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			p, err := output.NewPrinter(deps.Out, deps.Err, app.format, deps.StdoutTTY)
			if err != nil {
				return output.Usage([]string{"valid formats: toon, human"}, "%v", err)
			}
			app.printer = p
			return nil
		},
	}
	root.SetOut(deps.Out)
	root.SetErr(deps.Err)
	root.PersistentFlags().StringVar(&app.format, "format", "", "output format: toon or human (default: human on a terminal, toon otherwise)")

	// Flag and argument mistakes become exit-code-2 usage errors that list
	// what would have been valid.
	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return output.Usage(validFlagsHelp(c), "%v", err)
	})
	root.SetHelpFunc(func(c *cobra.Command, _ []string) {
		// Logo first, but only for a person: terminal, colour allowed, no agent.
		p, _ := output.NewPrinter(deps.Out, deps.Err, app.format, deps.StdoutTTY)
		if p != nil && p.ShowLogo() {
			fmt.Fprint(deps.Out, output.Logo(true), "\n")
		}
		if c.Long != "" {
			fmt.Fprintln(deps.Out, strings.TrimSpace(c.Long))
		} else {
			fmt.Fprintln(deps.Out, c.Short)
		}
		fmt.Fprint(deps.Out, "\n", c.UsageString())
	})
	// cobra can add --version itself, but we want the logo logic, so it is an
	// ordinary flag handled in RunE.
	root.Flags().BoolVar(&showVersion, "version", false, "print the pill version")

	root.AddCommand(
		newPiCmd(app),
		newServeCmd(app),
		newStopCmd(app),
		newPsCmd(app),
		newAddCmd(app),
		newDefaultCmd(app),
		newDoctorCmd(app),
		newVersionCmd(app),
	)
	return root, app
}

// validFlagsHelp lists the flags a command accepts, for usage errors.
func validFlagsHelp(c *cobra.Command) []string {
	var names []string
	add := func(f *pflag.Flag) { names = append(names, "--"+f.Name) }
	c.LocalFlags().VisitAll(add)
	c.InheritedFlags().VisitAll(add)
	sort.Strings(names)
	help := []string{"valid flags: " + strings.Join(names, ", ")}
	if len(c.Commands()) > 0 {
		var subs []string
		for _, s := range c.Commands() {
			if !s.Hidden {
				subs = append(subs, s.Name())
			}
		}
		help = append(help, "valid commands: "+strings.Join(subs, ", "))
	}
	return help
}
