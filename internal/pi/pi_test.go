package pi

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func sampleProvider() json.RawMessage {
	return BuildProvider(11435, []Model{
		{ID: "gemma4-26b-iq4xs-64k", Reasoning: true, Ctx: 65536, MaxTokens: 16384, Unverified: true},
		{ID: "other", Ctx: 8192, MaxTokens: 2048},
	})
}

func TestBuildProviderGolden(t *testing.T) {
	var buf bytes.Buffer
	// BuildProvider fixes the key order through struct field order; Indent
	// pretty-prints without reordering, so the golden file shows what pill writes.
	if err := json.Indent(&buf, sampleProvider(), "", "  "); err != nil {
		t.Fatal(err)
	}
	buf.WriteByte('\n')
	got := buf.String()

	path := filepath.Join("testdata", "provider.json.golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("provider JSON differs from golden (run with -update)\n--- got ---\n%s--- want ---\n%s", got, want)
	}
	if !strings.Contains(got, `"name": "gemma4-26b-iq4xs-64k (unverified)"`) ||
		!strings.Contains(got, `"id": "gemma4-26b-iq4xs-64k"`) {
		t.Error("the unverified marker belongs in name only; id stays plain")
	}
}

func TestMergeCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent", "models.json")
	changed, err := Merge(path, sampleProvider())
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	ids := mustIDs(t, path)
	if !reflect.DeepEqual(ids, []string{"gemma4-26b-iq4xs-64k", "other"}) {
		t.Errorf("ids = %v", ids)
	}
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Error("no backup expected when the file did not exist")
	}
}

func mustIDs(t *testing.T, path string) []string {
	t.Helper()
	prov, _, err := ReadProvider(path)
	if err != nil || prov == nil {
		t.Fatalf("ReadProvider: %v %v", prov, err)
	}
	return ProviderModelIDs(prov)
}

const existing = `{
  "defaultThing": {"keep": [3, 1, 2]},
  "providers": {
    "openai": {"apiKey": "sk-x", "models": []},
    "llama-cpp": {"baseUrl": "http://127.0.0.1:8080/v1"}
  },
  "zzz": true
}
`

func TestMergePreservesEverythingElse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := Merge(path, sampleProvider())
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	// Backup holds the original bytes.
	bak, err := os.ReadFile(path + ".bak")
	if err != nil || string(bak) != existing {
		t.Errorf("backup wrong: %q %v", bak, err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	// Top-level key order and other providers survive.
	iDefault, iProviders, iZ := strings.Index(text, `"defaultThing"`), strings.Index(text, `"providers"`), strings.Index(text, `"zzz"`)
	if iDefault >= iProviders || iProviders >= iZ {
		t.Errorf("top-level key order changed:\n%s", text)
	}
	if !strings.Contains(text, `"sk-x"`) || !strings.Contains(text, `"llama-cpp"`) || !strings.Contains(text, `"keep"`) {
		t.Errorf("other keys lost:\n%s", text)
	}
	others, _ := OtherProviders(path)
	if others["llama-cpp"] != "http://127.0.0.1:8080/v1" || len(others) != 2 {
		t.Errorf("others = %v", others)
	}
}

func TestMergeIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")
	_ = os.WriteFile(path, []byte(existing), 0o644)
	if _, err := Merge(path, sampleProvider()); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(path + ".bak")
	changed, err := Merge(path, sampleProvider())
	if err != nil || changed {
		t.Fatalf("second merge: changed=%v err=%v", changed, err)
	}
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Error("an unchanged merge must not rewrite the backup")
	}
}

func TestMergeRemovesOnlyPill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")
	_ = os.WriteFile(path, []byte(existing), 0o644)
	_, _ = Merge(path, sampleProvider())
	changed, err := Merge(path, nil)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	prov, others, _ := ReadProvider(path)
	if prov != nil || len(others) != 2 {
		t.Errorf("prov=%s others=%v", prov, others)
	}
	// Removing when there is nothing to remove is a no-op.
	if changed, _ := Merge(path, nil); changed {
		t.Error("removing an absent provider must be a no-op")
	}
	if changed, _ := Merge(filepath.Join(t.TempDir(), "none.json"), nil); changed {
		t.Error("no file and no provider: nothing to do")
	}
}

func TestMergeRefusesInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")
	_ = os.WriteFile(path, []byte("{not json"), 0o644)
	if _, err := Merge(path, sampleProvider()); err == nil {
		t.Fatal("must refuse to modify a file it cannot parse")
	}
	if data, _ := os.ReadFile(path); string(data) != "{not json" {
		t.Error("the unparseable file was modified")
	}
}

func TestAgentDir(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "/custom/agent")
	if d, _ := AgentDir(); d != "/custom/agent" {
		t.Errorf("got %s", d)
	}
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("HOME", "/h")
	if d, _ := AgentDir(); d != "/h/.pi/agent" {
		t.Errorf("got %s", d)
	}
}

func TestArgs(t *testing.T) {
	cases := []struct {
		user []string
		want []string
	}{
		{nil, []string{"--model", "pill/m", "--thinking", "off"}},
		{[]string{"fix the bug"}, []string{"--model", "pill/m", "--thinking", "off", "fix the bug"}},
		{[]string{"--model", "x/y"}, []string{"--thinking", "off", "--model", "x/y"}},
		{[]string{"--model=x/y", "hi"}, []string{"--thinking", "off", "--model=x/y", "hi"}},
		{[]string{"--thinking", "high"}, []string{"--model", "pill/m", "--thinking", "high"}},
		{[]string{"--model", "a", "--thinking=low"}, []string{"--model", "a", "--thinking=low"}},
		{[]string{"--models", "a,b"}, []string{"--model", "pill/m", "--thinking", "off", "--models", "a,b"}},
		{[]string{"--", "--model"}, []string{"--model", "pill/m", "--thinking", "off", "--", "--model"}},
	}
	for _, c := range cases {
		if got := Args("m", c.user); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Args(%v) = %v, want %v", c.user, got, c.want)
		}
	}
}

func TestExecUsesInjectedFunc(t *testing.T) {
	var gotBin string
	var gotArgs []string
	orig := ExecFunc
	defer func() { ExecFunc = orig }()
	ExecFunc = func(bin string, args []string, env []string) error {
		gotBin, gotArgs = bin, args
		return nil
	}
	_ = Exec("/bin/pi", []string{"--model", "pill/x"})
	if gotBin != "/bin/pi" || !reflect.DeepEqual(gotArgs, []string{"pi", "--model", "pill/x"}) {
		t.Errorf("bin=%s args=%v", gotBin, gotArgs)
	}
}
