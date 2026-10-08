// Package registry is pill's model book-keeping. It joins the portable
// models.toml with the machine-local results.json, and keeps the two
// generated artefacts (models.ini for llama-server, the "pill" provider in
// Pi's models.json) in step with them.
package registry

import (
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
		return r.resolveFile(arg, ctx)
	}
	if e, ok := r.Catalog.ResolveWithCtx(arg, ctx); ok {
		return FromCatalog(e), nil
	}
	return config.Model{}, fmt.Errorf("unknown model %q", arg)
}

func (r *Registry) resolveFile(file string, ctx int) (config.Model, error) {
	file = filepath.Base(file)
	if e, ok := r.Catalog.FindByFile(file); ok {
		if ctx > 0 {
			e.Ctx = ctx
			e.Name = catalog.NameFor(e.Family.Name, e.Variant.Quant, ctx)
		}
		return FromCatalog(e), nil
	}
	if ctx == 0 {
		ctx = genericCtx
	}
	stem := strings.TrimSuffix(file, filepath.Ext(file))
	return generic(hf.Ref{File: file}.BaseName()+"-"+catalog.CtxLabel(ctx), "", file, stem, ctx), nil
}

func (r *Registry) resolveRef(s *Snapshot, arg string, ctx int) (config.Model, error) {
	ref, err := hf.ParseRef(arg)
	if err != nil {
		return config.Model{}, err
	}
	if ctx == 0 {
		ctx = genericCtx
	}
	file := ref.File
	if file == "" {
		file = r.localFileForQuant(s, ref)
	}
	if file == "" {
		// Not downloaded yet: keep the quant so `pill pull` can look it up.
		name := ref.BaseName() + "-" + hf.ShortQuant(ref.Quant) + "-" + catalog.CtxLabel(ctx)
		return generic(name, ref.Repo, "", ref.Quant, ctx), nil
	}
	if e, ok := r.Catalog.FindByFile(file); ok && e.Family.Repo == ref.Repo {
		return r.resolveFile(file, ctx)
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

// localFileForQuant finds an already-downloaded GGUF for owner/repo:quant,
// first through the download manifest, then by file-name suffix.
func (r *Registry) localFileForQuant(s *Snapshot, ref hf.Ref) string {
	for file, info := range s.Results.Files {
		if info.Repo == ref.Repo && strings.EqualFold(info.Quant, ref.Quant) {
			return file
		}
	}
	entries, _ := os.ReadDir(r.Settings.ModelsDir)
	q := strings.ToLower(ref.Quant)
	for _, e := range entries {
		n := strings.ToLower(e.Name())
		if strings.HasSuffix(n, ".gguf") && (strings.HasSuffix(n, "-"+q+".gguf") || strings.HasSuffix(n, "."+q+".gguf")) {
			return e.Name()
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
