package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// modelSources says which kinds of names a command can complete.
type modelSources struct {
	catalog    bool // built-in catalog names (and the short family names)
	registered bool // models in models.toml
	files      bool // .gguf files in the models directory
}

// completeModels returns a cobra ValidArgsFunction. Cobra calls it when the
// shell asks for completions (through the hidden `pill __complete` command)
// and prints each returned string as one candidate; text after a tab is the
// description shell menus show next to it. Only the first argument is
// completed, and any problem simply yields no suggestions: completion must
// never print an error into somebody's prompt.
func (a *App) completeModels(src modelSources) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 || a.load() != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		seen := map[string]bool{}
		var out []string
		offer := func(name, desc string) {
			if seen[name] || !strings.HasPrefix(name, toComplete) {
				return
			}
			seen[name] = true
			out = append(out, name+"\t"+desc)
		}
		// Registered models first, so their gate state is the description shown
		// for a name that is also in the catalog.
		if src.registered {
			if snap, err := a.reg.Load(); err == nil {
				for _, m := range snap.Models.Models {
					offer(m.Name, a.reg.State(snap, m))
				}
			}
		}
		if src.catalog {
			for _, e := range a.reg.Catalog.Entries() {
				offer(e.Name, fmt.Sprintf("catalog: %s, %dK context", e.Variant.Quant, e.Ctx/1024))
			}
			for _, f := range a.reg.Catalog.Families {
				offer(f.Name, "catalog: recommended variant")
			}
		}
		if src.files {
			if entries, err := os.ReadDir(a.settings.ModelsDir); err == nil {
				for _, f := range entries {
					if !f.IsDir() && strings.HasSuffix(f.Name(), ".gguf") {
						offer(f.Name(), "file in the models directory")
					}
				}
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}
