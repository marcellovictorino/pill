// Package registry is pill's model book-keeping. It joins the portable
// models.toml with the machine-local results.json, and keeps the two
// generated artefacts (models.ini for llama-server, the "pill" provider in
// Pi's models.json) in step with them.
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/marcellovictorino/pill/internal/catalog"
	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/hf"
	"github.com/marcellovictorino/pill/internal/pi"
	"github.com/marcellovictorino/pill/internal/preset"
)

// Generic defaults for models that are not in the catalog.
const (
	genericCtx       = 32768
	genericMaxTokens = 8192
)

// Registry is the entry point; build it with Open.
type Registry struct {
	Paths    config.Paths
	Settings config.Settings
	PiDir    string
	Catalog  *catalog.Catalog
}

// Open loads the catalog and works out Pi's directory.
func Open(p config.Paths, s config.Settings) (*Registry, error) {
	cat, err := catalog.Load()
	if err != nil {
		return nil, err
	}
	dir, err := pi.AgentDir()
	if err != nil {
		return nil, err
	}
	return &Registry{Paths: p, Settings: s, PiDir: dir, Catalog: cat}, nil
}

// Snapshot is models.toml plus results.json, loaded together.
type Snapshot struct {
	Models  *config.ModelsFile
	Results *config.Results
}

// Load reads both files.
func (r *Registry) Load() (*Snapshot, error) {
	mf, err := config.LoadModels(r.Paths)
	if err != nil {
		return nil, err
	}
	res, err := config.LoadResults(r.Paths)
	if err != nil {
		return nil, err
	}
	return &Snapshot{Models: mf, Results: res}, nil
}

// Save writes both files.
func (r *Registry) Save(s *Snapshot) error {
	if err := config.SaveModels(r.Paths, s.Models); err != nil {
		return err
	}
	return config.SaveResults(r.Paths, s.Results)
}

// ErrNoChange is returned by an Update callback that decided not to modify
// anything; Update then skips the write.
var ErrNoChange = errors.New("no change")

// ErrNothingToDo is ErrNoChange for UpdateApply callers that also want the
// generated files left alone (an idempotent no-op such as removing a name that
// does not exist must not touch models.ini or Pi's configuration).
var ErrNothingToDo = errors.New("nothing to do")

// Refresh regenerates models.ini and Pi's provider from the current state,
// reading that state inside the lock so it can never apply an older snapshot
// over a concurrent add or rm.
func (r *Registry) Refresh(ctx context.Context) (*Snapshot, Applied, error) {
	return r.UpdateApply(ctx, func(*Snapshot) error { return ErrNoChange })
}

// Update changes the model state transactionally: it takes the state lock,
// reads models.toml and results.json fresh, lets fn modify them, and saves.
// Reading inside the lock is what stops a slow command (a benchmark that ran
// for 20 minutes) from writing back a stale copy over what others changed.
// fn must not call Update itself; the lock is not re-entrant.
func (r *Registry) Update(ctx context.Context, fn func(*Snapshot) error) (*Snapshot, error) {
	snap, _, err := r.update(ctx, fn, false)
	return snap, err
}

// UpdateApply is Update that also regenerates models.ini and Pi's provider
// before releasing the lock. Generating the files after unlocking would let
// two concurrent commands apply their snapshots in the opposite order and
// leave the generated files without the newer model.
func (r *Registry) UpdateApply(ctx context.Context, fn func(*Snapshot) error) (*Snapshot, Applied, error) {
	return r.update(ctx, fn, true)
}

