package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/output"
)

func newRmCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a model: its entry, Pi registration, and its GGUF when unused",
		Long: `Remove a model from models.toml, the generated models.ini and Pi. The GGUF is
deleted too, unless another entry (a different context size, say) still uses
the same file. A downloaded GGUF that was never registered can be removed by
its name as shown in 'pill ls'.

Removing something that is already gone is a no-op, not an error.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.load(); err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			return runRm(ctx, a, args[0])
		},
	}
}

func runRm(ctx context.Context, a *App, arg string) error {
	snap, err := a.reg.Load()
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	name := arg
	if _, ok := snap.Models.Find(name); !ok {
		if e, ok := a.reg.Catalog.Resolve(arg); ok {
			name = e.Name
		}
	}
	m, registered := snap.Models.Find(name)
	if !registered {
		return rmPulledFile(a, snap, arg)
	}
	entry := *m

	// Free the router's hold on the model before its file goes away.
	if a.rt.Healthy(ctx) {
		if loaded, _ := a.rt.Loaded(ctx); contains(loaded, entry.Name) {
			if _, err := a.rt.UnloadAll(ctx); err != nil {
				return output.Fail([]string{"pill stop"}, "cannot unload %s: %v", entry.Name, err)
			}
		}
	}

	snap.Models.Remove(entry.Name)
	delete(snap.Results.Models, entry.Name)

	fileStatus, freed := "kept (still used by "+usersOf(snap, entry.File)+")", int64(0)
	if users := usersOf(snap, entry.File); users == "" {
		path := a.reg.GGUFPath(entry)
		freed = a.reg.FileSize(entry)
		switch err := os.Remove(path); {
		case err == nil:
			fileStatus = "deleted"
		case os.IsNotExist(err):
			fileStatus, freed = "not on disk", 0
		default:
			return output.Fail(nil, "cannot delete %s: %v", path, err)
		}
		_ = os.Remove(path + ".partial")
		delete(snap.Results.Files, entry.File)
	}

	if a.settings.Default == entry.Name {
		if err := config.SetDefault(a.paths, ""); err != nil {
			return output.Fail(nil, "%v", err)
		}
	}
	if err := a.reg.Save(snap); err != nil {
		return output.Fail(nil, "%v", err)
	}
	applied, err := a.reg.Apply(snap)
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	if a.rt.Healthy(ctx) {
		if _, err := a.ensureRouter(ctx, applied); err != nil {
			return err
		}
	}

	doc := output.Obj{}.Set("model", entry.Name).Set("status", "removed").Set("file", fileStatus)
	if freed > 0 {
		doc = doc.Set("freed", humanBytes(freed))
	}
	var help []string
	if a.settings.Default == entry.Name {
		help = append(help, "the default model was removed; choose another: pill default <name>")
	}
	a.printer.Emit(doc.Set("help", help))
	return nil
}

// rmPulledFile handles a name that is not registered: either a downloaded
// GGUF with no entry (delete it) or nothing at all (a no-op).
func rmPulledFile(a *App, snap *snapshot, arg string) error {
	for _, row := range a.reg.Rows(snap) {
		if row.State != "pulled" || row.Name != arg {
			continue
		}
		path := filepath.Join(a.settings.ModelsDir, row.File)
		if err := os.Remove(path); err != nil {
			return output.Fail(nil, "cannot delete %s: %v", path, err)
		}
		delete(snap.Results.Files, row.File)
		if err := config.SaveResults(a.paths, snap.Results); err != nil {
			return output.Fail(nil, "%v", err)
		}
		a.printer.Emit(output.Obj{}.Set("model", arg).Set("status", "removed").Set("file", "deleted").Set("freed", humanBytes(row.Size)))
		return nil
	}
	a.printer.Emit(output.Obj{}.Set("model", arg).Set("status", "not found (nothing to remove)").
		Set("help", []string{"list what exists: pill ls"}))
	return nil
}

// usersOf names the other entries that use a GGUF file ("" when none).
func usersOf(snap *snapshot, file string) string {
	var names []string
	for _, m := range snap.Models.Models {
		if m.File == file {
			names = append(names, m.Name)
		}
	}
	return strings.Join(names, ", ")
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
