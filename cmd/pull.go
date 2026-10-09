package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

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
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeModels(modelSources{catalog: true, registered: true}),
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
	// A reference with a folder (owner/repo/sub/model.gguf) names an exact
	// remote file; the model entry only keeps the local name.
	file, status, sum, err := a.pullModel(ctx, snap, m, remotePathOf(arg))
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

// remotePathOf returns the exact path inside the repository that a reference
// such as owner/repo/sub/model.gguf names ("" for anything else).
func remotePathOf(arg string) string {
	if !hf.LooksLikeRef(arg) {
		return ""
	}
	ref, err := hf.ParseRef(arg)
	if err != nil {
		return ""
	}
	return ref.File
}

// pullModel downloads the GGUF for m (when it is not already present) and
// records where it came from. It never registers the model. remote, when not
// empty, is the exact path of the file inside the repository.
func (a *App) pullModel(ctx context.Context, snap *snapshot, m config.Model, remote string) (hf.File, string, string, error) {
	if m.Repo == "" {
		return hf.File{}, "", "", output.Fail([]string{"use a catalog name or a Hugging Face reference"}, "%s has no known download source", m.Name)
	}
	ref := hf.Ref{Repo: m.Repo, Quant: m.Quant}
	if m.File != "" {
		ref = hf.Ref{Repo: m.Repo, File: m.File}
	}
	if remote != "" {
		ref.File = remote
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

	// Files live in one flat directory under their base name, so two
	// repositories can offer the same name. Never reuse or overwrite a file
	// that came from somewhere else: that would register one repository's
	// bytes as another's, or replace weights an existing model is using.
	// Hold a per-file lock from the check to the record: otherwise two pulls of
	// the same name from different repositories could both pass the check and
	// the second would rename over the first's bytes.
	unlockFile, err := config.LockFileName(ctx, a.paths, file.Name())
	if err != nil {
		return hf.File{}, "", "", output.Fail(nil, "%v", err)
	}
	defer unlockFile()
	// Read what is recorded now, not what was loaded before the lock.
	current, err := config.LoadResults(a.paths)
	if err != nil {
		return hf.File{}, "", "", output.Fail(nil, "%v", err)
	}
	dest := filepath.Join(a.settings.ModelsDir, file.Name())
	rec, recorded := current.Files[file.Name()]
	if recorded && rec.Repo != "" && !strings.EqualFold(rec.Repo, m.Repo) {
		return hf.File{}, "", "", nameClash(a, file.Name(), m.Repo, rec.Repo)
	}
	// The same repository can hold two folders with one file name.
	if recorded && rec.Path != "" && rec.Path != file.Path {
		return hf.File{}, "", "", nameClash(a, file.Name(), m.Repo, m.Repo+"/"+rec.Path)
	}
	status, sum := "downloaded", ""
	st, statErr := os.Stat(dest)
	switch {
	case statErr == nil && st.Size() == file.Size:
		status, sum = "already present", file.SHA256 // size matches; pull does not re-hash 13 GB on every run
		if !recorded && file.SHA256 != "" {
			// A file nobody recorded (copied in by hand): prove it is this one once.
			a.printer.Progress("checking %s against Hugging Face's sha256...", file.Name())
			if local, err := hf.HashFile(dest); err != nil {
				return hf.File{}, "", "", output.Fail(nil, "%v", err)
			} else if !strings.EqualFold(local, file.SHA256) {
				return hf.File{}, "", "", nameClash(a, file.Name(), m.Repo, "an unknown source")
			}
		}
	case statErr == nil && !recorded:
		// Same name, different size, and pill never downloaded it: do not guess.
		return hf.File{}, "", "", nameClash(a, file.Name(), m.Repo, "an unknown source")
	default:
		a.printer.Progress("downloading %s (%s)", file.Name(), humanBytes(file.Size))
		var err error
		sum, err = client.Download(ctx, m.Repo, file, a.settings.ModelsDir, a.downloadProgress(file.Name()))
		if err != nil {
			return hf.File{}, "", "", hfFailure(err)
		}
	}
	// Record where the file came from, against the current state on disk.
	err = a.reg.UpdateResults(ctx, func(res *config.Results) error {
		cur, ok := res.Files[file.Name()]
		if ok && cur.Repo != "" && !strings.EqualFold(cur.Repo, m.Repo) {
			return nameClash(a, file.Name(), m.Repo, cur.Repo)
		}
		if ok && cur.Path != "" && cur.Path != file.Path {
			return nameClash(a, file.Name(), m.Repo, m.Repo+"/"+cur.Path)
		}
		// A quantisation is only worth recording when it is one: a model made
		// from a bare file name carries the file stem in its Quant field.
		quant := m.Quant
		if strings.EqualFold(quant, strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))) {
			quant = ""
		}
		if quant == "" && ok && strings.EqualFold(cur.Repo, m.Repo) {
			quant = cur.Quant // pulling by file name must not forget a known quantisation
		}
		res.Files[file.Name()] = config.FileInfo{Repo: m.Repo, Path: file.Path, Quant: quant, Size: file.Size, SHA256: sum}
		return nil
	})
	if err != nil {
		return hf.File{}, "", "", err
	}
	if cur, err := config.LoadResults(a.paths); err == nil {
		snap.Results.Files[file.Name()] = cur.Files[file.Name()] // keep the caller's snapshot in step
	}
	return file, status, sum, nil
}

// nameClash reports a file-name collision between repositories.
func nameClash(a *App, name, wantRepo, haveFrom string) error {
	return output.Fail([]string{
		"remove the existing file first if you no longer need it: pill rm <name>",
		"or download it under a different name from the Hugging Face page",
	}, "%s in %s is from %s, not %s; pill will not reuse or overwrite it", name, a.settings.ModelsDir, haveFrom, wantRepo)
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
