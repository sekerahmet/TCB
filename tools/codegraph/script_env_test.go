package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// Kural: kabuk ve Python betiklerinde (izlenen .sh, .py) yorum satırı dışında geçen her kodlu ortam değişkeni adı (her
// dildeki adı, ortak adı ya da kimliği: `TR_CPU_AVX2=0`, `export TR_DIL=en`, `${TR_DIL:-tr}`, `os.environ["TR_DIL"]`)
// betiğin dosya düğümünden o ad koduna `uses_env` kenarıdır: `query TR_env_GPU_SYNC` değişkeni koyan betiği de verir
// (eksikler listesi, ana oturum 2026-10-26: card_check.sh ve full_check.sh görünmüyordu). Betikler ve adlar
// codegraph'ın çıkarıcısından bağımsız, git'in listesinden ve düz metin taramasıyla bulunur.
func TestEveryCodedEnvironmentVariableInAScriptIsAnEdge(t *testing.T) {
	root, _, g := repoGraph(t)
	env := envCodes(root)
	rels, ok := codeindex.GitTracked(root)
	if !ok {
		t.Skip("git is not available")
	}
	word := regexp.MustCompile(`\bTR_[A-Za-z0-9_]+\b`)
	want := map[string]bool{} // "dosya kod"
	for _, rel := range rels {
		if ext := filepath.Ext(rel); ext != ".sh" && ext != ".py" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			for _, w := range word.FindAllString(line, -1) {
				if code := env[w]; code != "" {
					want["file:"+rel+" name:"+code] = true
				}
			}
		}
	}
	for _, must := range []string{"file:tools/gpu/card_check.sh name:TR_GPU_SYNC", "file:tools/full_check.sh name:TR_CPU_AVX2", "file:tools/full_check.sh name:TR_LANG"} {
		if !want[must] {
			t.Errorf("the scan misses %s; it is broken", must)
		}
	}
	have := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == edgeUsesEnv {
			have[e.From+" "+e.To] = true
		}
	}
	for k := range want {
		if !have[k] {
			t.Errorf("%s: no uses_env edge", k)
		}
	}
	if !stamped("tools/full_check.sh") || !stamped("tools/bench/run.py") {
		t.Errorf("a script's changes do not change the stamp (fresh.go stampExts)")
	}
}
