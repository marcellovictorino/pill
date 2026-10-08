package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/hf"
	"github.com/marcellovictorino/pill/internal/output"
)

func newPullCmd(a *App) *cobra.Command {
	var ctxSize int
	c := &cobra.Command{
		Use:   "pull <name>",
		Short: "Download a model's GGUF from Hugging Face (does not register it)",
		Long: `Download a GGUF into the models directory. <name> is a catalog name
(gemma4-26b, gemma4-26b-q3kxl-64k) or a Hugging Face reference:
hf.co/owner/repo:Q4_K_M, or owner/repo/file.gguf.

Downloads resume after an interruption and are verified against Hugging
Face's sha256. Gated repos need HF_TOKEN in the environment (it is never
stored). pull only downloads: register the model with 'pill add'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.load(); err != nil {
				return err
			}
			return runPull(cmd.Context(), a, args[0], ctxSize)
		},
	}
	c.Flags().IntVar(&ctxSize, "ctx", 0, "context size to use when the model is registered later (affects its name)")
	return c
}

func runPull(ctx context.Context, a *App, arg string, ctxSize int) error {
	snap, err := a.reg.Load()
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	m, err := a.reg.Resolve(snap, arg, ctxSize)
	if err != nil {
		return output.Fail([]string{"see the built-in models: pill catalog", "or use a Hugging Face reference: pill pull hf.co/owner/repo:Q4_K_M"}, "%v", err)
	}
	file, status, sum, err := a.pullModel(ctx, snap, m)
	if err != nil {
		return err
	}
	a.printer.Emit(output.Obj{}.
		Set("file", file.Name()).
		Set("status", status).
		Set("size", humanBytes(file.Size)).
		Set("sha256", orDash(sum)).
		Set("help", []string{"register it: pill add " + arg + " --unverified"}))
	return nil
}

// pullModel downloads the GGUF for m (when it is not already present) and
// records where it came from. It never registers the model.
func (a *App) pullModel(ctx context.Context, snap *snapshot, m config.Model) (hf.File, string, string, error) {
	if m.Repo == "" {
		return hf.File{}, "", "", output.Fail([]string{"use a catalog name or a Hugging Face reference"}, "%s has no known download source", m.Name)
	}
	ref := hf.Ref{Repo: m.Repo, Quant: m.Quant}
	if m.File != "" {
		ref = hf.Ref{Repo: m.Repo, File: m.File}
	}
	client := hf.NewClient()
	a.printer.Progress("looking up %s on Hugging Face...", m.Repo)
	files, err := client.ListFiles(ctx, m.Repo)
	if err != nil {
		return hf.File{}, "", "", hfFailure(err)
	}
	file, err := hf.FindFile(files, ref)
	if err != nil {
		return hf.File{}, "", "", output.Fail([]string{"check the name on https://huggingface.co/" + m.Repo}, "%v", err)
	}

	dest := filepath.Join(a.settings.ModelsDir, file.Name())
	status, sum := "downloaded", ""
	if st, err := os.Stat(dest); err == nil && st.Size() == file.Size {
		status = "already present" // size matches; pull does not re-hash 13 GB on every run
		sum = file.SHA256
	} else {
		a.printer.Progress("downloading %s (%s)", file.Name(), humanBytes(file.Size))
		sum, err = client.Download(ctx, m.Repo, file, a.settings.ModelsDir, a.downloadProgress(file.Name()))
		if err != nil {
			return hf.File{}, "", "", hfFailure(err)
		}
	}
	snap.Results.Files[file.Name()] = config.FileInfo{Repo: m.Repo, Quant: m.Quant, Size: file.Size, SHA256: sum}
	if err := config.SaveResults(a.paths, snap.Results); err != nil {
		return hf.File{}, "", "", output.Fail(nil, "%v", err)
	}
	return file, status, sum, nil
}

// hfFailure turns download errors into pill errors with a useful next step.
func hfFailure(err error) error {
	var auth *hf.AuthError
	switch {
	case errors.As(err, &auth):
		return output.Fail([]string{
			"gated or private repo? set HF_TOKEN in your environment (pill never stores it)",
			"check the repo name on https://huggingface.co",
		}, "%v", err)
	case hf.IsChecksumError(err):
		return output.Fail([]string{"run the same pill pull again to retry from scratch"}, "%v", err)
	}
	return output.Fail([]string{"check your network connection and run the same command again; downloads resume"}, "%v", err)
}
