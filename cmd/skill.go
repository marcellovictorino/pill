package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/output"
	"github.com/marcellovictorino/pill/internal/pi"
	"github.com/marcellovictorino/pill/skills"
)

func newSkillCmd(a *App) *cobra.Command {
	c := &cobra.Command{
		Use:   "skill",
		Short: "Manage the pill skill for coding agents",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	var dir string
	install := &cobra.Command{
		Use:   "install",
		Short: "Install the pill skill into Pi's skills directory",
		Long: `Write the embedded SKILL.md to <skills dir>/pill/SKILL.md so Pi (and other
agents that read the same layout) learn how to use pill. The default skills
directory is ~/.pi/agent/skills (PI_CODING_AGENT_DIR is honoured).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				d, err := pi.AgentDir()
				if err != nil {
					return output.Fail(nil, "%v", err)
				}
				dir = filepath.Join(d, "skills")
			}
			dest := filepath.Join(dir, "pill", "SKILL.md")
			status := "installed"
			if old, err := os.ReadFile(dest); err == nil {
				if string(old) == string(skills.PillSkill) {
					status = "already installed"
				} else {
					status = "updated"
				}
			}
			if status != "already installed" {
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
					return output.Fail(nil, "%v", err)
				}
				if err := config.WriteFileAtomic(dest, skills.PillSkill, 0o644); err != nil {
					return output.Fail(nil, "%v", err)
				}
			}
			a.printer.Emit(output.Obj{}.Set("skill", "pill").Set("status", status).Set("path", dest).
				Set("help", []string{"Pi loads skills from this directory on start"}))
			return nil
		},
	}
	install.Flags().StringVar(&dir, "dir", "", "skills directory to install into (default: <pi agent dir>/skills)")
	c.AddCommand(install)
	return c
}