func (r *Registry) update(ctx context.Context, fn func(*Snapshot) error, apply bool) (*Snapshot, Applied, error) {
	var applied Applied
	unlock, err := config.LockState(ctx, r.Paths)
	if err != nil {
		return nil, applied, err
	}
	defer unlock()
	snap, err := r.Load()
	if err != nil {
		return nil, applied, err
	}
	// ErrNothingToDo: leave everything alone, including the generated files.
	// ErrNoChange: nothing to save, but still regenerate (callers want Applied).
	// Any other error aborts; success saves whichever file actually changed.
	beforeModels, beforeResults := fingerprint(snap.Models), fingerprint(snap.Results)
	if err := fn(snap); errors.Is(err, ErrNothingToDo) {
		return snap, applied, nil
	} else if err != nil && !errors.Is(err, ErrNoChange) {
		return snap, applied, err
	} else if err == nil {
		// Write only the file that changed, so a removal that touches the
		// download manifest never reformats a hand-edited models.toml.
		if fingerprint(snap.Models) != beforeModels {
			if err := config.SaveModels(r.Paths, snap.Models); err != nil {
				return snap, applied, err
			}
		}
		if fingerprint(snap.Results) != beforeResults {
			if err := config.SaveResults(r.Paths, snap.Results); err != nil {
				return snap, applied, err
			}
		}
	}
	if apply {
		applied, err = r.Apply(snap)
	}
	return snap, applied, err
}

