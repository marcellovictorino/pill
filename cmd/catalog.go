package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/output"
	"github.com/marcellovictorino/pill/internal/sysinfo"
)

func newCatalogCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "catalog",
		Short: "List the built-in models and the variant recommended for this machine",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.load(); err != nil {
				return err
			}
			ramBytes, err := a.deps.Sys.TotalRAMBytes()
			if err != nil {
				return output.Fail(nil, "cannot read this machine's memory: %v", err)
			}
			ram := sysinfo.RAMGB(ramBytes)
			var table []output.Obj
			for _, e := range a.reg.Catalog.Entries() {
				local, recommended := "no", "no"
				if _, err := os.Stat(filepath.Join(a.settings.ModelsDir, e.Variant.File)); err == nil {
					local = "yes"
				}
				if e.Variant.Quant == e.Family.Recommended(ram).Quant {
					recommended = "yes"
				}
				table = append(table, output.Obj{}.
					Set("name", e.Name).
					Set("size", fmt.Sprintf("%.1f GB", e.Variant.SizeGB)).
					Set("ctx", e.Ctx).
					Set("recommended", recommended).
					Set("local", local))
			}
			a.printer.Emit(output.Obj{}.
				Set("ram_gb", ram).
				Set("catalog", table).
				Set("help", []string{"download one: pill pull <name>", "any Hugging Face GGUF works too: pill pull hf.co/owner/repo:Q4_K_M"}))
			return nil
		},
	}
}