// fingerprint is a comparable form of a state value, used to notice whether a
// transaction changed it.
func fingerprint(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// UpdateResults is Update for callers that only change results.json (the
// download manifest); models.toml is neither read nor written, so a download
// never creates or rewrites the portable model list.
func (r *Registry) UpdateResults(ctx context.Context, fn func(*config.Results) error) error {
	unlock, err := config.LockState(ctx, r.Paths)
	if err != nil {
		return err
	}
	defer unlock()
	res, err := config.LoadResults(r.Paths)
	if err != nil {
		return err
	}
	if err := fn(res); err != nil {
		return err
	}
	return config.SaveResults(r.Paths, res)
}

// GGUFPath is where a model's file lives (or would live).
func (r *Registry) GGUFPath(m config.Model) string {
	return filepath.Join(r.Settings.ModelsDir, m.File)
}

// FileSize returns the GGUF size, or 0 when the file is missing.
func (r *Registry) FileSize(m config.Model) int64 {
	st, err := os.Stat(r.GGUFPath(m))
	if err != nil || !st.Mode().IsRegular() {
		return 0
	}
	return st.Size()
}

// State is the effective gate state of an entry (see config.EffectiveState).
func (r *Registry) State(s *Snapshot, m config.Model) string {
	return config.EffectiveState(s.Results.Get(m.Name), preset.SectionHash(m, r.Settings.ModelsDir), r.FileSize(m))
}

// FromCatalog builds a models.toml entry from a catalog entry.
func FromCatalog(e catalog.Entry) config.Model {
	preset := map[string]any{}
	for k, v := range e.Family.Preset {
		preset[k] = v
	}
	return config.Model{
		Name: e.Name, Repo: e.Family.Repo, File: e.Variant.File, Quant: e.Variant.Quant,
		Ctx: e.Ctx, Reasoning: e.Family.Reasoning, MaxTokens: e.Family.MaxTokens, Preset: preset,
	}
}

// Resolve turns what a person typed into a models.toml entry (not saved).
// It accepts, in order: a name already in models.toml, a Hugging Face
// reference, a .gguf file name in the models dir, and a catalog name.
// ctx > 0 overrides the context size for catalog, reference and file forms.
func (r *Registry) Resolve(s *Snapshot, arg string, ctx int) (config.Model, error) {
	if m, ok := s.Models.Find(arg); ok && (ctx == 0 || ctx == m.Ctx) {
		return *m, nil
	}
	switch {
	case hf.LooksLikeRef(arg):
		return r.resolveRef(s, arg, ctx)
	case strings.HasSuffix(strings.ToLower(arg), ".gguf"):
		return r.resolveFile(s, arg, ctx)
	}
	if e, ok := r.Catalog.ResolveWithCtx(arg, ctx); ok {
		return FromCatalog(e), nil
	}
	return config.Model{}, fmt.Errorf("unknown model %q", arg)
}

// fromCatalogEntry builds the models.toml entry for a catalog variant, with
// an optional context override (which also renames it).
func fromCatalogEntry(e catalog.Entry, ctx int) config.Model {
	if ctx > 0 {
		e.Ctx = ctx
		e.Name = catalog.NameFor(e.Family.Name, e.Variant.Quant, ctx)
	}
	return FromCatalog(e)
}

// resolveFile turns a GGUF file name into an entry. A catalog file gets the
// catalog's settings; any other file keeps the Hugging Face source recorded
// when pill downloaded it, so moving models.toml to another machine still
// tells `pill sync` where to fetch it from.
func (r *Registry) resolveFile(s *Snapshot, file string, ctx int) (config.Model, error) {
	file = filepath.Base(file)
	if e, ok := r.Catalog.FindByFile(file); ok {
		return fromCatalogEntry(e, ctx), nil
	}
	if ctx == 0 {
		ctx = genericCtx
	}
	stem := strings.TrimSuffix(file, filepath.Ext(file))
	repo, quant := "", stem
	if info, ok := s.Results.Files[file]; ok && info.Repo != "" {
		repo = info.Repo
		if info.Quant != "" {
			quant = info.Quant
		}
	}
	return generic(hf.Ref{File: file}.BaseName()+"-"+catalog.CtxLabel(ctx), repo, file, quant, ctx), nil
}

func (r *Registry) resolveRef(s *Snapshot, arg string, ctx int) (config.Model, error) {
	ref, err := hf.ParseRef(arg)
	if err != nil {
		return config.Model{}, err
	}
	// A reference to a catalog model resolves to the catalog entry whether or
	// not it has been downloaded yet, so the same reference always names the
	// same entry (and a benchmark measures the settings that will be used).
	if ref.Quant != "" {
		if e, ok := r.Catalog.FindByRepoQuant(ref.Repo, ref.Quant); ok {
			return fromCatalogEntry(e, ctx), nil
		}
	}
	if ctx == 0 {
		ctx = genericCtx
	}
	// Locally a file is known by its base name: Hugging Face paths such as
	// sub/model.gguf are downloaded into the models directory as model.gguf.
	file := ""
	if ref.File != "" {
		file = filepath.Base(ref.File)
	} else {
		file = r.localFileForQuant(s, ref)
	}
	// A file name is shared by every repository, so an explicit reference only
	// matches a local file that was downloaded from the same repository.
	if info, ok := s.Results.Files[file]; ok && info.Repo != "" && !strings.EqualFold(info.Repo, ref.Repo) {
		return config.Model{}, fmt.Errorf("%s in %s is from %s, not %s; pill will not reuse or overwrite it", file, r.Settings.ModelsDir, info.Repo, ref.Repo)
	}
	if file == "" {
		// Not downloaded yet: keep the quant so `pill pull` can look it up.
		name := ref.BaseName() + "-" + hf.ShortQuant(ref.Quant) + "-" + catalog.CtxLabel(ctx)
		return generic(name, ref.Repo, "", ref.Quant, ctx), nil
	}
	if e, ok := r.Catalog.FindByFile(file); ok && strings.EqualFold(e.Family.Repo, ref.Repo) {
		return fromCatalogEntry(e, ctx), nil
	}
	name := ref.BaseName()
	if ref.File == "" {
		name += "-" + hf.ShortQuant(ref.Quant)
	}
	return generic(name+"-"+catalog.CtxLabel(ctx), ref.Repo, file, ref.Quant, ctx), nil
}

func generic(name, repo, file, quant string, ctx int) config.Model {
	return config.Model{
		Name: name, Repo: repo, File: file, Quant: quant, Ctx: ctx,
		MaxTokens: min(genericMaxTokens, ctx/4), Preset: map[string]any{},
	}
}

// localFileForQuant finds an already-downloaded GGUF for owner/repo:quant
// from the download manifest. Only a recorded download counts: matching file
// names by quantisation suffix would take repository A's a-Q4_K_M.gguf for
// repository B's Q4_K_M and register the wrong weights.
func (r *Registry) localFileForQuant(s *Snapshot, ref hf.Ref) string {
	for file, info := range s.Results.Files {
		if !strings.EqualFold(info.Repo, ref.Repo) {
			continue
		}
		// A download made by file name records no quantisation; within the
		// same repository the file name may still carry it.
		quantMatches := strings.EqualFold(info.Quant, ref.Quant)
		if !quantMatches && info.Quant == "" {
			_, err := hf.FindFile([]hf.File{{Path: file}}, hf.Ref{Repo: ref.Repo, Quant: ref.Quant})
			quantMatches = err == nil
		}
		if !quantMatches {
			continue
		}
		if st, err := os.Stat(filepath.Join(r.Settings.ModelsDir, file)); err == nil && st.Mode().IsRegular() {
			return file
		}
	}
	return ""
}

// Applied reports what Apply changed.
type Applied struct {
	IniChanged bool
	PiChanged  bool
	IniHash    string
	Served     []string // models written to models.ini
	PiModels   []string // models offered to Pi
}

// Apply regenerates models.ini and the Pi provider from the snapshot.
//
// Entries whose GGUF is missing are skipped in both: the router could not
// load them, and Pi would offer a model that fails on first use.
func (r *Registry) Apply(s *Snapshot) (Applied, error) {
	var served []config.Model
	var out Applied
	for _, m := range s.Models.Models {
		if r.FileSize(m) == 0 {
			continue
		}
		served = append(served, m)
		out.Served = append(out.Served, m.Name)
	}
	piModels := r.PiModels(s)

	ini := preset.Render(served, r.Settings.ModelsDir)
	out.IniHash = preset.HashOf(ini)
	if old, err := os.ReadFile(r.Paths.ModelsIni()); err != nil || string(old) != ini {
		if err := config.WriteFileAtomic(r.Paths.ModelsIni(), []byte(ini), 0o644); err != nil {
			return out, err
		}
		out.IniChanged = true
	}

	var provider []byte
	if len(piModels) > 0 {
		provider = pi.BuildProvider(r.Settings.Port, piModels)
	}
	changed, err := pi.Merge(pi.ModelsPath(r.PiDir), provider)
	if err != nil {
		return out, err
	}
	out.PiChanged = changed
	for _, m := range piModels {
		out.PiModels = append(out.PiModels, m.ID)
	}
	return out, nil
}

// PiModels lists what Pi should be offered: entries registered with Pi, not
// failed, whose GGUF exists.
func (r *Registry) PiModels(s *Snapshot) []pi.Model {
	var out []pi.Model
	for _, m := range s.Models.Models {
		if r.FileSize(m) == 0 {
			continue
		}
		st := s.Results.Get(m.Name)
		state := r.State(s, m)
		if st != nil && st.InPi && state != config.StateFailed {
			out = append(out, pi.Model{
				ID: m.Name, Reasoning: m.Reasoning, Ctx: m.Ctx, MaxTokens: m.MaxTokens,
				Unverified: state == config.StateUnverified,
			})
		}
	}
	return out
}

// Row is one line of `pill ls`.
type Row struct {
	Name    string
	State   string // pulled, unverified, passed, failed
	Default bool
	InPi    bool
	Missing bool // declared in models.toml but the GGUF is not here
	Size    int64
	File    string
}

// Rows lists every entry plus downloaded GGUFs that no entry uses ("pulled").
func (r *Registry) Rows(s *Snapshot) []Row {
	var rows []Row
	used := map[string]bool{}
	for _, m := range s.Models.Models {
		used[m.File] = true
		size := r.FileSize(m)
		row := Row{Name: m.Name, State: r.State(s, m), Default: m.Name == r.Settings.Default, Size: size, File: m.File, Missing: size == 0}
		if st := s.Results.Get(m.Name); st != nil {
			row.InPi = st.InPi && row.State != config.StateFailed
		}
		rows = append(rows, row)
	}
	files, _ := os.ReadDir(r.Settings.ModelsDir)
	for _, f := range files {
		if used[f.Name()] || !strings.HasSuffix(strings.ToLower(f.Name()), ".gguf") || f.IsDir() {
			continue
		}
		info, err := f.Info()
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
		if e, ok := r.Catalog.FindByFile(f.Name()); ok {
			name = e.Name
		}
		rows = append(rows, Row{Name: name, State: "pulled", Size: info.Size(), File: f.Name()})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}
